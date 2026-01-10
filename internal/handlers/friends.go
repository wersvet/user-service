package handlers

import (
	"context"
	"database/sql"
	nethttp "net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"

	"user-service/internal/observability"
	"user-service/internal/rabbitmq"
	"user-service/internal/repositories"
	"user-service/internal/services"
)

type FriendHandler struct {
	friends   repositories.FriendRepository
	users     *services.UserService
	publisher rabbitmq.Publisher
}

func NewFriendHandler(friends repositories.FriendRepository, users *services.UserService, publisher rabbitmq.Publisher) *FriendHandler {
	return &FriendHandler{friends: friends, users: users, publisher: publisher}
}

type sendRequestBody struct {
	ToUserID int64 `json:"to_user_id" binding:"required"`
}

func (h *FriendHandler) SendRequest(c *gin.Context) {
	var body sendRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": "invalid request body"})
		h.publishAuditEvent(c, "friends.request", body.ToUserID, 0, false, "invalid request body")
		return
	}

	userIDVal, _ := c.Get("userID")
	fromUserID := userIDVal.(int64)

	toUserID := body.ToUserID
	if toUserID == fromUserID {
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": "cannot send request to yourself"})
		h.publishAuditEvent(c, "friends.request", toUserID, 0, false, "cannot send request to yourself")
		return
	}

	ctx := c.Request.Context()
	if _, err := h.users.GetUserByID(ctx, toUserID); err != nil {
		c.JSON(nethttp.StatusBadGateway, gin.H{"error": "target user not found"})
		h.publishAuditEvent(c, "friends.request", toUserID, 0, false, "target user not found")
		return
	}

	exists, err := h.friends.HasPendingRequest(ctx, fromUserID, toUserID)
	if err != nil {
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": "failed to check requests"})
		h.publishAuditEvent(c, "friends.request", toUserID, 0, false, "failed to check requests")
		return
	}
	if exists {
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": "pending friend request already exists"})
		h.publishAuditEvent(c, "friends.request", toUserID, 0, false, "pending friend request already exists")
		return
	}

	friends, err := h.friends.AreFriends(ctx, fromUserID, toUserID)
	if err != nil {
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": "failed to check friendship"})
		h.publishAuditEvent(c, "friends.request", toUserID, 0, false, "failed to check friendship")
		return
	}
	if friends {
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": "users are already friends"})
		h.publishAuditEvent(c, "friends.request", toUserID, 0, false, "users are already friends")
		return
	}

	req, err := h.friends.CreateRequest(ctx, fromUserID, toUserID)
	if err != nil {
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": "failed to create request"})
		h.publishAuditEvent(c, "friends.request", toUserID, 0, false, "failed to create request")
		return
	}

	c.JSON(nethttp.StatusCreated, req)
	h.publishAuditEvent(c, "friends.request", toUserID, req.ID, true, "")
}

func (h *FriendHandler) ListIncoming(c *gin.Context) {
	userIDVal, _ := c.Get("userID")
	userID := userIDVal.(int64)

	requests, err := h.friends.GetIncomingRequests(c.Request.Context(), userID)
	if err != nil {
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": "failed to load requests"})
		return
	}

	resp := make([]gin.H, 0, len(requests))
	for _, req := range requests {
		sender, err := h.users.GetUserByID(c.Request.Context(), req.FromUserID)
		if err != nil {
			c.JSON(nethttp.StatusBadGateway, gin.H{"error": "failed to fetch requester info"})
			return
		}
		resp = append(resp, gin.H{
			"id":            req.ID,
			"from_user_id":  req.FromUserID,
			"from_username": sender.Username,
			"status":        req.Status,
			"created_at":    req.CreatedAt,
		})
	}

	c.JSON(nethttp.StatusOK, resp)
}

func (h *FriendHandler) AcceptRequest(c *gin.Context) {
	h.handleDecision(c, h.friends.AcceptRequest, "friends.accept", "accepted")
}

func (h *FriendHandler) RejectRequest(c *gin.Context) {
	h.handleDecision(c, h.friends.RejectRequest, "friends.reject", "rejected")
}

func (h *FriendHandler) handleDecision(c *gin.Context, action func(ctx context.Context, requestID, userID int64) error, actionName, status string) {
	idStr := c.Param("id")
	reqID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": "invalid request id"})
		h.publishAuditEvent(c, actionName, 0, 0, false, "invalid request id")
		return
	}

	userIDVal, _ := c.Get("userID")
	userID := userIDVal.(int64)

	ctx := c.Request.Context()
	if err := action(ctx, reqID, userID); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(nethttp.StatusNotFound, gin.H{"error": "request not found"})
			h.publishAuditEvent(c, actionName, 0, reqID, false, "request not found")
			return
		}
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": "failed to update request"})
		h.publishAuditEvent(c, actionName, 0, reqID, false, "failed to update request")
		return
	}

	c.JSON(nethttp.StatusOK, gin.H{"status": status})
	h.publishAuditEvent(c, actionName, 0, reqID, true, "")
}

func (h *FriendHandler) publishAuditEvent(c *gin.Context, action string, toUserID, requestID int64, ok bool, errMsg string) {
	if h.publisher == nil {
		return
	}

	userIDVal, _ := c.Get("userID")
	userID, _ := userIDVal.(int64)

	deviceID := c.GetHeader("X-Device-Id")
	requestIDHeader := c.GetHeader("X-Request-Id")
	traceParent := c.GetHeader("traceparent")
	traceID := trace.SpanFromContext(c.Request.Context()).SpanContext().TraceID().String()
	if traceID == "" {
		traceID = "unknown"
	}

	target := map[string]any{}
	if toUserID != 0 {
		target["to_user_id"] = toUserID
	}
	if requestID != 0 {
		target["request_id"] = requestID
	}

	envelope := map[string]any{
		"event_type": "audit_events",
		"event_name": action,
		"payload": map[string]any{
			"actor": map[string]any{
				"user_id":   userID,
				"device_id": deviceID,
			},
			"action": action,
			"target": target,
			"http": map[string]any{
				"method": c.Request.Method,
				"path":   c.Request.URL.Path,
				"status": c.Writer.Status(),
			},
			"result": map[string]any{
				"ok":    ok,
				"error": nil,
			},
			"request_id":  requestIDHeader,
			"trace_id":    traceID,
			"traceparent": traceParent,
			"ip":          c.ClientIP(),
		},
	}
	if !ok {
		envelope["payload"].(map[string]any)["result"].(map[string]any)["error"] = errMsg
	}

	if err := h.publisher.PublishEvent("audit_events.friends", envelope); err != nil {
		observability.IncAMQPPublishError()
		return
	}
	observability.IncAuditEventPublished(action)
}

func (h *FriendHandler) ListFriends(c *gin.Context) {
	userIDVal, _ := c.Get("userID")
	userID := userIDVal.(int64)

	friends, err := h.friends.ListFriends(c.Request.Context(), userID)
	if err != nil {
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": "failed to fetch friends"})
		return
	}

	resp := make([]*services.UserDTO, 0, len(friends))
	for _, fid := range friends {
		friendUser, err := h.users.GetUserByID(c.Request.Context(), fid)
		if err != nil {
			c.JSON(nethttp.StatusBadGateway, gin.H{"error": "failed to fetch friend info"})
			return
		}
		resp = append(resp, friendUser)
	}

	c.JSON(nethttp.StatusOK, resp)
}
