package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"user-service/internal/mocks"
	"user-service/internal/models"
)

func TestGetUserByIDSuccess(t *testing.T) {
	t.Parallel()

	mockUsers := new(mocks.MockUserRepository)
	userSvc := NewUserService(mockUsers)

	expectedUser := &models.User{ID: 10, Username: "john", AvatarURL: "/uploads/avatars/10/avatar.png"}
	mockUsers.On("GetByID", mock.Anything, int64(10)).Return(expectedUser, nil).Once()

	dto, err := userSvc.GetUserByID(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, expectedUser.ID, dto.ID)
	require.Equal(t, expectedUser.Username, dto.Username)
	require.Equal(t, "/uploads/avatars/10/avatar.png", dto.AvatarURL)
	require.Empty(t, dto.CreatedAt)
	mockUsers.AssertExpectations(t)
}

func TestGetUserByIDAuthError(t *testing.T) {
	t.Parallel()

	mockUsers := new(mocks.MockUserRepository)
	userSvc := NewUserService(mockUsers)

	mockErr := errors.New("auth down")
	mockUsers.On("GetByID", mock.Anything, int64(5)).Return((*models.User)(nil), mockErr).Once()

	dto, err := userSvc.GetUserByID(context.Background(), 5)
	require.Nil(t, dto)
	require.ErrorIs(t, err, mockErr)

	mockUsers.AssertExpectations(t)
}
