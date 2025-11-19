package models

type User struct {
	ID       int64  `db:"id" json:"id"`
	Username string `db:"username" json:"username"`
}
