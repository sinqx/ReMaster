package server

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	cfg "remaster/shared"
	"remaster/shared/connection"
)

type BaseServer struct {
	Name   string
	Config *cfg.Config
	Logger *slog.Logger

	// optional dependencies (nil if not used)
	MongoMgr *connection.MongoManager
	RedisMgr *connection.RedisManager
	// KafkaMgr   *connection.KafkaManager
	// AWSManager *connection.AWSManager
}

type ServerOption func(*BaseServer) error

// NewBaseServer create a new BaseServer with optional dependencies
func NewBaseServer(name string, cfg *cfg.Config, logger *slog.Logger, opts ...ServerOption) (*BaseServer, error) {
	s := &BaseServer{
		Name:   name,
		Config: cfg,
		Logger: logger.With(slog.String("service", name)),
	}

	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, err
		}
	}

	return s, nil
}

// ============================================================================
// Dependencies injection options
// ============================================================================

func WithMongo(ctx context.Context) ServerOption {
	return func(s *BaseServer) error {
		s.Logger.Info("Initializing MongoDB connection...")

		if err := s.MongoMgr.Connect(ctx); err != nil {
			s.Logger.Error("Failed to connect to MongoDB", "error", err)
			return err
		}

		s.Logger.Info("MongoDB connected successfully")
		return nil
	}
}

func WithRedis(ctx context.Context) ServerOption {
	return func(s *BaseServer) error {
		s.Logger.Info("Initializing Redis connection...")

		if err := s.RedisMgr.Connect(ctx); err != nil {
			s.Logger.Error("Failed to connect to Redis", "error", err)
			return err
		}

		s.Logger.Info("Redis connected successfully")
		return nil
	}
}

// ============================================================================
// shutdown utils
// ============================================================================

// Cleanup закрывает все активные соединения
func (s *BaseServer) Cleanup(ctx context.Context) {
	s.Logger.Info("Cleaning up resources...")

	if s.MongoMgr != nil {
		if err := s.MongoMgr.Disconnect(ctx); err != nil {
			s.Logger.Error("Error closing MongoDB", "error", err)
		} else {
			s.Logger.Info("MongoDB connection closed")
		}
	}

	if s.RedisMgr != nil {
		if err := s.RedisMgr.Disconnect(); err != nil {
			s.Logger.Error("Error closing Redis", "error", err)
		} else {
			s.Logger.Info("Redis connection closed")
		}
	}

	s.Logger.Info("Cleanup completed")
}

func (s *BaseServer) WaitForShutdownSignal(cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	s.Logger.Info("Received shutdown signal", "signal", sig.String())
	cancel()
}
