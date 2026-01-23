package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"user-service/internal/mocks"
	authpb "user-service/proto/auth"
)

func TestGetUserByIDSuccess(t *testing.T) {
	t.Parallel()

	mockAuth := new(mocks.MockAuthClient)
	mockUsers := new(mocks.MockUserRepository)
	userSvc := NewUserService(mockAuth, mockUsers)

	expectedResp := &authpb.GetUserResponse{Id: 10, Username: "john", CreatedAt: "now"}
	mockAuth.On("GetUser", mock.Anything, int64(10)).Return(expectedResp, nil).Once()
	mockUsers.On("GetAvatarURL", mock.Anything, int64(10)).Return("/uploads/avatars/10/avatar.png", nil).Once()

	dto, err := userSvc.GetUserByID(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, expectedResp.Id, dto.ID)
	require.Equal(t, expectedResp.Username, dto.Username)
	require.Equal(t, "/uploads/avatars/10/avatar.png", dto.AvatarURL)
	require.Equal(t, expectedResp.CreatedAt, dto.CreatedAt)

	mockAuth.AssertExpectations(t)
	mockUsers.AssertExpectations(t)
}

func TestGetUserByIDAuthError(t *testing.T) {
	t.Parallel()

	mockAuth := new(mocks.MockAuthClient)
	userSvc := NewUserService(mockAuth, new(mocks.MockUserRepository))

	mockErr := errors.New("auth down")
	mockAuth.On("GetUser", mock.Anything, int64(5)).Return((*authpb.GetUserResponse)(nil), mockErr).Once()

	dto, err := userSvc.GetUserByID(context.Background(), 5)
	require.Nil(t, dto)
	require.ErrorIs(t, err, mockErr)

	mockAuth.AssertExpectations(t)
}
