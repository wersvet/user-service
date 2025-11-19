package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"user-service/internal/db"
	"user-service/internal/handlers"
	"user-service/internal/middleware"
	"user-service/internal/repositories"
	"user-service/internal/services"
)

func main() {
	dsn := os.Getenv("DB_DSN")
	jwtSecret := os.Getenv("JWT_SECRET")
	authServiceURL := os.Getenv("AUTH_SERVICE_URL")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	if dsn == "" || jwtSecret == "" || authServiceURL == "" {
		log.Fatal("DB_DSN, JWT_SECRET, and AUTH_SERVICE_URL environment variables must be set")
	}

	database, err := db.Connect(dsn)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	friendRepo := repositories.NewFriendRepository(database)
	userService := services.NewUserService(authServiceURL)

	userHandler := handlers.NewUserHandler(userService, friendRepo)
	friendHandler := handlers.NewFriendHandler(friendRepo, userService)

	r := gin.Default()
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(200)
			return
		}

		c.Next()
	})

	r.GET("/users/:id", userHandler.GetUserByID)

	auth := r.Group("", middleware.JWTAuth(jwtSecret))
	auth.GET("/users/me", userHandler.GetMe)
	auth.POST("/friends/request", friendHandler.SendRequest)
	auth.GET("/friends/requests/incoming", friendHandler.ListIncoming)
	auth.POST("/friends/requests/:id/accept", friendHandler.AcceptRequest)
	auth.POST("/friends/requests/:id/reject", friendHandler.RejectRequest)
	auth.GET("/friends", friendHandler.ListFriends)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
