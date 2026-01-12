package services

import (
	"context"

	authpb "user-service/proto/auth"
)

// AuthClient describes the subset of the auth gRPC client used by the service.
type AuthClient interface {
	GetUser(ctx context.Context, userID int64) (*authpb.GetUserResponse, error)
}

type UserService struct {
	authClient AuthClient
}

type UserDTO struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	CreatedAt string `json:"created_at,omitempty"`
}

func NewUserService(authClient AuthClient) *UserService {
	return &UserService{authClient: authClient}
}

func (s *UserService) GetUserByID(ctx context.Context, id int64) (*UserDTO, error) {
	user, err := s.authClient.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}
	return &UserDTO{ID: user.Id, Username: user.Username, CreatedAt: user.CreatedAt}, nil
}
