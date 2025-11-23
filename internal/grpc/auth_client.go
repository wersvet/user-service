package igrpc

import (
	"context"
	"fmt"

	authpb "user-service/proto/auth"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type AuthClient struct {
	conn   *grpc.ClientConn
	client authpb.AuthServiceClient
}

func NewAuthClient(addr string) (*AuthClient, error) {
	if addr == "" {
		return nil, fmt.Errorf("auth gRPC address is required")
	}

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to dial auth gRPC: %w", err)
	}

	return &AuthClient{
		conn:   conn,
		client: authpb.NewAuthServiceClient(conn),
	}, nil
}

func (c *AuthClient) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *AuthClient) GetUser(ctx context.Context, userID int64) (*authpb.GetUserResponse, error) {
	return c.client.GetUser(ctx, &authpb.GetUserRequest{UserId: userID})
}

func (c *AuthClient) ValidateToken(ctx context.Context, token string) (*authpb.ValidateTokenResponse, error) {
	return c.client.ValidateToken(ctx, &authpb.ValidateTokenRequest{Token: token})
}
