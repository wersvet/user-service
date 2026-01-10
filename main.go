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

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	"user-service/internal/db"
	grpcsvc "user-service/internal/grpc"
	"user-service/internal/handlers"
	"user-service/internal/middleware"
	"user-service/internal/observability"
	"user-service/internal/repositories"
	"user-service/internal/services"
)

func main() {
	dsn := os.Getenv("DB_DSN")
	jwtSecret := os.Getenv("JWT_SECRET")
	authGRPCAddr := os.Getenv("AUTH_GRPC_ADDR")
	amqpURL := os.Getenv("AMQP_URL")
	amqpExchange := os.Getenv("AMQP_EXCHANGE")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if amqpExchange == "" {
		amqpExchange = "app.logs"
	}

	if dsn == "" || jwtSecret == "" || authGRPCAddr == "" {
		log.Fatal("DB_DSN, JWT_SECRET, and AUTH_GRPC_ADDR environment variables must be set")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tracingShutdown, err := observability.InitTracing(ctx, "user-service")
	if err != nil {
		log.Printf("warning: failed to initialize tracing: %v", err)
	} else {
		defer func() {
			if err := tracingShutdown(context.Background()); err != nil {
				log.Printf("warning: tracing shutdown failed: %v", err)
			}
		}()
	}

	observability.InitMetrics(prometheus.DefaultRegisterer)

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
		pub, err := rabbitmq.NewPublisherWithExchange(amqpURL, amqpExchange)
		if err != nil {
			log.Printf("warning: failed to initialize RabbitMQ publisher: %v", err)
		} else {
			publisher = pub
		}
	}
	defer publisher.Close()

	friendRepo := repositories.NewFriendRepository(database, publisher)
	userService := services.NewUserService(authClient)

	userHandler := handlers.NewUserHandler(userService, friendRepo)
	friendHandler := handlers.NewFriendHandler(friendRepo, userService, publisher)

	if _, err := grpcsvc.StartGRPCServer(ctx, ":8085", friendRepo, authClient); err != nil {
		log.Fatalf("failed to start gRPC server: %v", err)
	}

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(otelgin.Middleware("user-service"))
	r.Use(middleware.Metrics())

	r.GET("/users/:id", userHandler.GetUserByID)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

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
