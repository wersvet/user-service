package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/require"

	"user-service/internal/metrics"
	"user-service/internal/mocks"
	"user-service/internal/services"
)

func setupFriendsMetricsRouter(handler *FriendHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/friends/request", handler.SendRequest)
	r.POST("/friends/requests/:id/accept", handler.AcceptRequest)
	r.POST("/friends/requests/:id/reject", handler.RejectRequest)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	return r
}

func fetchMetrics(t *testing.T, router *gin.Engine) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	return rec.Body.String()
}

func metricValue(metricsBody, name, status string) (float64, bool) {
	target := name + `{status="` + status + `"}`
	for _, line := range strings.Split(metricsBody, "\n") {
		if strings.HasPrefix(line, target+" ") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return 0, false
			}
			value, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				return 0, false
			}
			return value, true
		}
	}
	return 0, false
}

func assertMetricIncrement(t *testing.T, router *gin.Engine, name, status string, call func()) {
	t.Helper()
	before, _ := metricValue(fetchMetrics(t, router), name, status)
	call()
	after, found := metricValue(fetchMetrics(t, router), name, status)
	require.True(t, found)
	require.Greater(t, after, before)
}

func TestFriendRequestMetricsFailed(t *testing.T) {
	metrics.RegisterFriendMetrics()
	handler := NewFriendHandler(new(mocks.MockFriendRepository), services.NewUserService(new(mocks.MockAuthClient)), nil)
	router := setupFriendsMetricsRouter(handler)

	assertMetricIncrement(t, router, "friend_requests_total", "failed", func() {
		req := httptest.NewRequest(http.MethodPost, "/friends/request", bytes.NewBufferString(`{"to_user_id":"bad"}`))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestFriendAcceptMetricsFailed(t *testing.T) {
	metrics.RegisterFriendMetrics()
	handler := NewFriendHandler(new(mocks.MockFriendRepository), services.NewUserService(new(mocks.MockAuthClient)), nil)
	router := setupFriendsMetricsRouter(handler)

	assertMetricIncrement(t, router, "friend_accepts_total", "failed", func() {
		req := httptest.NewRequest(http.MethodPost, "/friends/requests/abc/accept", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestFriendRejectMetricsFailed(t *testing.T) {
	metrics.RegisterFriendMetrics()
	handler := NewFriendHandler(new(mocks.MockFriendRepository), services.NewUserService(new(mocks.MockAuthClient)), nil)
	router := setupFriendsMetricsRouter(handler)

	assertMetricIncrement(t, router, "friend_rejects_total", "failed", func() {
		req := httptest.NewRequest(http.MethodPost, "/friends/requests/abc/reject", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}
