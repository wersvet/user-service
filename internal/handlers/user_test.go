package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
	r.POST("/users/me/avatar", userHandler.UploadAvatar)
	r.DELETE("/users/me/avatar", userHandler.DeleteAvatar)
	return r
}

func TestGetUserByIDOK(t *testing.T) {
	mockAuth := new(mocks.MockAuthClient)
	mockUsers := new(mocks.MockUserRepository)
	userSvc := services.NewUserService(mockAuth, mockUsers)
	friendRepo := new(mocks.MockFriendRepository)
	handler := NewUserHandler(userSvc, friendRepo, mockUsers, t.TempDir())
	router := setupUserRouter(handler)

	mockAuth.On("GetUser", mock.Anything, int64(42)).Return(&authpb.GetUserResponse{Id: 42, Username: "alice"}, nil).Once()
	mockUsers.On("GetAvatarURL", mock.Anything, int64(42)).Return("/uploads/avatars/42/avatar.png", nil).Once()

	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp services.UserDTO
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	require.Equal(t, int64(42), resp.ID)
	require.Equal(t, "alice", resp.Username)
	require.Equal(t, "/uploads/avatars/42/avatar.png", resp.AvatarURL)

	mockAuth.AssertExpectations(t)
	mockUsers.AssertExpectations(t)
}

func TestGetUserByIDInvalidID(t *testing.T) {
	mockUsers := new(mocks.MockUserRepository)
	userSvc := services.NewUserService(new(mocks.MockAuthClient), mockUsers)
	handler := NewUserHandler(userSvc, new(mocks.MockFriendRepository), mockUsers, t.TempDir())
	router := setupUserRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/users/abc", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetMeSuccess(t *testing.T) {
	mockAuth := new(mocks.MockAuthClient)
	mockFriends := new(mocks.MockFriendRepository)
	mockUsers := new(mocks.MockUserRepository)
	mockUsers.On("GetAvatarURL", mock.Anything, mock.Anything).Return("", nil)
	userSvc := services.NewUserService(mockAuth, mockUsers)
	handler := NewUserHandler(userSvc, mockFriends, mockUsers, t.TempDir())
	router := setupUserRouter(handler)

	mockAuth.On("GetUser", mock.Anything, int64(1)).Return(&authpb.GetUserResponse{Id: 1, Username: "me"}, nil).Once()
	mockFriends.On("ListFriends", mock.Anything, int64(1)).Return([]int64{2}, nil).Once()
	mockFriends.On("GetIncomingRequests", mock.Anything, int64(1)).Return([]models.FriendRequest{{ID: 7, FromUserID: 3}}, nil).Once()
	mockAuth.On("GetUser", mock.Anything, int64(2)).Return(&authpb.GetUserResponse{Id: 2, Username: "wersvet"}, nil).Once()
	mockAuth.On("GetUser", mock.Anything, int64(3)).Return(&authpb.GetUserResponse{Id: 3, Username: "alimzhan"}, nil).Once()

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
	require.Equal(t, "alimzhan", incomingEntry["from_username"])

	mockAuth.AssertExpectations(t)
	mockFriends.AssertExpectations(t)
	mockUsers.AssertExpectations(t)
}

func TestGetMeDependencyError(t *testing.T) {
	mockAuth := new(mocks.MockAuthClient)
	mockFriends := new(mocks.MockFriendRepository)
	mockUsers := new(mocks.MockUserRepository)
	userSvc := services.NewUserService(mockAuth, mockUsers)
	handler := NewUserHandler(userSvc, mockFriends, mockUsers, t.TempDir())
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
	mockUsers := new(mocks.MockUserRepository)
	userSvc := services.NewUserService(mockAuth, mockUsers)
	handler := NewUserHandler(userSvc, new(mocks.MockFriendRepository), mockUsers, t.TempDir())
	router := setupUserRouter(handler)

	mockAuth.On("GetUser", mock.Anything, int64(9)).Return((*authpb.GetUserResponse)(nil), assert.AnError).Once()

	req := httptest.NewRequest(http.MethodGet, "/users/9", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadGateway, rec.Code)
	mockAuth.AssertExpectations(t)
}

func TestUploadAvatar(t *testing.T) {
	mockAuth := new(mocks.MockAuthClient)
	mockUsers := new(mocks.MockUserRepository)
	userSvc := services.NewUserService(mockAuth, mockUsers)
	handler := NewUserHandler(userSvc, new(mocks.MockFriendRepository), mockUsers, t.TempDir())
	router := setupUserRouter(handler)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "avatar.png")
	require.NoError(t, err)
	_, err = io.Copy(part, strings.NewReader("avatar-content"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	mockUsers.On("SetAvatarURL", mock.Anything, int64(1), mock.AnythingOfType("string")).Return(nil).Once()

	req := httptest.NewRequest(http.MethodPost, "/users/me/avatar", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	avatarURL := resp["avatar_url"]
	require.NotEmpty(t, avatarURL)

	relativePath := strings.TrimPrefix(avatarURL, "/uploads/avatars/")
	_, err = os.Stat(filepath.Join(handler.avatarDir, relativePath))
	require.NoError(t, err)

	mockUsers.AssertExpectations(t)
}

func TestDeleteAvatar(t *testing.T) {
	mockAuth := new(mocks.MockAuthClient)
	mockUsers := new(mocks.MockUserRepository)
	avatarDir := t.TempDir()
	userSvc := services.NewUserService(mockAuth, mockUsers)
	handler := NewUserHandler(userSvc, new(mocks.MockFriendRepository), mockUsers, avatarDir)
	router := setupUserRouter(handler)

	avatarURL := "/uploads/avatars/1/to-delete.png"
	filePath := filepath.Join(avatarDir, "1", "to-delete.png")
	require.NoError(t, os.MkdirAll(filepath.Dir(filePath), 0o755))
	require.NoError(t, os.WriteFile(filePath, []byte("content"), 0o644))

	mockUsers.On("GetAvatarURL", mock.Anything, int64(1)).Return(avatarURL, nil).Once()
	mockUsers.On("ClearAvatarURL", mock.Anything, int64(1)).Return(nil).Once()

	req := httptest.NewRequest(http.MethodDelete, "/users/me/avatar", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	mockUsers.AssertExpectations(t)
}
