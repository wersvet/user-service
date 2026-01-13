package handlers

import (
	"context"
	"database/sql"
	"log"
	nethttp "net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"user-service/internal/repositories"
	"user-service/internal/services"
	"user-service/internal/telemetry"
)

type FriendHandler struct {
	friends         repositories.FriendRepository
	users           *services.UserService
	publisher       telemetry.Publisher
	telemetryConfig telemetry.Config
}

func NewFriendHandler(friends repositories.FriendRepository, users *services.UserService, publisher telemetry.Publisher, telemetryConfig telemetry.Config) *FriendHandler {
	return &FriendHandler{friends: friends, users: users, publisher: publisher, telemetryConfig: telemetryConfig}
}

type sendRequestBody struct {
	ToUserID int64 `json:"to_user_id" binding:"required"`
}

func (h *FriendHandler) SendRequest(c *gin.Context) {
	userID, ok := h.mustUserID(c)
	if !ok {
		h.publishFriendRequestEvent(c, 0, 0, 0, "error", "unauthorized")
		return
	}

	var body sendRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		h.publishFriendRequestEvent(c, userID, 0, 0, "error", "invalid request body")
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	fromUserID := userID
	toUserID := body.ToUserID
	if toUserID == fromUserID {
		h.publishFriendRequestEvent(c, fromUserID, toUserID, 0, "error", "cannot send request to yourself")
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": "cannot send request to yourself"})
		return
	}

	ctx := c.Request.Context()
	if _, err := h.users.GetUserByID(ctx, toUserID); err != nil {
		h.publishFriendRequestEvent(c, fromUserID, toUserID, 0, "error", "target user not found")
		c.JSON(nethttp.StatusBadGateway, gin.H{"error": "target user not found"})
		return
	}

	exists, err := h.friends.HasPendingRequest(ctx, fromUserID, toUserID)
	if err != nil {
		h.publishFriendRequestEvent(c, fromUserID, toUserID, 0, "error", "failed to check requests")
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": "failed to check requests"})
		return
	}
	if exists {
		h.publishFriendRequestEvent(c, fromUserID, toUserID, 0, "error", "pending friend request already exists")
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": "pending friend request already exists"})
		return
	}

	friends, err := h.friends.AreFriends(ctx, fromUserID, toUserID)
	if err != nil {
		h.publishFriendRequestEvent(c, fromUserID, toUserID, 0, "error", "failed to check friendship")
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": "failed to check friendship"})
		return
	}
	if friends {
		h.publishFriendRequestEvent(c, fromUserID, toUserID, 0, "error", "users are already friends")
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": "users are already friends"})
		return
	}

	req, err := h.friends.CreateRequest(ctx, fromUserID, toUserID)
	if err != nil {
		h.publishFriendRequestEvent(c, fromUserID, toUserID, 0, "error", "failed to create request")
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": "failed to create request"})
		return
	}

	h.publishFriendRequestEvent(c, fromUserID, toUserID, req.ID, "success", "")
	c.JSON(nethttp.StatusCreated, req)
}

func (h *FriendHandler) ListIncoming(c *gin.Context) {
	userID, ok := h.mustUserID(c)
	if !ok {
		return
	}

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
	h.handleDecision(c, h.friends.AcceptRequest, "accepted", "friend_request_accepted")
}

func (h *FriendHandler) RejectRequest(c *gin.Context) {
	h.handleDecision(c, h.friends.RejectRequest, "rejected", "friend_request_rejected")
}

func (h *FriendHandler) handleDecision(c *gin.Context, action func(ctx context.Context, requestID, userID int64) error, status string, auditAction string) {
	idStr := c.Param("id")
	reqID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.publishDecisionEvent(c, 0, idStr, status, "error", "invalid request id", auditAction)
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": "invalid request id"})
		return
	}

	userID, ok := h.mustUserID(c)
	if !ok {
		h.publishDecisionEvent(c, 0, strconv.FormatInt(reqID, 10), status, "error", "unauthorized", auditAction)
		return
	}

	ctx := c.Request.Context()
	if err := action(ctx, reqID, userID); err != nil {
		if err == sql.ErrNoRows {
			h.publishDecisionEvent(c, userID, strconv.FormatInt(reqID, 10), status, "error", "request not found", auditAction)
			c.JSON(nethttp.StatusNotFound, gin.H{"error": "request not found"})
			return
		}
		h.publishDecisionEvent(c, userID, strconv.FormatInt(reqID, 10), status, "error", "failed to update request", auditAction)
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": "failed to update request"})
		return
	}

	h.publishDecisionEvent(c, userID, strconv.FormatInt(reqID, 10), status, "success", "", auditAction)
	c.JSON(nethttp.StatusOK, gin.H{"status": status})
}

func (h *FriendHandler) ListFriends(c *gin.Context) {
	userID, ok := h.mustUserID(c)
	if !ok {
		return
	}

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

func (h *FriendHandler) mustUserID(c *gin.Context) (int64, bool) {
	userIDVal, ok := c.Get("userID")
	if !ok {
		c.JSON(nethttp.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return 0, false
	}
	userID, ok := userIDVal.(int64)
	if !ok {
		c.JSON(nethttp.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return 0, false
	}
	return userID, true
}

func (h *FriendHandler) publishFriendRequestEvent(c *gin.Context, fromUserID, toUserID, requestID int64, result, errMsg string) {
	payload := telemetry.FriendRequestSentPayload{
		Action:    "friend_request_sent",
		RequestID: formatID(requestID),
		FromUser:  formatID(fromUserID),
		ToUser:    formatID(toUserID),
		Status:    "pending",
		Result:    result,
		Error:     errMsg,
	}
	h.publishAuditEvent(c, payload)
}

func (h *FriendHandler) publishDecisionEvent(c *gin.Context, byUserID int64, requestID, status, result, errMsg, action string) {
	payload := telemetry.FriendRequestDecisionPayload{
		Action:          action,
		FriendRequestID: requestID,
		ByUser:          formatID(byUserID),
		Status:          status,
		Result:          result,
		Error:           errMsg,
	}
	h.publishAuditEvent(c, payload)
}

func (h *FriendHandler) publishAuditEvent(c *gin.Context, payload any) {
	if h.publisher == nil {
		return
	}
	requestID := h.requestID(c)
	userID := h.userIDString(c)
	event := telemetry.NewEnvelope(h.telemetryConfig, requestID, userID, payload)
	if err := h.publisher.Publish(c.Request.Context(), telemetry.AuditFriendsKey, event); err != nil {
		log.Printf("warning: failed to publish audit event: %v", err)
	}
}

func (h *FriendHandler) requestID(c *gin.Context) string {
	requestID := c.GetHeader("X-Request-ID")
	if requestID == "" {
		requestID = uuid.NewString()
		c.Request.Header.Set("X-Request-ID", requestID)
	}
	return requestID
}

func (h *FriendHandler) userIDString(c *gin.Context) string {
	userIDVal, ok := c.Get("userID")
	if !ok {
		return ""
	}
	switch value := userIDVal.(type) {
	case int64:
		return formatID(value)
	case int:
		return formatID(int64(value))
	case string:
		return value
	default:
		return ""
	}
}

func formatID(value int64) string {
	if value == 0 {
		return ""
	}
	return strconv.FormatInt(value, 10)
}
