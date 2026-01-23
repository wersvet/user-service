package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"user-service/internal/telemetry"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"user-service/internal/mocks"
	"user-service/internal/models"
	"user-service/internal/services"
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
	r.DELETE("/friends/:friend_id", handler.DeleteFriend)
	return r
}

func expectAuditPublish(t *testing.T, publisher *mocks.MockPublisher, requestID, level, text string, userID *int64) {
	t.Helper()
	publisher.On("Publish", mock.Anything, telemetry.AuditRoutingKey, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		event, ok := args.Get(2).(telemetry.Envelope)
		require.True(t, ok)
		require.Equal(t, "audit_log", event.EventType)
		require.Equal(t, requestID, event.RequestID)
		require.Equal(t, level, event.Payload.Level)
		require.Equal(t, text, event.Payload.Text)
		if userID == nil {
			require.Nil(t, event.UserID)
		} else {
			require.NotNil(t, event.UserID)
			require.Equal(t, *userID, *event.UserID)
		}
	}).Once()
}

func TestSendRequestInvalidBody(t *testing.T) {
	mockPublisher := new(mocks.MockPublisher)
	emitter := telemetry.NewAuditEmitter(mockPublisher, "user-service", "local")
	handler := NewFriendHandler(new(mocks.MockFriendRepository), services.NewUserService(new(mocks.MockUserRepository)), emitter)
	router := setupFriendsRouter(handler)

	requestID := "req-1"
	userID := int64(1)
	expectAuditPublish(t, mockPublisher, requestID, "ERROR", "invalid request payload", &userID)

	req := httptest.NewRequest(http.MethodPost, "/friends/request", bytes.NewBufferString(`{"to_user_id":"bad"}`))
	req.Header.Set("X-Request-ID", requestID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	mockPublisher.AssertExpectations(t)
}

func TestSendRequestTargetNotFound(t *testing.T) {
	mockFriends := new(mocks.MockFriendRepository)
	mockUsers := new(mocks.MockUserRepository)
	userSvc := services.NewUserService(mockUsers)
	mockPublisher := new(mocks.MockPublisher)
	emitter := telemetry.NewAuditEmitter(mockPublisher, "user-service", "local")
	handler := NewFriendHandler(mockFriends, userSvc, emitter)
	router := setupFriendsRouter(handler)

	mockUsers.On("GetByID", mock.Anything, int64(2)).Return((*models.User)(nil), sql.ErrNoRows).Once()

	requestID := "req-1b"
	userID := int64(1)
	expectAuditPublish(t, mockPublisher, requestID, "ERROR", "target user not found", &userID)

	req := httptest.NewRequest(http.MethodPost, "/friends/request", bytes.NewBufferString(`{"to_user_id":2}`))
	req.Header.Set("X-Request-ID", requestID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	mockPublisher.AssertExpectations(t)
	mockUsers.AssertExpectations(t)
}

func TestSendRequestPendingExists(t *testing.T) {
	mockFriends := new(mocks.MockFriendRepository)
	mockUsers := new(mocks.MockUserRepository)
	userSvc := services.NewUserService(mockUsers)
	mockPublisher := new(mocks.MockPublisher)
	emitter := telemetry.NewAuditEmitter(mockPublisher, "user-service", "local")
	handler := NewFriendHandler(mockFriends, userSvc, emitter)
	router := setupFriendsRouter(handler)

	mockUsers.On("GetByID", mock.Anything, int64(2)).Return(&models.User{ID: 2, Username: "bob"}, nil).Once()
	mockFriends.On("HasPendingRequest", mock.Anything, int64(1), int64(2)).Return(true, nil).Once()

	requestID := "req-2"
	userID := int64(1)
	expectAuditPublish(t, mockPublisher, requestID, "ERROR", "pending friend request already exists", &userID)

	req := httptest.NewRequest(http.MethodPost, "/friends/request", bytes.NewBufferString(`{"to_user_id":2}`))
	req.Header.Set("X-Request-ID", requestID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code)
	mockFriends.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
	mockUsers.AssertExpectations(t)
}

func TestSendRequestAlreadyFriends(t *testing.T) {
	mockFriends := new(mocks.MockFriendRepository)
	mockUsers := new(mocks.MockUserRepository)
	userSvc := services.NewUserService(mockUsers)
	mockPublisher := new(mocks.MockPublisher)
	emitter := telemetry.NewAuditEmitter(mockPublisher, "user-service", "local")
	handler := NewFriendHandler(mockFriends, userSvc, emitter)
	router := setupFriendsRouter(handler)

	mockUsers.On("GetByID", mock.Anything, int64(2)).Return(&models.User{ID: 2, Username: "bob"}, nil).Once()
	mockFriends.On("HasPendingRequest", mock.Anything, int64(1), int64(2)).Return(false, nil).Once()
	mockFriends.On("AreFriends", mock.Anything, int64(1), int64(2)).Return(true, nil).Once()

	requestID := "req-3"
	userID := int64(1)
	expectAuditPublish(t, mockPublisher, requestID, "ERROR", "users are already friends", &userID)

	req := httptest.NewRequest(http.MethodPost, "/friends/request", bytes.NewBufferString(`{"to_user_id":2}`))
	req.Header.Set("X-Request-ID", requestID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code)
	mockFriends.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
	mockUsers.AssertExpectations(t)
}

func TestSendRequestSuccess(t *testing.T) {
	mockFriends := new(mocks.MockFriendRepository)
	mockUsers := new(mocks.MockUserRepository)
	userSvc := services.NewUserService(mockUsers)
	mockPublisher := new(mocks.MockPublisher)
	emitter := telemetry.NewAuditEmitter(mockPublisher, "user-service", "local")
	handler := NewFriendHandler(mockFriends, userSvc, emitter)
	router := setupFriendsRouter(handler)

	mockUsers.On("GetByID", mock.Anything, int64(2)).Return(&models.User{ID: 2, Username: "bob"}, nil).Once()
	mockFriends.On("HasPendingRequest", mock.Anything, int64(1), int64(2)).Return(false, nil).Once()
	mockFriends.On("AreFriends", mock.Anything, int64(1), int64(2)).Return(false, nil).Once()
	expected := &models.FriendRequest{ID: 5, FromUserID: 1, ToUserID: 2, Status: "pending"}
	mockFriends.On("CreateRequest", mock.Anything, int64(1), int64(2)).Return(expected, nil).Once()

	requestID := "req-4"
	userID := int64(1)
	expectAuditPublish(t, mockPublisher, requestID, "INFO", "Friend request sent to '2'", &userID)

	req := httptest.NewRequest(http.MethodPost, "/friends/request", bytes.NewBufferString(`{"to_user_id":2}`))
	req.Header.Set("X-Request-ID", requestID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	var resp models.FriendRequest
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	require.Equal(t, expected.ID, resp.ID)

	mockFriends.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
	mockUsers.AssertExpectations(t)
}

func TestAcceptRequestInvalidID(t *testing.T) {
	handler := NewFriendHandler(new(mocks.MockFriendRepository), services.NewUserService(new(mocks.MockUserRepository)), nil)
	router := setupFriendsRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/friends/requests/abc/accept", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAcceptRequestSuccess(t *testing.T) {
	mockFriends := new(mocks.MockFriendRepository)
	mockPublisher := new(mocks.MockPublisher)
	emitter := telemetry.NewAuditEmitter(mockPublisher, "user-service", "local")
	handler := NewFriendHandler(mockFriends, services.NewUserService(new(mocks.MockUserRepository)), emitter)
	router := setupFriendsRouter(handler)

	mockFriends.On("AcceptRequest", mock.Anything, int64(7), int64(1)).Return(nil).Once()

	requestID := "req-5"
	userID := int64(1)
	expectAuditPublish(t, mockPublisher, requestID, "INFO", "Friend request accepted", &userID)

	req := httptest.NewRequest(http.MethodPost, "/friends/requests/7/accept", nil)
	req.Header.Set("X-Request-ID", requestID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	mockFriends.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}

func TestDeleteFriend(t *testing.T) {
	mockFriends := new(mocks.MockFriendRepository)
	mockUsers := new(mocks.MockUserRepository)
	userSvc := services.NewUserService(mockUsers)
	handler := NewFriendHandler(mockFriends, userSvc, nil)
	router := setupFriendsRouter(handler)

	mockUsers.On("GetByID", mock.Anything, int64(2)).Return(&models.User{ID: 2, Username: "bob"}, nil).Twice()
	mockFriends.On("AreFriends", mock.Anything, int64(1), int64(2)).Return(true, nil).Once()
	mockFriends.On("DeleteFriendship", mock.Anything, int64(1), int64(2)).Return(nil).Once()
	mockFriends.On("AreFriends", mock.Anything, int64(1), int64(2)).Return(false, nil).Once()

	req := httptest.NewRequest(http.MethodDelete, "/friends/2", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code)

	req = httptest.NewRequest(http.MethodDelete, "/friends/2", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Contains(t, []int{http.StatusBadRequest, http.StatusNotFound}, rec.Code)

	mockFriends.AssertExpectations(t)
	mockUsers.AssertExpectations(t)
}

func TestRejectRequestSuccess(t *testing.T) {
	mockFriends := new(mocks.MockFriendRepository)
	mockPublisher := new(mocks.MockPublisher)
	emitter := telemetry.NewAuditEmitter(mockPublisher, "user-service", "local")
	handler := NewFriendHandler(mockFriends, services.NewUserService(new(mocks.MockUserRepository)), emitter)

	router := setupFriendsRouter(handler)

	mockFriends.On("RejectRequest", mock.Anything, int64(8), int64(1)).Return(nil).Once()

	requestID := "req-6"
	userID := int64(1)
	expectAuditPublish(t, mockPublisher, requestID, "INFO", "Friend request rejected", &userID)

	req := httptest.NewRequest(http.MethodPost, "/friends/requests/8/reject", nil)
	req.Header.Set("X-Request-ID", requestID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	mockFriends.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}

func TestListFriendsSuccess(t *testing.T) {
	mockFriends := new(mocks.MockFriendRepository)
	mockUsers := new(mocks.MockUserRepository)
	userSvc := services.NewUserService(mockUsers)
	handler := NewFriendHandler(mockFriends, userSvc, nil)
	router := setupFriendsRouter(handler)

	mockFriends.On("ListFriends", mock.Anything, int64(1)).Return([]int64{2, 3}, nil).Once()
	mockUsers.On("GetByID", mock.Anything, int64(2)).Return(&models.User{ID: 2, Username: "bob"}, nil).Once()
	mockUsers.On("GetByID", mock.Anything, int64(3)).Return(&models.User{ID: 3, Username: "carol"}, nil).Once()

	req := httptest.NewRequest(http.MethodGet, "/friends", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp []services.UserDTO
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	require.Len(t, resp, 2)
	require.Equal(t, int64(2), resp[0].ID)
	require.Equal(t, int64(3), resp[1].ID)

	mockFriends.AssertExpectations(t)
	mockUsers.AssertExpectations(t)
}

func TestListIncomingSuccess(t *testing.T) {
	mockFriends := new(mocks.MockFriendRepository)
	mockUsers := new(mocks.MockUserRepository)
	userSvc := services.NewUserService(mockUsers)
	handler := NewFriendHandler(mockFriends, userSvc, nil)
	router := setupFriendsRouter(handler)

	incoming := []models.FriendRequest{{ID: 11, FromUserID: 2}, {ID: 12, FromUserID: 3}}
	mockFriends.On("GetIncomingRequests", mock.Anything, int64(1)).Return(incoming, nil).Once()
	mockUsers.On("GetByID", mock.Anything, int64(2)).Return(&models.User{ID: 2, Username: "bob"}, nil).Once()
	mockUsers.On("GetByID", mock.Anything, int64(3)).Return(&models.User{ID: 3, Username: "carol"}, nil).Once()

	req := httptest.NewRequest(http.MethodGet, "/friends/requests/incoming", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp []map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	require.Len(t, resp, 2)
	require.Equal(t, float64(11), resp[0]["id"])
	require.Equal(t, "bob", resp[0]["from_username"])

	mockFriends.AssertExpectations(t)
	mockUsers.AssertExpectations(t)
}

func TestHandleDecisionNotFound(t *testing.T) {
	mockFriends := new(mocks.MockFriendRepository)
	mockPublisher := new(mocks.MockPublisher)
	emitter := telemetry.NewAuditEmitter(mockPublisher, "user-service", "local")
	handler := NewFriendHandler(mockFriends, services.NewUserService(new(mocks.MockUserRepository)), emitter)
	router := setupFriendsRouter(handler)

	mockFriends.On("AcceptRequest", mock.Anything, int64(15), int64(1)).Return(sql.ErrNoRows).Once()

	requestID := "req-7"
	userID := int64(1)
	expectAuditPublish(t, mockPublisher, requestID, "ERROR", "friend request not found", &userID)

	req := httptest.NewRequest(http.MethodPost, "/friends/requests/15/accept", nil)
	req.Header.Set("X-Request-ID", requestID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	mockFriends.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}

func TestRejectRequestNotFound(t *testing.T) {
	mockFriends := new(mocks.MockFriendRepository)
	mockPublisher := new(mocks.MockPublisher)
	emitter := telemetry.NewAuditEmitter(mockPublisher, "user-service", "local")
	handler := NewFriendHandler(mockFriends, services.NewUserService(new(mocks.MockUserRepository)), emitter)
	router := setupFriendsRouter(handler)

	mockFriends.On("RejectRequest", mock.Anything, int64(18), int64(1)).Return(sql.ErrNoRows).Once()

	requestID := "req-7b"
	userID := int64(1)
	expectAuditPublish(t, mockPublisher, requestID, "ERROR", "friend request not found", &userID)

	req := httptest.NewRequest(http.MethodPost, "/friends/requests/18/reject", nil)
	req.Header.Set("X-Request-ID", requestID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	mockFriends.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}

func TestHandleDecisionInternalError(t *testing.T) {
	mockFriends := new(mocks.MockFriendRepository)
	mockPublisher := new(mocks.MockPublisher)
	emitter := telemetry.NewAuditEmitter(mockPublisher, "user-service", "local")
	handler := NewFriendHandler(mockFriends, services.NewUserService(new(mocks.MockUserRepository)), emitter)
	router := setupFriendsRouter(handler)

	mockFriends.On("RejectRequest", mock.Anything, int64(16), int64(1)).Return(errors.New("db down")).Once()

	requestID := "req-8"
	userID := int64(1)
	expectAuditPublish(t, mockPublisher, requestID, "ERROR", "internal error", &userID)

	req := httptest.NewRequest(http.MethodPost, "/friends/requests/16/reject", nil)
	req.Header.Set("X-Request-ID", requestID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	mockFriends.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}
