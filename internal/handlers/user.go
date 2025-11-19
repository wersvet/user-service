package handlers

import (
	nethttp "net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"user-service/internal/repositories"
	"user-service/internal/services"
)

type UserHandler struct {
	userService *services.UserService
	friends     repositories.FriendRepository
}

func NewUserHandler(userService *services.UserService, friends repositories.FriendRepository) *UserHandler {
	return &UserHandler{userService: userService, friends: friends}
}

func (h *UserHandler) GetMe(c *gin.Context) {
	userIDVal, _ := c.Get("userID")
	userID := userIDVal.(int64)

	ctx := c.Request.Context()
	user, err := h.userService.GetUserByID(ctx, userID)
	if err != nil {
		c.JSON(nethttp.StatusBadGateway, gin.H{"error": "failed to fetch user"})
		return
	}

	friends, err := h.friends.ListFriends(ctx, userID)
	if err != nil {
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": "failed to load friends"})
		return
	}

	incoming, err := h.friends.GetIncomingRequests(ctx, userID)
	if err != nil {
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": "failed to load friend requests"})
		return
	}

	friendUsers := make([]*services.UserDTO, 0, len(friends))
	for _, fid := range friends {
		fUser, err := h.userService.GetUserByID(ctx, fid)
		if err != nil {
			c.JSON(nethttp.StatusBadGateway, gin.H{"error": "failed to fetch friend info"})
			return
		}
		friendUsers = append(friendUsers, fUser)
	}

	incomingWithUsers := make([]gin.H, 0, len(incoming))
	for _, req := range incoming {
		sender, err := h.userService.GetUserByID(ctx, req.FromUserID)
		if err != nil {
			c.JSON(nethttp.StatusBadGateway, gin.H{"error": "failed to fetch requester info"})
			return
		}
		incomingWithUsers = append(incomingWithUsers, gin.H{
			"id":            req.ID,
			"from_user_id":  req.FromUserID,
			"from_username": sender.Username,
			"status":        req.Status,
			"created_at":    req.CreatedAt,
		})
	}

	c.JSON(nethttp.StatusOK, gin.H{
		"id":                user.ID,
		"username":          user.Username,
		"friends":           friendUsers,
		"incoming_requests": incomingWithUsers,
	})
}

func (h *UserHandler) GetUserByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	user, err := h.userService.GetUserByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(nethttp.StatusBadGateway, gin.H{"error": "user not found"})
		return
	}

	c.JSON(nethttp.StatusOK, user)
}
