package models

import "time"

type FriendRequest struct {
	ID         int64     `db:"id" json:"id"`
	FromUserID int64     `db:"from_user_id" json:"from_user_id"`
	ToUserID   int64     `db:"to_user_id" json:"to_user_id"`
	Status     string    `db:"status" json:"status"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
}

type Friendship struct {
	ID       int64 `db:"id" json:"id"`
	UserID   int64 `db:"user_id" json:"user_id"`
	FriendID int64 `db:"friend_id" json:"friend_id"`
}
