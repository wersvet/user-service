package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func requestIDFromHeader(c *gin.Context) string {
	requestID := c.GetHeader("X-Request-ID")
	if requestID == "" {
		requestID = uuid.NewString()
	}
	return requestID
}

func userIDFromContext(c *gin.Context) *int64 {
	if userIDVal, ok := c.Get("userID"); ok {
		if userID, ok := userIDVal.(int64); ok {
			return &userID
		}
	}

	if header := c.GetHeader("X-User-ID"); header != "" {
		if parsed, err := strconv.ParseInt(header, 10, 64); err == nil {
			return &parsed
		}
	}

	return nil
}
