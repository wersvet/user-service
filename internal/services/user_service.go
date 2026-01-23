package services

import (
	"context"
	"log"

	"user-service/internal/repositories"
)

type UserService struct {
	users repositories.UserRepository
}

type UserDTO struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

func NewUserService(users repositories.UserRepository) *UserService {
	return &UserService{users: users}
}

func (s *UserService) GetUserByID(ctx context.Context, id int64) (*UserDTO, error) {
	user, err := s.users.GetByID(ctx, id)
	if err != nil {
		log.Printf("debug: failed to load user %d: %v", id, err)
		return nil, err
	}
	return &UserDTO{
		ID:        user.ID,
		Username:  user.Username,
		AvatarURL: user.AvatarURL,
	}, nil
}
