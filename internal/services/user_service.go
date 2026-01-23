package services

import (
	"context"

	"user-service/internal/repositories"
	authpb "user-service/proto/auth"
)

// AuthClient describes the subset of the auth gRPC client used by the service.
type AuthClient interface {
	GetUser(ctx context.Context, userID int64) (*authpb.GetUserResponse, error)
}

type UserService struct {
	authClient AuthClient
	users      repositories.UserRepository
}

type UserDTO struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

func NewUserService(authClient AuthClient, users repositories.UserRepository) *UserService {
	return &UserService{authClient: authClient, users: users}
}

func (s *UserService) GetUserByID(ctx context.Context, id int64) (*UserDTO, error) {
	user, err := s.authClient.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}
	avatarURL := ""
	if s.users != nil {
		avatarURL, err = s.users.GetAvatarURL(ctx, id)
		if err != nil {
			return nil, err
		}
	}
	return &UserDTO{
		ID:        user.Id,
		Username:  user.Username,
		AvatarURL: avatarURL,
		CreatedAt: user.CreatedAt,
	}, nil
}
