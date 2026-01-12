package igrpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"user-service/internal/mocks"
	userpb "user-service/proto/user"
)

func TestAreFriends(t *testing.T) {
	mockFriends := new(mocks.MockFriendRepository)
	srv := NewUserGRPCServer(mockFriends, new(mocks.MockAuthClient))

	mockFriends.On("AreFriends", mock.Anything, int64(1), int64(2)).Return(true, nil).Once()

	resp, err := srv.AreFriends(context.Background(), &userpb.AreFriendsRequest{UserId: 1, FriendId: 2})
	require.NoError(t, err)
	assert.True(t, resp.GetAreFriends())

	mockFriends.AssertExpectations(t)
}

func TestAreFriendsFalse(t *testing.T) {
	mockFriends := new(mocks.MockFriendRepository)
	srv := NewUserGRPCServer(mockFriends, new(mocks.MockAuthClient))

	mockFriends.On("AreFriends", mock.Anything, int64(2), int64(3)).Return(false, nil).Once()

	resp, err := srv.AreFriends(context.Background(), &userpb.AreFriendsRequest{UserId: 2, FriendId: 3})
	require.NoError(t, err)
	assert.False(t, resp.GetAreFriends())

	mockFriends.AssertExpectations(t)
}
