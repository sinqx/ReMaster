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
	Name         string
	Config       *cfg.Config
	Logger       *slog.Logger
	MongoManager *connection.MongoManager
	RedisManager *connection.RedisManager

	authHandler *handlers.AuthHandler
	grpcServer  *grpc.Server
}

func NewServer(config *cfg.Config, logger *slog.Logger, mongoMgr *connection.MongoManager, redisMgr *connection.RedisManager) *Server {
	serviceName := "auth"

	serviceLogger := logger.With(slog.String("service", serviceName))
	jwtUtils := utils.NewJWTUtils(&config.JWT)
	googleAuthClient := oauth.NewGoogleAuthClient(&config.OAuth)

	authRepo := repositories.NewAuthRepository(mongoMgr.GetDatabase(), serviceLogger)
	authService := services.NewAuthService(authRepo, redisMgr, googleAuthClient, jwtUtils)
	authHandler := handlers.NewAuthHandler(authService, serviceLogger)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := authRepo.EnsureIndexes(ctx); err != nil {
		serviceLogger.Error("Failed to create database indexes", "error", err)
		os.Exit(1)
	}
	serviceLogger.Info("Database indexes created successfully")

	return &Server{
		Name:         serviceName,
		Config:       config,
		Logger:       serviceLogger,
		MongoManager: mongoMgr,
		RedisManager: redisMgr,
		authHandler:  authHandler,
	}
}

func (s *Server) Start() error {
	grpcAddr, err := s.Config.GetServiceGRPCAddr(s.Name)
	if err != nil {
		s.Logger.Error("Failed to get gRPC service address", "error", err)
		return fmt.Errorf("failed to get gRPC address: %w", err)
	}

	s.Logger.Info("Creating server for", "service", s.Name, "grpc_address", grpcAddr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return s.startGRPCServer(gCtx, grpcAddr)
	})

	g.Go(func() error {
		return s.handleShutdown(ctx, cancel)
	})

	if err := g.Wait(); err != nil {
		s.Logger.Error("Server stopped with error", "error", err)
		return err
	}
	return nil
}

func (s *Server) handleShutdown(ctx context.Context, cancel context.CancelFunc) error {
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

	s.Logger.Info("Graceful shutdown completed")
	return nil
}
