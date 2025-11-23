package services

import (
	"context"

	grpcsvc "user-service/internal/grpc"
)

type UserService struct {
	authClient *grpcsvc.AuthClient
}

type UserDTO struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	CreatedAt string `json:"created_at,omitempty"`
}

func NewUserService(authClient *grpcsvc.AuthClient) *UserService {
	return &UserService{authClient: authClient}
}

func (s *UserService) GetUserByID(ctx context.Context, id int64) (*UserDTO, error) {
	user, err := s.authClient.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}
	return &UserDTO{ID: user.Id, Username: user.Username, CreatedAt: user.CreatedAt}, nil
}
