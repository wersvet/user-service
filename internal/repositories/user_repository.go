package repositories

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"

	"user-service/internal/models"
)

type UserRepository interface {
	GetByID(ctx context.Context, id int64) (*models.User, error)
	GetAvatarURL(ctx context.Context, id int64) (string, error)
	SetAvatarURL(ctx context.Context, id int64, avatarURL string) error
	ClearAvatarURL(ctx context.Context, id int64) error
}

type userRepository struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) GetByID(ctx context.Context, id int64) (*models.User, error) {
	var user models.User
	err := r.db.GetContext(ctx, &user, "SELECT id, username, avatar_url FROM users WHERE id=$1", id)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetAvatarURL(ctx context.Context, id int64) (string, error) {
	var avatar sql.NullString
	err := r.db.GetContext(ctx, &avatar, "SELECT avatar_url FROM users WHERE id=$1", id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	if !avatar.Valid {
		return "", nil
	}
	return avatar.String, nil
}

func (r *userRepository) SetAvatarURL(ctx context.Context, id int64, avatarURL string) error {
	res, err := r.db.ExecContext(ctx, "UPDATE users SET avatar_url=$2 WHERE id=$1", id, avatarURL)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *userRepository) ClearAvatarURL(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, "UPDATE users SET avatar_url=NULL WHERE id=$1", id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}
