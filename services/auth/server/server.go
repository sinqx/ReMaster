package server

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"remaster/services/auth/handlers"
	"remaster/services/auth/repositories"
	"remaster/services/auth/services"
	cfg "remaster/shared"
	"remaster/shared/connection"
)

type Server struct {
	Config       *cfg.Config
	Logger       *slog.Logger
	MongoManager *connection.MongoManager
	RedisManager *connection.RedisManager

	httpServer  *http.Server
	grpcServer  *grpc.Server
	authHandler *handlers.AuthHandler
}

func NewServer(config *cfg.Config, logger *slog.Logger, mongoMgr *connection.MongoManager, redisMgr *connection.RedisManager) *Server {
	authRepo := repositories.NewAuthRepository(mongoMgr.GetDatabase(), logger)
	authService := services.NewAuthService(*authRepo, redisMgr, &config.OAuth, &config.JWT)
	authHandler := handlers.NewAuthHandler(authService, logger)
	return &Server{
		Config:       config,
		Logger:       logger,
		MongoManager: mongoMgr,
		RedisManager: redisMgr,
		authHandler:  authHandler,
	}
}

func (s *Server) Start() error {
	g, ctx := errgroup.WithContext(context.Background())

	// gRPC
	g.Go(func() error {
		return s.startGRPCServer(ctx)
	})

	//  HTTP
	g.Go(func() error {
		return s.startHTTPServer(ctx)
	})

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-stop:
		s.Logger.Info("shutting down server due to OS signal")
	case <-ctx.Done():
		s.Logger.Info("shutting down server due to context cancellation")
	}

	s.shutdown()
	return g.Wait()
}

func (s *Server) shutdown() {
	log.Println("Cleaning up resources...")

	ctx, close := context.WithTimeout(context.Background(), s.Config.Mongo.ConnectTimeout)
	defer close()

	if s.MongoManager != nil {
		if err := s.MongoManager.Disconnect(ctx); err != nil {
			s.Logger.Error("Error closing MongoDB", "error", err)
		}
	}

	if s.RedisManager != nil {
		if err := s.RedisManager.Disconnect(); err != nil {
			s.Logger.Error("Error closing Redis", "error", err)
		}
	}
}
