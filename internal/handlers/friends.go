package handlers

import (
	"context"
	"database/sql"
	nethttp "net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"user-service/internal/repositories"
	"user-service/internal/services"
)

type FriendHandler struct {
	friends repositories.FriendRepository
	users   *services.UserService
}

func NewFriendHandler(friends repositories.FriendRepository, users *services.UserService) *FriendHandler {
	return &FriendHandler{friends: friends, users: users}
}

type sendRequestBody struct {
	ToUserID int64 `json:"to_user_id" binding:"required"`
}

func (h *FriendHandler) SendRequest(c *gin.Context) {
	var body sendRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	userIDVal, _ := c.Get("userID")
	fromUserID := userIDVal.(int64)

	toUserID := body.ToUserID
	if toUserID == fromUserID {
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": "cannot send request to yourself"})
		return
	}

	ctx := c.Request.Context()
	if _, err := h.users.GetUserByID(ctx, toUserID); err != nil {
		c.JSON(nethttp.StatusBadGateway, gin.H{"error": "target user not found"})
		return
	}

	exists, err := h.friends.HasPendingRequest(ctx, fromUserID, toUserID)
	if err != nil {
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": "failed to check requests"})
		return
	}
	if exists {
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": "pending friend request already exists"})
		return
	}

	friends, err := h.friends.AreFriends(ctx, fromUserID, toUserID)
	if err != nil {
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": "failed to check friendship"})
		return
	}
	if friends {
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": "users are already friends"})
		return
	}

	req, err := h.friends.CreateRequest(ctx, fromUserID, toUserID)
	if err != nil {
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": "failed to create request"})
		return
	}

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
	h.handleDecision(c, h.friends.AcceptRequest, "accepted")
}

func (h *FriendHandler) RejectRequest(c *gin.Context) {
	h.handleDecision(c, h.friends.RejectRequest, "rejected")
}

func (h *FriendHandler) handleDecision(c *gin.Context, action func(ctx context.Context, requestID, userID int64) error, status string) {
	idStr := c.Param("id")
	reqID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": "invalid request id"})
		return
	}

	userIDVal, _ := c.Get("userID")
	userID := userIDVal.(int64)

	ctx := c.Request.Context()
	if err := action(ctx, reqID, userID); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(nethttp.StatusNotFound, gin.H{"error": "request not found"})
			return
		}
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": "failed to update request"})
		return
	}

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
			c.JSON(nethttp.StatusBadGateway, gin.H{"error": "failed to fetch friend info"})
			return
		}
		resp = append(resp, friendUser)
	}

	c.JSON(nethttp.StatusOK, resp)
}
