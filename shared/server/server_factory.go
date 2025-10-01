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

	cfg "remaster/shared"
	"remaster/shared/connection"
	"remaster/shared/errors"
)

type Server struct {
	Name   string
	Config *cfg.Config
	Logger *slog.Logger

	// Core components 
	ErrorHandler *errors.ErrorHandler
	GRPCManager  *GRPCServerManager

	// Optional dependencies
	MongoMgr *connection.MongoManager
	RedisMgr *connection.RedisManager
	// KafkaMgr *connection.KafkaManager
	// AWSMgr   *connection.AWSManager

	// Internal
	cancelFunc context.CancelFunc
}

type ServerConfig struct {
	Name   string
	Config *cfg.Config
	Logger *slog.Logger

	// gRPC settings
	EnableHealthCheck bool
	EnableReflection  bool
	InterceptorConfig InterceptorConfig

	// Optional dependencies
	Dependencies []ServerOption
}

// for applying optional dependencies
type ServerOption func(*Server) error

func NewServer(config ServerConfig) (*Server, error) {
	logger := config.Logger.With(slog.String("service", config.Name))
	logger.Info("Initializing unified server")

	server := &Server{
		Name:         config.Name,
		Config:       config.Config,
		Logger:       logger,
		ErrorHandler: errors.NewErrorHandler(logger),
	}

	// Apply optional dependencies
	for _, opt := range config.Dependencies {
		if err := opt(server); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	// Create gRPC server
	grpcAddr, err := config.Config.GetServiceGRPCAddr(config.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get gRPC address: %w", err)
	}

	grpcCfg := GRPCServerConfig{
		Address:           grpcAddr,
		Logger:            logger,
		Config:            &config.Config.GRPC,
		EnableHealthCheck: config.EnableHealthCheck,
		EnableReflection:  config.EnableReflection,
		InterceptorConfig: config.InterceptorConfig,
	}

	grpcMgr, err := NewGRPCServer(grpcCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC server: %w", err)
	}

	server.GRPCManager = grpcMgr
	logger.Info("gRPC server created successfully", "address", grpcAddr)

	return server, nil
}

// ============================================================================
// Dependency Options
// ============================================================================

func WithMongo(ctx context.Context) ServerOption {
	return func(s *Server) error {
		s.Logger.Info("Connecting to MongoDB...")

		mgr := connection.NewMongoManager(&s.Config.Mongo)
		if err := mgr.Connect(ctx); err != nil {
			s.Logger.Error("Failed to connect to MongoDB", "error", err)
			return fmt.Errorf("mongodb connection failed: %w", err)
		}

		s.MongoMgr = mgr
		s.Logger.Info("MongoDB connected successfully")
		return nil
	}
}

func WithRedis(ctx context.Context) ServerOption {
	return func(s *Server) error {
		s.Logger.Info("Connecting to Redis...")

		mgr := connection.NewRedisManager(&s.Config.Redis)
		if err := mgr.Connect(ctx); err != nil {
			s.Logger.Error("Failed to connect to Redis", "error", err)
			return fmt.Errorf("redis connection failed: %w", err)
		}

		s.RedisMgr = mgr
		s.Logger.Info("Redis connected successfully")
		return nil
	}
}

// ============================================================================
// Server Lifecycle
// ============================================================================

// GetGRPCServer - возвращает gRPC сервер для регистрации сервисов
func (s *Server) GetGRPCServer() *grpc.Server {
	return s.GRPCManager.GetGRPCServer()
}

func (s *Server) Start(ctx context.Context) error {
	s.Logger.Info("Starting unified server")

	ctx, cancel := context.WithCancel(ctx)
	s.cancelFunc = cancel

	g, gCtx := errgroup.WithContext(ctx)

	// Start gRPC server
	g.Go(func() error {
		return s.GRPCManager.Start(gCtx)
	})

	// Wait for shutdown signal
	g.Go(func() error {
		s.waitForShutdownSignal(cancel)
		return nil
	})

	// Wait for all goroutines
	if err := g.Wait(); err != nil {
		s.Logger.Error("Server stopped with error", "error", err)
		return err
	}

	// Cleanup
	s.cleanup(context.Background())
	return nil
}

func (s *Server) Shutdown() {
	if s.cancelFunc != nil {
		s.cancelFunc()
	}
}

func (s *Server) waitForShutdownSignal(cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	s.Logger.Info("Received shutdown signal", "signal", sig.String())
	cancel()
}

func (s *Server) cleanup(ctx context.Context) {
	s.Logger.Info("Cleaning up resources...")

	// Cleanup with timeout
	cleanupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if s.MongoMgr != nil {
		if err := s.MongoMgr.Disconnect(cleanupCtx); err != nil {
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
