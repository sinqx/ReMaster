package server

import (
	"context"
	"fmt"
	"log/slog"
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
)

type Server struct {
	Name        string
	authHandler *handlers.AuthHandler
	Config      *cfg.Config

	MongoManager *connection.MongoManager
	RedisManager *connection.RedisManager

	grpcServer *grpc.Server

	// v      *errors.Validator
	Logger *slog.Logger
}

func NewServer(config *cfg.Config, logger *slog.Logger, mongoMgr *connection.MongoManager, redisMgr *connection.RedisManager) *Server {
	name := "auth"
	jwtUtils := utils.NewJWTUtils(&config.JWT)
	googleAuthClient := oauth.NewGoogleAuthClient(&config.OAuth)
	authRepo := repositories.NewAuthRepository(mongoMgr.GetDatabase(), logger)
	authService := services.NewAuthService(*authRepo, redisMgr, googleAuthClient, jwtUtils)
	authHandler := handlers.NewAuthHandler(authService, logger)
	logger = logger.With(slog.String("service", name))
	return &Server{
		Name:         name,
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

	grpcAddr, err := s.Config.GetServiceGRPCAddr(s.Name)
	if err != nil {
		s.Logger.Error("failed to get auth service address", "error", err)
		cancel()
		return fmt.Errorf("failed to get auth service gRPC address: %w", err)
	}
	// gRPC
	g.Go(func() error {
		s.Logger.Info("gRPC server starting", "Address", grpcAddr)
		cancel()
		return s.startGRPCServer(ctx, grpcAddr)
	})

	// graceful shutdown
	go s.shutdown(ctx, cancel, g)

	return g.Wait()
}

func (s *Server) shutdown(ctx context.Context, cancel context.CancelFunc, g *errgroup.Group) error {
	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		s.Logger.Info("Received shutdown signal", "signal", sig.String())
		cancel()
	case <-ctx.Done():
		s.Logger.Info("Context cancelled")
	}

	s.Logger.Info("Cleaning up resources...")
	if s.grpcServer != nil {
		stopped := make(chan struct{})
		go func() {
			s.grpcServer.GracefulStop()
			close(stopped)
		}()

		select {
		case <-stopped:
			s.Logger.Info("gRPC server stopped")
		case <-time.After(s.Config.HTTP.ShutdownTimeout):
			s.Logger.Warn("gRPC server shutdown timed out, forcing stop")
			s.grpcServer.Stop()
		}
	}

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

	// Wait for all goroutines with timeout
	done := make(chan error, 1)
	go func() {
		done <- g.Wait()
	}()

	select {
	case err := <-done:
		if err != nil && err != context.Canceled {
			s.Logger.Error("Error during shutdown", "error", err)
			return err
		}
		s.Logger.Info("Graceful shutdown completed")
	case <-time.After(s.Config.HTTP.ShutdownTimeout):
		s.Logger.Error("Shutdown timeout exceeded, forcing exit")
		return fmt.Errorf("shutdown timeout after %v", s.Config.HTTP.ShutdownTimeout)
	}

	return nil
}
