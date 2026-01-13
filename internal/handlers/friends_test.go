package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"user-service/internal/mocks"
	"user-service/internal/models"
	"user-service/internal/services"
	"user-service/internal/telemetry"
	authpb "user-service/proto/auth"
)

func setupFriendsRouter(handler *FriendHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", int64(1))
		c.Next()
	})
	r.POST("/friends/request", handler.SendRequest)
	r.GET("/friends/requests/incoming", handler.ListIncoming)
	r.POST("/friends/requests/:id/accept", handler.AcceptRequest)
	r.POST("/friends/requests/:id/reject", handler.RejectRequest)
	r.GET("/friends", handler.ListFriends)
	return r
}

func TestSendRequestInvalidBody(t *testing.T) {
	handler := NewFriendHandler(new(mocks.MockFriendRepository), services.NewUserService(new(mocks.MockAuthClient)), telemetry.NewNoopPublisher(), telemetry.Config{})
	router := setupFriendsRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/friends/request", bytes.NewBufferString(`{"to_user_id":"bad"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSendRequestPendingExists(t *testing.T) {
	mockAuth := new(mocks.MockAuthClient)
	mockFriends := new(mocks.MockFriendRepository)
	userSvc := services.NewUserService(mockAuth)
	handler := NewFriendHandler(mockFriends, userSvc, telemetry.NewNoopPublisher(), telemetry.Config{})
	router := setupFriendsRouter(handler)

	mockAuth.On("GetUser", mock.Anything, int64(2)).Return(&authpb.GetUserResponse{Id: 2, Username: "bob"}, nil).Once()
	mockFriends.On("HasPendingRequest", mock.Anything, int64(1), int64(2)).Return(true, nil).Once()

	req := httptest.NewRequest(http.MethodPost, "/friends/request", bytes.NewBufferString(`{"to_user_id":2}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	mockAuth.AssertExpectations(t)
	mockFriends.AssertExpectations(t)
}

func TestSendRequestAlreadyFriends(t *testing.T) {
	mockAuth := new(mocks.MockAuthClient)
	mockFriends := new(mocks.MockFriendRepository)
	userSvc := services.NewUserService(mockAuth)
	handler := NewFriendHandler(mockFriends, userSvc, telemetry.NewNoopPublisher(), telemetry.Config{})
	router := setupFriendsRouter(handler)

	mockAuth.On("GetUser", mock.Anything, int64(2)).Return(&authpb.GetUserResponse{Id: 2, Username: "bob"}, nil).Once()
	mockFriends.On("HasPendingRequest", mock.Anything, int64(1), int64(2)).Return(false, nil).Once()
	mockFriends.On("AreFriends", mock.Anything, int64(1), int64(2)).Return(true, nil).Once()

	req := httptest.NewRequest(http.MethodPost, "/friends/request", bytes.NewBufferString(`{"to_user_id":2}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	mockAuth.AssertExpectations(t)
	mockFriends.AssertExpectations(t)
}

func TestSendRequestSuccess(t *testing.T) {
	mockAuth := new(mocks.MockAuthClient)
	mockFriends := new(mocks.MockFriendRepository)
	userSvc := services.NewUserService(mockAuth)
	handler := NewFriendHandler(mockFriends, userSvc, telemetry.NewNoopPublisher(), telemetry.Config{})
	router := setupFriendsRouter(handler)

	mockAuth.On("GetUser", mock.Anything, int64(2)).Return(&authpb.GetUserResponse{Id: 2, Username: "bob"}, nil).Once()
	mockFriends.On("HasPendingRequest", mock.Anything, int64(1), int64(2)).Return(false, nil).Once()
	mockFriends.On("AreFriends", mock.Anything, int64(1), int64(2)).Return(false, nil).Once()
	expected := &models.FriendRequest{ID: 5, FromUserID: 1, ToUserID: 2, Status: "pending"}
	mockFriends.On("CreateRequest", mock.Anything, int64(1), int64(2)).Return(expected, nil).Once()

	req := httptest.NewRequest(http.MethodPost, "/friends/request", bytes.NewBufferString(`{"to_user_id":2}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	var resp models.FriendRequest
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	require.Equal(t, expected.ID, resp.ID)

	mockAuth.AssertExpectations(t)
	mockFriends.AssertExpectations(t)
}

func TestAcceptRequestInvalidID(t *testing.T) {
	handler := NewFriendHandler(new(mocks.MockFriendRepository), services.NewUserService(new(mocks.MockAuthClient)), telemetry.NewNoopPublisher(), telemetry.Config{})
	router := setupFriendsRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/friends/requests/abc/accept", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAcceptRequestSuccess(t *testing.T) {
	mockFriends := new(mocks.MockFriendRepository)
	handler := NewFriendHandler(mockFriends, services.NewUserService(new(mocks.MockAuthClient)), telemetry.NewNoopPublisher(), telemetry.Config{})
	router := setupFriendsRouter(handler)

	mockFriends.On("AcceptRequest", mock.Anything, int64(7), int64(1)).Return(nil).Once()

	req := httptest.NewRequest(http.MethodPost, "/friends/requests/7/accept", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	mockFriends.AssertExpectations(t)
}

func TestRejectRequestSuccess(t *testing.T) {
	mockFriends := new(mocks.MockFriendRepository)
	handler := NewFriendHandler(mockFriends, services.NewUserService(new(mocks.MockAuthClient)), telemetry.NewNoopPublisher(), telemetry.Config{})
	router := setupFriendsRouter(handler)

	mockFriends.On("RejectRequest", mock.Anything, int64(8), int64(1)).Return(nil).Once()

	req := httptest.NewRequest(http.MethodPost, "/friends/requests/8/reject", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	mockFriends.AssertExpectations(t)
}

func TestListFriendsSuccess(t *testing.T) {
	mockAuth := new(mocks.MockAuthClient)
	mockFriends := new(mocks.MockFriendRepository)
	userSvc := services.NewUserService(mockAuth)
	handler := NewFriendHandler(mockFriends, userSvc, telemetry.NewNoopPublisher(), telemetry.Config{})
	router := setupFriendsRouter(handler)

	mockFriends.On("ListFriends", mock.Anything, int64(1)).Return([]int64{2, 3}, nil).Once()
	mockAuth.On("GetUser", mock.Anything, int64(2)).Return(&authpb.GetUserResponse{Id: 2, Username: "bob"}, nil).Once()
	mockAuth.On("GetUser", mock.Anything, int64(3)).Return(&authpb.GetUserResponse{Id: 3, Username: "carol"}, nil).Once()

	req := httptest.NewRequest(http.MethodGet, "/friends", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp []services.UserDTO
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	require.Len(t, resp, 2)
	require.Equal(t, int64(2), resp[0].ID)
	require.Equal(t, int64(3), resp[1].ID)

	mockAuth.AssertExpectations(t)
	mockFriends.AssertExpectations(t)
}

func TestListIncomingSuccess(t *testing.T) {
	mockAuth := new(mocks.MockAuthClient)
	mockFriends := new(mocks.MockFriendRepository)
	userSvc := services.NewUserService(mockAuth)
	handler := NewFriendHandler(mockFriends, userSvc, telemetry.NewNoopPublisher(), telemetry.Config{})
	router := setupFriendsRouter(handler)

	incoming := []models.FriendRequest{{ID: 11, FromUserID: 2}, {ID: 12, FromUserID: 3}}
	mockFriends.On("GetIncomingRequests", mock.Anything, int64(1)).Return(incoming, nil).Once()
	mockAuth.On("GetUser", mock.Anything, int64(2)).Return(&authpb.GetUserResponse{Id: 2, Username: "bob"}, nil).Once()
	mockAuth.On("GetUser", mock.Anything, int64(3)).Return(&authpb.GetUserResponse{Id: 3, Username: "carol"}, nil).Once()

	req := httptest.NewRequest(http.MethodGet, "/friends/requests/incoming", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp []map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	require.Len(t, resp, 2)
	require.Equal(t, float64(11), resp[0]["id"])
	require.Equal(t, "bob", resp[0]["from_username"])

	mockAuth.AssertExpectations(t)
	mockFriends.AssertExpectations(t)
}

func TestHandleDecisionNotFound(t *testing.T) {
	mockFriends := new(mocks.MockFriendRepository)
	handler := NewFriendHandler(mockFriends, services.NewUserService(new(mocks.MockAuthClient)), telemetry.NewNoopPublisher(), telemetry.Config{})
	router := setupFriendsRouter(handler)

	mockFriends.On("AcceptRequest", mock.Anything, int64(15), int64(1)).Return(sql.ErrNoRows).Once()

	req := httptest.NewRequest(http.MethodPost, "/friends/requests/15/accept", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	mockFriends.AssertExpectations(t)
}

func TestHandleDecisionInternalError(t *testing.T) {
	mockFriends := new(mocks.MockFriendRepository)
	handler := NewFriendHandler(mockFriends, services.NewUserService(new(mocks.MockAuthClient)), telemetry.NewNoopPublisher(), telemetry.Config{})
	router := setupFriendsRouter(handler)

	mockFriends.On("RejectRequest", mock.Anything, int64(16), int64(1)).Return(errors.New("db down")).Once()

	req := httptest.NewRequest(http.MethodPost, "/friends/requests/16/reject", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	mockFriends.AssertExpectations(t)
}

func TestSendRequestPublishesAuditEventSuccess(t *testing.T) {
	mockAuth := new(mocks.MockAuthClient)
	mockFriends := new(mocks.MockFriendRepository)
	mockPublisher := new(mocks.MockTelemetryPublisher)
	userSvc := services.NewUserService(mockAuth)
	handler := NewFriendHandler(mockFriends, userSvc, mockPublisher, telemetry.Config{Environment: "test", ServiceName: "user-service"})
	router := setupFriendsRouter(handler)

	mockAuth.On("GetUser", mock.Anything, int64(2)).Return(&authpb.GetUserResponse{Id: 2, Username: "bob"}, nil).Once()
	mockFriends.On("HasPendingRequest", mock.Anything, int64(1), int64(2)).Return(false, nil).Once()
	mockFriends.On("AreFriends", mock.Anything, int64(1), int64(2)).Return(false, nil).Once()
	expected := &models.FriendRequest{ID: 5, FromUserID: 1, ToUserID: 2, Status: "pending"}
	mockFriends.On("CreateRequest", mock.Anything, int64(1), int64(2)).Return(expected, nil).Once()
	mockPublisher.On("Publish", mock.Anything, telemetry.AuditFriendsKey, mock.MatchedBy(func(event telemetry.Envelope) bool {
		payload, ok := event.Payload.(telemetry.FriendRequestSentPayload)
		if !ok {
			return false
		}
		return event.EventType == telemetry.AuditEventType &&
			event.RequestID == "req-123" &&
			payload.Action == "friend_request_sent" &&
			payload.Result == "success" &&
			payload.Status == "pending" &&
			payload.RequestID == "5"
	})).Return(nil).Once()

	req := httptest.NewRequest(http.MethodPost, "/friends/request", bytes.NewBufferString(`{"to_user_id":2}`))
	req.Header.Set("X-Request-ID", "req-123")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	mockAuth.AssertExpectations(t)
	mockFriends.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}

func TestSendRequestPublishesAuditEventError(t *testing.T) {
	mockAuth := new(mocks.MockAuthClient)
	mockFriends := new(mocks.MockFriendRepository)
	mockPublisher := new(mocks.MockTelemetryPublisher)
	userSvc := services.NewUserService(mockAuth)
	handler := NewFriendHandler(mockFriends, userSvc, mockPublisher, telemetry.Config{Environment: "test", ServiceName: "user-service"})
	router := setupFriendsRouter(handler)

	mockAuth.On("GetUser", mock.Anything, int64(2)).Return(&authpb.GetUserResponse{Id: 2, Username: "bob"}, nil).Once()
	mockFriends.On("HasPendingRequest", mock.Anything, int64(1), int64(2)).Return(true, nil).Once()
	mockPublisher.On("Publish", mock.Anything, telemetry.AuditFriendsKey, mock.MatchedBy(func(event telemetry.Envelope) bool {
		payload, ok := event.Payload.(telemetry.FriendRequestSentPayload)
		if !ok {
			return false
		}
		return event.EventType == telemetry.AuditEventType &&
			event.RequestID == "req-456" &&
			payload.Action == "friend_request_sent" &&
			payload.Result == "error" &&
			payload.Error == "pending friend request already exists"
	})).Return(nil).Once()

	req := httptest.NewRequest(http.MethodPost, "/friends/request", bytes.NewBufferString(`{"to_user_id":2}`))
	req.Header.Set("X-Request-ID", "req-456")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	mockAuth.AssertExpectations(t)
	mockFriends.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}

func TestAcceptRequestPublishesAuditEventSuccess(t *testing.T) {
	mockFriends := new(mocks.MockFriendRepository)
	mockPublisher := new(mocks.MockTelemetryPublisher)
	handler := NewFriendHandler(mockFriends, services.NewUserService(new(mocks.MockAuthClient)), mockPublisher, telemetry.Config{Environment: "test", ServiceName: "user-service"})
	router := setupFriendsRouter(handler)

	mockFriends.On("AcceptRequest", mock.Anything, int64(7), int64(1)).Return(nil).Once()
	mockPublisher.On("Publish", mock.Anything, telemetry.AuditFriendsKey, mock.MatchedBy(func(event telemetry.Envelope) bool {
		payload, ok := event.Payload.(telemetry.FriendRequestDecisionPayload)
		if !ok {
			return false
		}
		return event.EventType == telemetry.AuditEventType &&
			event.RequestID == "req-789" &&
			payload.Action == "friend_request_accepted" &&
			payload.Result == "success" &&
			payload.Status == "accepted" &&
			payload.FriendRequestID == "7"
	})).Return(nil).Once()

	req := httptest.NewRequest(http.MethodPost, "/friends/requests/7/accept", nil)
	req.Header.Set("X-Request-ID", "req-789")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	mockFriends.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}

func TestAcceptRequestPublishesAuditEventError(t *testing.T) {
	mockFriends := new(mocks.MockFriendRepository)
	mockPublisher := new(mocks.MockTelemetryPublisher)
	handler := NewFriendHandler(mockFriends, services.NewUserService(new(mocks.MockAuthClient)), mockPublisher, telemetry.Config{Environment: "test", ServiceName: "user-service"})
	router := setupFriendsRouter(handler)

	mockPublisher.On("Publish", mock.Anything, telemetry.AuditFriendsKey, mock.MatchedBy(func(event telemetry.Envelope) bool {
		payload, ok := event.Payload.(telemetry.FriendRequestDecisionPayload)
		if !ok {
			return false
		}
		return event.EventType == telemetry.AuditEventType &&
			event.RequestID == "req-999" &&
			payload.Action == "friend_request_accepted" &&
			payload.Result == "error" &&
			payload.Error == "invalid request id" &&
			payload.FriendRequestID == "abc"
	})).Return(nil).Once()

	req := httptest.NewRequest(http.MethodPost, "/friends/requests/abc/accept", nil)
	req.Header.Set("X-Request-ID", "req-999")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	mockPublisher.AssertExpectations(t)
}

func TestRejectRequestPublishesAuditEventSuccess(t *testing.T) {
	mockFriends := new(mocks.MockFriendRepository)
	mockPublisher := new(mocks.MockTelemetryPublisher)
	handler := NewFriendHandler(mockFriends, services.NewUserService(new(mocks.MockAuthClient)), mockPublisher, telemetry.Config{Environment: "test", ServiceName: "user-service"})
	router := setupFriendsRouter(handler)

	mockFriends.On("RejectRequest", mock.Anything, int64(8), int64(1)).Return(nil).Once()
	mockPublisher.On("Publish", mock.Anything, telemetry.AuditFriendsKey, mock.MatchedBy(func(event telemetry.Envelope) bool {
		payload, ok := event.Payload.(telemetry.FriendRequestDecisionPayload)
		if !ok {
			return false
		}
		return event.EventType == telemetry.AuditEventType &&
			event.RequestID == "req-321" &&
			payload.Action == "friend_request_rejected" &&
			payload.Result == "success" &&
			payload.Status == "rejected" &&
			payload.FriendRequestID == "8"
	})).Return(nil).Once()

	req := httptest.NewRequest(http.MethodPost, "/friends/requests/8/reject", nil)
	req.Header.Set("X-Request-ID", "req-321")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	mockFriends.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}

func TestRejectRequestPublishesAuditEventError(t *testing.T) {
	mockFriends := new(mocks.MockFriendRepository)
	mockPublisher := new(mocks.MockTelemetryPublisher)
	handler := NewFriendHandler(mockFriends, services.NewUserService(new(mocks.MockAuthClient)), mockPublisher, telemetry.Config{Environment: "test", ServiceName: "user-service"})
	router := setupFriendsRouter(handler)

	mockPublisher.On("Publish", mock.Anything, telemetry.AuditFriendsKey, mock.MatchedBy(func(event telemetry.Envelope) bool {
		payload, ok := event.Payload.(telemetry.FriendRequestDecisionPayload)
		if !ok {
			return false
		}
		return event.EventType == telemetry.AuditEventType &&
			event.RequestID == "req-654" &&
			payload.Action == "friend_request_rejected" &&
			payload.Result == "error" &&
			payload.Error == "invalid request id" &&
			payload.FriendRequestID == "abc"
	})).Return(nil).Once()

	req := httptest.NewRequest(http.MethodPost, "/friends/requests/abc/reject", nil)
	req.Header.Set("X-Request-ID", "req-654")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	mockPublisher.AssertExpectations(t)
}
