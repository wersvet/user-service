package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"user-service/internal/services"
	authpb "user-service/proto/auth"
)

type simpleAuth struct{}

func (s *simpleAuth) GetUser(ctx context.Context, userID int64) (*authpb.GetUserResponse, error) {
	return &authpb.GetUserResponse{Id: userID, Username: "any"}, nil
}

func setupFriendsRouter(handler *FriendHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.Default()
	r.POST("/friends/request", handler.SendRequest)
	return r
}

func TestSendRequest_EmptyBodyReturnsBadRequest(t *testing.T) {
	svc := services.NewUserService(&simpleAuth{})
	handler := NewFriendHandler(nil, svc)
	router := setupFriendsRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/friends/request", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
