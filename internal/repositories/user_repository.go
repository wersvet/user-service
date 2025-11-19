package repositories

import (
	"context"

	"github.com/jmoiron/sqlx"

	"user-service/internal/models"
)

type UserRepository interface {
	GetByID(ctx context.Context, id int64) (*models.User, error)
}

type userRepository struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) GetByID(ctx context.Context, id int64) (*models.User, error) {
	var user models.User
	err := r.db.GetContext(ctx, &user, "SELECT id, username FROM users WHERE id=$1", id)
	if err != nil {
		return nil, err
	}
	return &user, nil
}
