package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	cfg "remaster/shared"
	"remaster/shared/connection"
)

type Server struct {
	config       *cfg.Config
	mongoManager *connection.MongoManager
	redisManager *connection.RedisManager
	httpServer   *http.Server
	grpcServer   *grpc.Server
	// authService *services.AuthService
}

// start server
func main() {
	log.Println("Starting Auth micro-service")

	cfg, err := cfg.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	server, err := NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func NewServer(cfg *cfg.Config) (*Server, error) {
	// MongoDB connection
	mongoMgr := connection.NewMongoManager(&cfg.Mongo)
	if err := mongoMgr.Connect(context.Background()); err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}

	// Redis connection
	redisMgr := connection.NewRedisManager(&cfg.Redis)
	if err := redisMgr.Connect(context.Background()); err != nil {
		return nil, fmt.Errorf("redis connect: %w", err)
	}

	// create services and repositories
	// userRepo := repositories.NewUserRepository(mongoManager)
	// authService := services.NewAuthService(userRepo, redisManager, &cfg.JWT, &cfg.OAuth)

	return &Server{
		config:       cfg,
		mongoManager: mongoMgr,
		redisManager: redisMgr,
		// authService: authService,
	}, nil
}

func (s *Server) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	// gRPC server
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.startGRPCServer(ctx); err != nil {
			log.Printf("gRPC server error: %v", err)
		}
	}()

	// HTTP сервер server
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.startHTTPServer(ctx); err != nil {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	<-c
	log.Println("Shutting down Auth service")

	cancel()
	s.shutdown()

	wg.Wait()
	log.Println("Auth Service stopped")

	return nil
}

func (s *Server) startGRPCServer(ctx context.Context) error {
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%s", s.config.GRPC.Host, s.config.GRPC.Port))
	if err != nil {
		return fmt.Errorf("failed to listen on gRPC port: %w", err)
	}

	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(s.config.GRPC.MaxReceiveSize),
		grpc.MaxSendMsgSize(s.config.GRPC.MaxSendSize),
	}

	s.grpcServer = grpc.NewServer(opts...)

	// authController := controllers.NewAuthController(s.authService)
	// auth.RegisterAuthServiceServer(s.grpcServer, authController)

	if s.config.GRPC.EnableHealthCheck {
		healthServer := health.NewServer()
		healthServer.SetServingStatus("auth-service", grpc_health_v1.HealthCheckResponse_SERVING)
		grpc_health_v1.RegisterHealthServer(s.grpcServer, healthServer)
	}

	// reflection + debugging
	if s.config.GRPC.EnableReflection {
		reflection.Register(s.grpcServer)
	}

	log.Printf("gRPC server starting on %s:%s", s.config.GRPC.Host, s.config.GRPC.Port)

	// start goroutine for graceful shutdown
	go func() {
		<-ctx.Done()
		log.Println("Stopping gRPC server...")
		s.grpcServer.GracefulStop()
	}()

	if err := s.grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve gRPC: %w", err)
	}

	return nil
}

func (s *Server) startHTTPServer(ctx context.Context) error {
	if s.config.App.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	router.GET("/health", func(c *gin.Context) {
		mongoErr := s.mongoManager.HealthCheck(ctx)
		redisErr := s.redisManager.HealthCheck(ctx)

		status := "healthy"
		httpStatus := http.StatusOK

		checks := map[string]string{
			"mongodb": "ok",
			"redis":   "ok",
		}

		if mongoErr != nil {
			checks["mongodb"] = mongoErr.Error()
			status = "unhealthy"
			httpStatus = http.StatusServiceUnavailable
		}

		if redisErr != nil {
			checks["redis"] = redisErr.Error()
			status = "unhealthy"
			httpStatus = http.StatusServiceUnavailable
		}

		c.JSON(httpStatus, gin.H{
			"status":    status,
			"service":   "auth-service",
			"version":   s.config.App.Version,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"checks":    checks,
		})
	})

	// Metrics endpoint (Prometheus)
	router.GET("/metrics", func(c *gin.Context) {
		c.String(http.StatusOK, "# Metrics endpoint - TODO: implement Prometheus metrics")
	})

	// Ready check (Kubernetes)
	router.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ready",
			"service": "auth-service",
		})
	})

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%s", s.config.HTTP.Host, s.config.HTTP.Port),
		Handler:      router,
		ReadTimeout:  s.config.HTTP.ReadTimeout,
		WriteTimeout: s.config.HTTP.WriteTimeout,
	}

	log.Printf("🌐 HTTP server starting on %s:%s", s.config.HTTP.Host, s.config.HTTP.Port)

	// Graceful shutdown
	go func() {
		<-ctx.Done()
		log.Println("Stopping HTTP server...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.config.HTTP.ShutdownTimeout)
		defer cancel()

		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}
	}()

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	return nil
}

// Mongo and Redis shutdown
func (s *Server) shutdown() {
	log.Println("Cleaning up resources...")

	ctx, close := context.WithTimeout(context.Background(), s.config.Mongo.ConnectTimeout)
	defer close()

	if s.mongoManager != nil {
		if err := s.mongoManager.Disconnect(ctx); err != nil {
			log.Printf("Error closing MongoDB: %v", err)
		}
	}

	if s.redisManager != nil {
		if err := s.redisManager.Disconnect(); err != nil {
			log.Printf("Error closing Redis: %v", err)
		}
	}
}

// func (s *Server) createIndexes(ctx context.Context) error {
// 	log.Println("Creating indexes")

// 	userRepo := repositories.NewUserRepository(s.mongoManager)
// 	if err := userRepo.CreateIndexes(ctx); err != nil {
// 		return fmt.Errorf("failed to create indexes: %w", err)
// 	}

// 	log.Println("indexes created")
// 	return nil
// }
