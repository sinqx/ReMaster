package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	cfg "remaster/shared"
	"remaster/shared/connection"
	auth_pb "remaster/shared/proto/auth"
)

type Server struct {
	Config *cfg.Config
	Logger *slog.Logger

	httpServer      *http.Server
	grpcConnections map[string]*grpc.ClientConn

	MongoManager *connection.MongoManager
	RedisManager *connection.RedisManager

	authClient auth_pb.AuthServiceClient
}

func NewServer(config *cfg.Config, logger *slog.Logger, mongoMgr *connection.MongoManager, redisMgr *connection.RedisManager) *Server {
	return &Server{
		Config:          config,
		Logger:          logger,
		MongoManager:    mongoMgr,
		RedisManager:    redisMgr,
		grpcConnections: make(map[string]*grpc.ClientConn),
	}
}

func (s *Server) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// GRPC clients
	if err := s.initializeGRPCClients(); err != nil {
		return err
	}

	g, ctx := errgroup.WithContext(ctx)

	// HTTP server
	g.Go(func() error {
		return s.startHTTPServer(ctx)
	})

	// Graceful shutdown
	return s.waitForShutdown(ctx, cancel, g)
}

func (s *Server) initializeGRPCClients() error {
	services := map[string]string{
		"auth": "auth-service:9090",
	}

	for serviceName, address := range services {
		if err := s.connectToService(serviceName, address); err != nil {
			return fmt.Errorf("failed to connect to %s: %w", serviceName, err)
		}
	}

	return nil
}

func (s *Server) connectToService(serviceName, address string) error {
	s.Logger.Info("Connecting to service", "service", serviceName, "address", address)

	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}

	conn.Connect()
	connectCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for {
		state := conn.GetState()
		if state == connectivity.Ready {
			break
		}
		if state == connectivity.TransientFailure || state == connectivity.Shutdown {
			conn.Close()
			return fmt.Errorf("connection failed: state %v", state)
		}
		if !conn.WaitForStateChange(connectCtx, state) {
			conn.Close()
			return fmt.Errorf("connection timeout")
		}
	}

	s.grpcConnections[serviceName] = conn
	s.authClient = auth_pb.NewAuthServiceClient(conn)

	s.Logger.Info("Connected to service", "service", serviceName)
	return nil
}

func (s *Server) waitForShutdown(ctx context.Context, cancel context.CancelFunc, g *errgroup.Group) error {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-stop:
		s.Logger.Info("shutting down server due to OS signal", "signal", sig.String())
		cancel()
	case <-ctx.Done():
		s.Logger.Info("shutting down server due to context cancellation")
	}

	s.shutdown()
	done := make(chan error, 1)
	go func() {
		done <- g.Wait()
	}()

	select {
	case err := <-done:
		if err != nil && err != context.Canceled {
			s.Logger.Error("server error during shutdown", "error", err)
			return err
		}
		s.Logger.Info("server shutdown completed")
	case <-time.After(s.Config.HTTP.ShutdownTimeout):
		s.Logger.Warn("server shutdown timed out, forcing exit")
		return fmt.Errorf("shutdown timeout")
	}

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
