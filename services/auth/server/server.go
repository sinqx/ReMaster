package server

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"remaster/services/auth/handlers"
	oauth "remaster/services/auth/oAuth"
	"remaster/services/auth/repositories"
	"remaster/services/auth/services"
	"remaster/services/auth/utils"
	cfg "remaster/shared"
	"remaster/shared/connection"
	"remaster/shared/errors"
)

type Server struct {
	authHandler *handlers.AuthHandler
	Config *cfg.Config

	MongoManager *connection.MongoManager
	RedisManager *connection.RedisManager

	httpServer *http.Server
	grpcServer *grpc.Server

	v      *errors.Validator
	Logger *slog.Logger
}

func NewServer(config *cfg.Config, logger *slog.Logger, mongoMgr *connection.MongoManager, redisMgr *connection.RedisManager) *Server {
	jwtUtils := utils.NewJWTUtils(&config.JWT)
	googleAuthClient := oauth.NewGoogleAuthClient(&config.OAuth)
	authRepo := repositories.NewAuthRepository(mongoMgr.GetDatabase(), logger)
	authService := services.NewAuthService(*authRepo, redisMgr, googleAuthClient, jwtUtils)
	authHandler := handlers.NewAuthHandler(authService, logger)
	logger = logger.With(slog.String("service", "auth"))
	return &Server{
		Config:       config,
		Logger:       logger,
		MongoManager: mongoMgr,
		RedisManager: redisMgr,
		authHandler:  authHandler,
	}
}

func (s *Server) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(context.Background())

	// gRPC
	g.Go(func() error {
		s.Logger.Info("gRPC server starting", "host", s.Config.GRPC.Host, "port", s.Config.GRPC.Port)
		return s.startGRPCServer(ctx)
	})

	//  HTTP
	g.Go(func() error {
		s.Logger.Info("HTTP server starting", "host", s.Config.HTTP.Host, "port", s.Config.HTTP.Port)
		return s.startHTTPServer(ctx)
	})

	// graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-stop:
		s.Logger.Info("shutting down server due to OS signal")
		cancel()
	case <-ctx.Done():
		s.Logger.Info("shutting down server due to context cancellation")
	}

	s.shutdown()
	done := make(chan struct{})
	go func() {
		g.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.Logger.Info("server shutdown completed")
	case <-time.After(s.Config.HTTP.ShutdownTimeout):
		s.Logger.Warn("server shutdown timed out, forcing exit")
	}

	cancel()
	return nil
}

func (s *Server) shutdown() {
	s.Logger.Info("Cleaning up resources...")

	ctx, cancel := context.WithTimeout(context.Background(), s.Config.HTTP.ShutdownTimeout)
	defer cancel()

	if s.MongoManager != nil {
		if err := s.MongoManager.Disconnect(ctx); err != nil {
			s.Logger.Error("Error closing MongoDB", "error", err)
		} else {
			s.Logger.Info("MongoDB connection closed")
		}
	}

	if s.RedisManager != nil {
		if err := s.RedisManager.Disconnect(); err != nil {
			s.Logger.Error("Error closing Redis", "error", err)
		} else {
			s.Logger.Info("Redis connection closed")
		}
	}
}
