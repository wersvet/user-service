package igrpc

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"user-service/internal/mocks"
	authpb "user-service/proto/auth"
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

func TestBulkUsersSuccess(t *testing.T) {
	mockAuth := new(mocks.MockAuthClient)
	srv := NewUserGRPCServer(new(mocks.MockFriendRepository), mockAuth)

	mockAuth.On("GetUser", mock.Anything, int64(1)).Return(&authpb.GetUserResponse{Id: 1, Username: "a"}, nil).Once()
	mockAuth.On("GetUser", mock.Anything, int64(2)).Return(&authpb.GetUserResponse{Id: 2, Username: "b"}, nil).Once()

	resp, err := srv.BulkUsers(context.Background(), &userpb.BulkUsersRequest{Ids: []int64{1, 2}})
	require.NoError(t, err)
	require.Len(t, resp.GetUsers(), 2)
	assert.Equal(t, int64(1), resp.GetUsers()[0].GetId())
	assert.Equal(t, int64(2), resp.GetUsers()[1].GetId())

	mockAuth.AssertExpectations(t)
}

func TestBulkUsersError(t *testing.T) {
	mockAuth := new(mocks.MockAuthClient)
	srv := NewUserGRPCServer(new(mocks.MockFriendRepository), mockAuth)

	mockAuth.On("GetUser", mock.Anything, int64(1)).Return(&authpb.GetUserResponse{Id: 1, Username: "a"}, nil).Once()
	mockAuth.On("GetUser", mock.Anything, int64(2)).Return((*authpb.GetUserResponse)(nil), errors.New("auth fail")).Once()

	resp, err := srv.BulkUsers(context.Background(), &userpb.BulkUsersRequest{Ids: []int64{1, 2}})
	require.Nil(t, resp)
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))

	mockAuth.AssertExpectations(t)
}
