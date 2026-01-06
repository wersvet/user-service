package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"user-service/internal/mocks"
	"user-service/internal/models"
	"user-service/internal/services"
	authpb "user-service/proto/auth"
)

func setupUserRouter(userHandler *UserHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", int64(1))
		c.Next()
	})
	r.GET("/users/:id", userHandler.GetUserByID)
	r.GET("/users/me", userHandler.GetMe)
	return r
}

func TestGetUserByIDOK(t *testing.T) {
	mockAuth := new(mocks.MockAuthClient)
	userSvc := services.NewUserService(mockAuth)
	friendRepo := new(mocks.MockFriendRepository)
	handler := NewUserHandler(userSvc, friendRepo)
	router := setupUserRouter(handler)

	mockAuth.On("GetUser", mock.Anything, int64(42)).Return(&authpb.GetUserResponse{Id: 42, Username: "alice"}, nil).Once()

	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp services.UserDTO
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	require.Equal(t, int64(42), resp.ID)
	require.Equal(t, "alice", resp.Username)

	mockAuth.AssertExpectations(t)
}

func TestGetUserByIDInvalidID(t *testing.T) {
	userSvc := services.NewUserService(new(mocks.MockAuthClient))
	handler := NewUserHandler(userSvc, new(mocks.MockFriendRepository))
	router := setupUserRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/users/abc", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetMeSuccess(t *testing.T) {
	mockAuth := new(mocks.MockAuthClient)
	mockFriends := new(mocks.MockFriendRepository)
	userSvc := services.NewUserService(mockAuth)
	handler := NewUserHandler(userSvc, mockFriends)
	router := setupUserRouter(handler)

	mockAuth.On("GetUser", mock.Anything, int64(1)).Return(&authpb.GetUserResponse{Id: 1, Username: "me"}, nil).Once()
	mockFriends.On("ListFriends", mock.Anything, int64(1)).Return([]int64{2}, nil).Once()
	mockFriends.On("GetIncomingRequests", mock.Anything, int64(1)).Return([]models.FriendRequest{{ID: 7, FromUserID: 3}}, nil).Once()
	mockAuth.On("GetUser", mock.Anything, int64(2)).Return(&authpb.GetUserResponse{Id: 2, Username: "bob"}, nil).Once()
	mockAuth.On("GetUser", mock.Anything, int64(3)).Return(&authpb.GetUserResponse{Id: 3, Username: "carol"}, nil).Once()

	req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	require.Equal(t, float64(1), resp["id"])
	require.Equal(t, "me", resp["username"])

	friends := resp["friends"].([]any)
	require.Len(t, friends, 1)
	friendEntry := friends[0].(map[string]any)
	require.Equal(t, float64(2), friendEntry["id"])

	incoming := resp["incoming_requests"].([]any)
	require.Len(t, incoming, 1)
	incomingEntry := incoming[0].(map[string]any)
	require.Equal(t, float64(7), incomingEntry["id"])
	require.Equal(t, "carol", incomingEntry["from_username"])

	mockAuth.AssertExpectations(t)
	mockFriends.AssertExpectations(t)
}

func TestGetMeDependencyError(t *testing.T) {
	mockAuth := new(mocks.MockAuthClient)
	mockFriends := new(mocks.MockFriendRepository)
	userSvc := services.NewUserService(mockAuth)
	handler := NewUserHandler(userSvc, mockFriends)
	router := setupUserRouter(handler)

	mockAuth.On("GetUser", mock.Anything, int64(1)).Return((*authpb.GetUserResponse)(nil), assert.AnError).Once()

	req := httptest.NewRequest(http.MethodGet, "/users/me", bytes.NewReader([]byte{}))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadGateway, rec.Code)
	mockAuth.AssertExpectations(t)
}

func TestGetUserByIDDependencyError(t *testing.T) {
	mockAuth := new(mocks.MockAuthClient)
	userSvc := services.NewUserService(mockAuth)
	handler := NewUserHandler(userSvc, new(mocks.MockFriendRepository))
	router := setupUserRouter(handler)

	mockAuth.On("GetUser", mock.Anything, int64(9)).Return((*authpb.GetUserResponse)(nil), assert.AnError).Once()

	req := httptest.NewRequest(http.MethodGet, "/users/9", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadGateway, rec.Code)
	mockAuth.AssertExpectations(t)
}
