package mocks

import (
	"context"
	"database/sql"

	"github.com/stretchr/testify/mock"

	"user-service/internal/models"
	"user-service/internal/rabbitmq"
	authpb "user-service/proto/auth"
)

// MockAuthClient mocks the auth gRPC client interactions.
type MockAuthClient struct {
	mock.Mock
}

func (m *MockAuthClient) GetUser(ctx context.Context, userID int64) (*authpb.GetUserResponse, error) {
	args := m.Called(ctx, userID)
	var resp *authpb.GetUserResponse
	if val := args.Get(0); val != nil {
		resp = val.(*authpb.GetUserResponse)
	}
	return resp, args.Error(1)
}

func (m *MockAuthClient) ValidateToken(ctx context.Context, token string) (*authpb.ValidateTokenResponse, error) {
	args := m.Called(ctx, token)
	var resp *authpb.ValidateTokenResponse
	if val := args.Get(0); val != nil {
		resp = val.(*authpb.ValidateTokenResponse)
	}
	return resp, args.Error(1)
}

// MockFriendRepository mocks FriendRepository behavior for handlers and services.
type MockFriendRepository struct {
	mock.Mock
}

func (m *MockFriendRepository) CreateRequest(ctx context.Context, fromUserID, toUserID int64) (*models.FriendRequest, error) {
	args := m.Called(ctx, fromUserID, toUserID)
	var req *models.FriendRequest
	if val := args.Get(0); val != nil {
		req = val.(*models.FriendRequest)
	}
	return req, args.Error(1)
}

func (m *MockFriendRepository) GetIncomingRequests(ctx context.Context, userID int64) ([]models.FriendRequest, error) {
	args := m.Called(ctx, userID)
	var reqs []models.FriendRequest
	if val := args.Get(0); val != nil {
		reqs = val.([]models.FriendRequest)
	}
	return reqs, args.Error(1)
}

func (m *MockFriendRepository) AcceptRequest(ctx context.Context, requestID, userID int64) error {
	args := m.Called(ctx, requestID, userID)
	return args.Error(0)
}

func (m *MockFriendRepository) RejectRequest(ctx context.Context, requestID, userID int64) error {
	args := m.Called(ctx, requestID, userID)
	return args.Error(0)
}

func (m *MockFriendRepository) ListFriends(ctx context.Context, userID int64) ([]int64, error) {
	args := m.Called(ctx, userID)
	var friends []int64
	if val := args.Get(0); val != nil {
		friends = val.([]int64)
	}
	return friends, args.Error(1)
}

func (m *MockFriendRepository) HasPendingRequest(ctx context.Context, fromUserID, toUserID int64) (bool, error) {
	args := m.Called(ctx, fromUserID, toUserID)
	return args.Bool(0), args.Error(1)
}

func (m *MockFriendRepository) AreFriends(ctx context.Context, userID, otherID int64) (bool, error) {
	args := m.Called(ctx, userID, otherID)
	return args.Bool(0), args.Error(1)
}

// Compile-time assertions
var _ interface {
	GetUser(context.Context, int64) (*authpb.GetUserResponse, error)
	ValidateToken(context.Context, string) (*authpb.ValidateTokenResponse, error)
} = (*MockAuthClient)(nil)

var _ interface {
	CreateRequest(context.Context, int64, int64) (*models.FriendRequest, error)
	GetIncomingRequests(context.Context, int64) ([]models.FriendRequest, error)
	AcceptRequest(context.Context, int64, int64) error
	RejectRequest(context.Context, int64, int64) error
	ListFriends(context.Context, int64) ([]int64, error)
	HasPendingRequest(context.Context, int64, int64) (bool, error)
	AreFriends(context.Context, int64, int64) (bool, error)
} = (*MockFriendRepository)(nil)

// MockPublisher mocks RabbitMQ publisher behavior for telemetry.
type MockPublisher struct {
	mock.Mock
}

func (m *MockPublisher) Publish(ctx context.Context, routingKey string, event any) error {
	args := m.Called(ctx, routingKey, event)
	return args.Error(0)
}

func (m *MockPublisher) Close() error {
	args := m.Called()
	return args.Error(0)
}

var _ rabbitmq.Publisher = (*MockPublisher)(nil)

// Additional compile-time assertion against sql.ErrNoRows usages.
var _ = sql.ErrNoRows
