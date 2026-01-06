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

type fakeAuth struct{}

func (f *fakeAuth) GetUser(ctx context.Context, userID int64) (*authpb.GetUserResponse, error) {
	return &authpb.GetUserResponse{Id: userID, Username: "demo"}, nil
}

func setupUserRouter(handler *UserHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.Default()
	r.GET("/users/:id", handler.GetUserByID)
	return r
}

func TestGetUserByID_ReturnsOK(t *testing.T) {
	svc := services.NewUserService(&fakeAuth{})
	h := NewUserHandler(svc, nil)
	router := setupUserRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/users/1", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
