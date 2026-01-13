package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"user-service/internal/rabbitmq"
	"user-service/internal/telemetry"

	"github.com/gin-gonic/gin"

	"user-service/internal/db"
	grpcsvc "user-service/internal/grpc"
	"user-service/internal/handlers"
	"user-service/internal/middleware"
	"user-service/internal/repositories"
	"user-service/internal/services"
)

func main() {
	dsn := os.Getenv("DB_DSN")
	jwtSecret := os.Getenv("JWT_SECRET")
	authGRPCAddr := os.Getenv("AUTH_GRPC_ADDR")
	amqpURL := os.Getenv("AMQP_URL")
	logsExchange := os.Getenv("LOGS_EXCHANGE")
	serviceName := os.Getenv("SERVICE_NAME")
	environment := os.Getenv("ENVIRONMENT")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	if dsn == "" || jwtSecret == "" || authGRPCAddr == "" {
		log.Fatal("DB_DSN, JWT_SECRET, and AUTH_GRPC_ADDR environment variables must be set")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	database, err := db.Connect(dsn)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	authClient, err := grpcsvc.NewAuthClient(authGRPCAddr)
	if err != nil {
		log.Fatalf("failed to create auth gRPC client: %v", err)
	}
	defer authClient.Close()

	publisher := rabbitmq.NewNoopPublisher()
	if amqpURL == "" {
		log.Printf("warning: AMQP_URL not set; event publishing disabled")
	} else {
		pub, err := rabbitmq.NewPublisher(amqpURL)
		if err != nil {
			log.Printf("warning: failed to initialize RabbitMQ publisher: %v", err)
		} else {
			publisher = pub
		}
	}
	defer publisher.Close()

	telemetryPublisher := telemetry.NewNoopPublisher()
	telemetryConfig := telemetry.Config{Environment: environment, ServiceName: serviceName}
	if amqpURL == "" {
		log.Printf("warning: AMQP_URL not set; audit events disabled")
	} else if logsExchange == "" || serviceName == "" || environment == "" {
		log.Printf("warning: LOGS_EXCHANGE, SERVICE_NAME, and ENVIRONMENT must be set; audit events disabled")
	} else {
		auditPublisher := telemetry.NewRabbitPublisher(amqpURL, logsExchange)
		if err := auditPublisher.Connect(); err != nil {
			log.Printf("warning: failed to initialize telemetry publisher: %v", err)
		} else {
			telemetryPublisher = auditPublisher
		}
	}
	defer telemetryPublisher.Close()

	friendRepo := repositories.NewFriendRepository(database, publisher)
	userService := services.NewUserService(authClient)

	userHandler := handlers.NewUserHandler(userService, friendRepo)
	friendHandler := handlers.NewFriendHandler(friendRepo, userService, telemetryPublisher, telemetryConfig)

	if _, err := grpcsvc.StartGRPCServer(ctx, ":8085", friendRepo, authClient); err != nil {
		log.Fatalf("failed to start gRPC server: %v", err)
	}

	r := gin.Default()
	r.Use(gin.Logger(), gin.Recovery())

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

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
}
