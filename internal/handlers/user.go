package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	nethttp "net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"user-service/internal/repositories"
	"user-service/internal/services"
)

type UserHandler struct {
	userService *services.UserService
	friends     repositories.FriendRepository
	users       repositories.UserRepository
	avatarDir   string
}

func NewUserHandler(userService *services.UserService, friends repositories.FriendRepository, users repositories.UserRepository, avatarDir string) *UserHandler {
	return &UserHandler{
		userService: userService,
		friends:     friends,
		users:       users,
		avatarDir:   avatarDir,
	}
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
		"avatar_url":        user.AvatarURL,
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

func (h *UserHandler) UploadAvatar(c *gin.Context) {
	userIDVal, _ := c.Get("userID")
	userID := userIDVal.(int64)

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": "missing file"})
		return
	}

	ext := filepath.Ext(file.Filename)
	filename := fmt.Sprintf("%s%s", uuid.NewString(), ext)
	userDir := filepath.Join(h.avatarDir, strconv.FormatInt(userID, 10))
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": "failed to create upload directory"})
		return
	}

	dstPath := filepath.Join(userDir, filename)
	if err := c.SaveUploadedFile(file, dstPath); err != nil {
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": "failed to save file"})
		return
	}

	avatarURL := fmt.Sprintf("/uploads/avatars/%d/%s", userID, filename)
	if err := h.users.SetAvatarURL(c.Request.Context(), userID, avatarURL); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(nethttp.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": "failed to update avatar"})
		return
	}

	c.JSON(nethttp.StatusOK, gin.H{"avatar_url": avatarURL})
}

func (h *UserHandler) DeleteAvatar(c *gin.Context) {
	userIDVal, _ := c.Get("userID")
	userID := userIDVal.(int64)

	avatarURL, err := h.users.GetAvatarURL(c.Request.Context(), userID)
	if err != nil {
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": "failed to fetch avatar"})
		return
	}

	if avatarURL != "" {
		const prefix = "/uploads/avatars/"
		if strings.HasPrefix(avatarURL, prefix) {
			relativePath := strings.TrimPrefix(avatarURL, prefix)
			_ = os.Remove(filepath.Join(h.avatarDir, relativePath))
		}
	}

	if err := h.users.ClearAvatarURL(c.Request.Context(), userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(nethttp.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": "failed to clear avatar"})
		return
	}

	c.Status(nethttp.StatusNoContent)
}
