package services

import (
	"context"
	"testing"

	authpb "user-service/proto/auth"
)

type fakeAuthClient struct{}

func (f *fakeAuthClient) GetUser(ctx context.Context, userID int64) (*authpb.GetUserResponse, error) {
	return &authpb.GetUserResponse{Id: userID, Username: "test"}, nil
}

func TestGetUserByID_Success(t *testing.T) {
	svc := NewUserService(&fakeAuthClient{})

	user, err := svc.GetUserByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user == nil {
		t.Fatalf("expected user, got nil")
	}
}
