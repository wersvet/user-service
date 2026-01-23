package models

type User struct {
	ID        int64  `db:"id" json:"id"`
	Username  string `db:"username" json:"username"`
	AvatarURL string `db:"avatar_url" json:"avatar_url"`
}
