package repositories

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"
	"user-service/internal/rabbitmq"

	"github.com/jmoiron/sqlx"

	"user-service/internal/models"
)

var ErrRequestForbidden = errors.New("friend request not allowed")

type FriendRepository interface {
	CreateRequest(ctx context.Context, fromUserID, toUserID int64) (*models.FriendRequest, error)
	GetIncomingRequests(ctx context.Context, userID int64) ([]models.FriendRequest, error)
	AcceptRequest(ctx context.Context, requestID, userID int64) error
	RejectRequest(ctx context.Context, requestID, userID int64) error
	ListFriends(ctx context.Context, userID int64) ([]int64, error)
	HasPendingRequest(ctx context.Context, fromUserID, toUserID int64) (bool, error)
	AreFriends(ctx context.Context, userID, otherID int64) (bool, error)
	DeleteFriendship(ctx context.Context, userID, friendID int64) error
}

type friendRepository struct {
	db        *sqlx.DB
	publisher rabbitmq.Publisher
}

func NewFriendRepository(db *sqlx.DB, publisher rabbitmq.Publisher) FriendRepository {
	return &friendRepository{db: db, publisher: publisher}
}

func (r *friendRepository) CreateRequest(ctx context.Context, fromUserID, toUserID int64) (*models.FriendRequest, error) {
	var req models.FriendRequest
	err := r.db.QueryRowxContext(ctx, `
INSERT INTO friend_requests (from_user_id, to_user_id, status)
VALUES ($1, $2, 'pending')
RETURNING id, from_user_id, to_user_id, status, created_at
`, fromUserID, toUserID).StructScan(&req)
	if err != nil {
		return nil, err
	}

	r.logPublish(ctx, "friend.request.created", map[string]any{
		"request_id":   req.ID,
		"from_user_id": req.FromUserID,
		"to_user_id":   req.ToUserID,
		"created_at":   req.CreatedAt,
	})

	return &req, nil
}

func (r *friendRepository) GetIncomingRequests(ctx context.Context, userID int64) ([]models.FriendRequest, error) {
	var reqs []models.FriendRequest
	err := r.db.SelectContext(ctx, &reqs, `
SELECT id, from_user_id, to_user_id, status, created_at
FROM friend_requests
WHERE to_user_id=$1 AND status='pending'
ORDER BY created_at DESC
`, userID)
	return reqs, err
}

func (r *friendRepository) AcceptRequest(ctx context.Context, requestID, userID int64) error {
	var eventPayload map[string]any
	err := r.withTx(ctx, func(tx *sqlx.Tx) error {
		var req models.FriendRequest
		if err := tx.GetContext(ctx, &req, `SELECT id, from_user_id, to_user_id, status, created_at FROM friend_requests WHERE id=$1`, requestID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return sql.ErrNoRows
			}
			return err
		}
		if req.ToUserID != userID {
			return ErrRequestForbidden
		}
		if req.Status != "pending" {
			return nil
		}

		acceptedAt := time.Now().UTC()

		if _, err := tx.ExecContext(ctx, `UPDATE friend_requests SET status='accepted' WHERE id=$1`, requestID); err != nil {
			return err
		}

		if err := r.insertFriendship(ctx, tx, req.FromUserID, req.ToUserID); err != nil {
			return err
		}
		if err := r.insertFriendship(ctx, tx, req.ToUserID, req.FromUserID); err != nil {
			return err
		}

		eventPayload = map[string]any{
			"user_id":     req.FromUserID,
			"friend_id":   req.ToUserID,
			"accepted_at": acceptedAt,
		}
		return nil
	})
	if err != nil {
		return err
	}

	if eventPayload != nil {
		r.logPublish(ctx, "friendship.created", eventPayload)
	}

	return nil
}

func (r *friendRepository) RejectRequest(ctx context.Context, requestID, userID int64) error {
	var toUserID int64
	if err := r.db.GetContext(ctx, &toUserID, `SELECT to_user_id FROM friend_requests WHERE id=$1`, requestID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		return err
	}
	if toUserID != userID {
		return ErrRequestForbidden
	}
	res, err := r.db.ExecContext(ctx, `
UPDATE friend_requests SET status='rejected'
WHERE id=$1 AND to_user_id=$2 AND status='pending'
`, requestID, userID)
	if err != nil {
		return err
	}
	count, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *friendRepository) ListFriends(ctx context.Context, userID int64) ([]int64, error) {
	var friends []int64
	err := r.db.SelectContext(ctx, &friends, `
SELECT friend_id
FROM friendships
WHERE user_id=$1
ORDER BY friend_id
`, userID)
	return friends, err
}

func (r *friendRepository) HasPendingRequest(ctx context.Context, fromUserID, toUserID int64) (bool, error) {
	var exists bool
	err := r.db.GetContext(ctx, &exists, `
SELECT EXISTS(
SELECT 1 FROM friend_requests
WHERE ((from_user_id=$1 AND to_user_id=$2) OR (from_user_id=$2 AND to_user_id=$1))
AND status='pending'
)
`, fromUserID, toUserID)
	return exists, err
}

func (r *friendRepository) AreFriends(ctx context.Context, userID, otherID int64) (bool, error) {
	var exists bool
	err := r.db.GetContext(ctx, &exists, `
SELECT EXISTS(
SELECT 1 FROM friendships WHERE user_id=$1 AND friend_id=$2
)
`, userID, otherID)
	return exists, err
}

func (r *friendRepository) DeleteFriendship(ctx context.Context, userID, friendID int64) error {
	_, err := r.db.ExecContext(ctx, `
DELETE FROM friendships
WHERE (user_id=$1 AND friend_id=$2) OR (user_id=$2 AND friend_id=$1)
`, userID, friendID)
	return err
}

func (r *friendRepository) insertFriendship(ctx context.Context, tx *sqlx.Tx, userID, friendID int64) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO friendships (user_id, friend_id) VALUES ($1, $2)
ON CONFLICT (user_id, friend_id) DO NOTHING
`, userID, friendID)
	return err
}

func (r *friendRepository) withTx(ctx context.Context, fn func(*sqlx.Tx) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (r *friendRepository) logPublish(ctx context.Context, eventType string, payload any) {
	if r.publisher == nil {
		return
	}
	if err := r.publisher.Publish(ctx, eventType, payload); err != nil {
		log.Printf("warning: failed to publish %s: %v", eventType, err)
	}
}
