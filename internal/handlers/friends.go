package handlers

import (
	"context"
	"database/sql"
	nethttp "net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"user-service/internal/metrics"
	"user-service/internal/repositories"
	"user-service/internal/services"
	"user-service/internal/telemetry"
)

type FriendHandler struct {
	friends repositories.FriendRepository
	users   *services.UserService
	audit   *telemetry.AuditEmitter
}

func NewFriendHandler(friends repositories.FriendRepository, users *services.UserService, audit *telemetry.AuditEmitter) *FriendHandler {
	return &FriendHandler{friends: friends, users: users, audit: audit}
}

type sendRequestBody struct {
	ToUserID int64 `json:"to_user_id" binding:"required"`
}

func (h *FriendHandler) SendRequest(c *gin.Context) {
	requestID := requestIDFromHeader(c)
	userID := userIDFromContext(c)
	var body sendRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		h.emitAudit(c.Request.Context(), "ERROR", "invalid request payload", requestID, userID)
		metrics.IncFriendRequest(metrics.StatusFailed)
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if userID == nil {
		h.emitAudit(c.Request.Context(), "ERROR", "internal error", requestID, nil)
		metrics.IncFriendRequest(metrics.StatusFailed)
		c.JSON(nethttp.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	fromUserID := *userID

	toUserID := body.ToUserID
	if toUserID == fromUserID {
		metrics.IncFriendRequest(metrics.StatusFailed)
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": "cannot send request to yourself"})
		return
	}

	ctx := c.Request.Context()
	if _, err := h.users.GetUserByID(ctx, toUserID); err != nil {
		if err == sql.ErrNoRows {
			h.emitAudit(ctx, "ERROR", "target user not found", requestID, userID)
			metrics.IncFriendRequest(metrics.StatusFailed)
			c.JSON(nethttp.StatusNotFound, gin.H{"error": "target user not found"})
			return
		}
		h.emitAudit(ctx, "ERROR", "target user not found", requestID, userID)
		metrics.IncFriendRequest(metrics.StatusFailed)
		c.JSON(nethttp.StatusNotFound, gin.H{"error": "target user not found"})
		return
	}

	exists, err := h.friends.HasPendingRequest(ctx, fromUserID, toUserID)
	if err != nil {
		h.emitAudit(ctx, "ERROR", "internal error", requestID, userID)
		metrics.IncFriendRequest(metrics.StatusFailed)
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": "failed to check requests"})
		return
	}
	if exists {
		h.emitAudit(ctx, "ERROR", "pending friend request already exists", requestID, userID)
		metrics.IncFriendRequest(metrics.StatusFailed)
		c.JSON(nethttp.StatusConflict, gin.H{"error": "pending friend request already exists"})
		return
	}

	friends, err := h.friends.AreFriends(ctx, fromUserID, toUserID)
	if err != nil {
		h.emitAudit(ctx, "ERROR", "internal error", requestID, userID)
		metrics.IncFriendRequest(metrics.StatusFailed)
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": "failed to check friendship"})
		return
	}
	if friends {
		h.emitAudit(ctx, "ERROR", "users are already friends", requestID, userID)
		metrics.IncFriendRequest(metrics.StatusFailed)
		c.JSON(nethttp.StatusConflict, gin.H{"error": "users are already friends"})
		return
	}

	req, err := h.friends.CreateRequest(ctx, fromUserID, toUserID)
	if err != nil {
		h.emitAudit(ctx, "ERROR", "internal error", requestID, userID)
		metrics.IncFriendRequest(metrics.StatusFailed)
		return
	}

	h.emitAudit(ctx, "INFO", "Friend request sent to '"+strconv.FormatInt(toUserID, 10)+"'", requestID, userID)
	metrics.IncFriendRequest(metrics.StatusSuccess)
	c.JSON(nethttp.StatusCreated, req)
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
			if err == sql.ErrNoRows {
				c.JSON(nethttp.StatusNotFound, gin.H{"error": "requester not found"})
				return
			}
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
	h.handleDecision(c, h.friends.AcceptRequest, "accepted", "accept", metrics.IncFriendAccept)
}

func (h *FriendHandler) RejectRequest(c *gin.Context) {
	h.handleDecision(c, h.friends.RejectRequest, "rejected", "reject", metrics.IncFriendReject)
}

func (h *FriendHandler) handleDecision(c *gin.Context, action func(ctx context.Context, requestID, userID int64) error, status, verb string, inc func(string)) {
	idStr := c.Param("id")
	reqID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		inc(metrics.StatusFailed)
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": "invalid request id"})
		return
	}

	requestID := requestIDFromHeader(c)
	userID := userIDFromContext(c)
	if userID == nil {
		h.emitAudit(c.Request.Context(), "ERROR", "internal error", requestID, nil)
		inc(metrics.StatusFailed)
		c.JSON(nethttp.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	userIDVal := *userID

	ctx := c.Request.Context()
	if err := action(ctx, reqID, userIDVal); err != nil {
		if err == sql.ErrNoRows {
			h.emitAudit(ctx, "ERROR", "friend request not found", requestID, userID)
			inc(metrics.StatusFailed)
			c.JSON(nethttp.StatusNotFound, gin.H{"error": "request not found"})
			return
		}
		if err == repositories.ErrRequestForbidden {
			h.emitAudit(ctx, "ERROR", "not allowed to "+verb+" this request", requestID, userID)
			inc(metrics.StatusFailed)
			c.JSON(nethttp.StatusForbidden, gin.H{"error": "not allowed to " + verb + " this request"})
			return
		}
		h.emitAudit(ctx, "ERROR", "internal error", requestID, userID)
		inc(metrics.StatusFailed)
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": "failed to update request"})
		return
	}

	h.emitAudit(ctx, "INFO", "Friend request "+status, requestID, userID)
	inc(metrics.StatusSuccess)
	c.JSON(nethttp.StatusOK, gin.H{"status": status})
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
			if err == sql.ErrNoRows {
				c.JSON(nethttp.StatusNotFound, gin.H{"error": "friend not found"})
				return
			}
			c.JSON(nethttp.StatusBadGateway, gin.H{"error": "failed to fetch friend info"})
			return
		}
		resp = append(resp, friendUser)
	}

	c.JSON(nethttp.StatusOK, resp)
}

func (h *FriendHandler) DeleteFriend(c *gin.Context) {
	friendIDStr := c.Param("friend_id")
	friendID, err := strconv.ParseInt(friendIDStr, 10, 64)
	if err != nil {
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": "invalid friend id"})
		return
	}

	userID := userIDFromContext(c)
	if userID == nil {
		c.JSON(nethttp.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	ctx := c.Request.Context()
	if _, err := h.users.GetUserByID(ctx, friendID); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(nethttp.StatusNotFound, gin.H{"error": "friend not found"})
			return
		}
		c.JSON(nethttp.StatusNotFound, gin.H{"error": "friend not found"})
		return
	}

	areFriends, err := h.friends.AreFriends(ctx, *userID, friendID)
	if err != nil {
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": "failed to check friendship"})
		return
	}
	if !areFriends {
		c.JSON(nethttp.StatusNotFound, gin.H{"error": "friendship not found"})
		return
	}

	if err := h.friends.DeleteFriendship(ctx, *userID, friendID); err != nil {
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": "failed to delete friendship"})
		return
	}

	c.Status(nethttp.StatusNoContent)
}

func (h *FriendHandler) emitAudit(ctx context.Context, level, text, requestID string, userID *int64) {
	if h.audit == nil {
		return
	}
	h.audit.EmitAudit(ctx, level, text, requestID, userID)
}
