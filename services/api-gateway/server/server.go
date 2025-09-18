package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	cfg "remaster/shared"
	auth_pb "remaster/shared/proto/auth"
)

type Server struct {
	Config    *cfg.Config
	Logger    *slog.Logger
	connMutex sync.RWMutex

	httpServer *http.Server
	router     *gin.Engine

	grpcConnections map[string]*grpc.ClientConn

	// GRPC clients
	authClient auth_pb.AuthServiceClient
}

func NewServer(config *cfg.Config, logger *slog.Logger) *Server {
	return &Server{
		Config:          config,
		Logger:          logger,
		grpcConnections: make(map[string]*grpc.ClientConn),
	}
}

func (s *Server) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Logger.Info("Starting API Gateway server",
		"environment", s.Config.App.Environment,
		"http_port", s.Config.HTTP.Port,
	)

	// Initialize components
	if err := s.initializeGRPCClients(); err != nil {
		s.Logger.Error("Failed to initialize servers", "error", err)
		return fmt.Errorf("initialization failed: %w", err)
	}

	// Setup routes
	s.setupRoutes()

	// Start server with graceful shutdown
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return s.runHTTPServer(ctx)
	})

	go s.shutdown(ctx, cancel, g)

	return g.Wait()
}

func (s *Server) initializeGRPCClients() error {
	s.Logger.Info("Initializing server components")
	services := []struct {
		name string
		init func(*grpc.ClientConn)
	}{
		{
			name: "auth",
			init: func(conn *grpc.ClientConn) {
				s.authClient = auth_pb.NewAuthServiceClient(conn)
			},
		},
	}

	for _, service := range services {
		address, err := s.Config.GetServiceGRPCAddr(service.name)
		if err != nil {
			return fmt.Errorf("failed to get auth service address: %w", err)
		}
		s.Logger.Info("Connecting to GRPC service",
			"service", service.name,
			"address", address,
		)

		conn, err := s.connectToService(service.name, address)
		if err != nil {
			s.Logger.Error("Failed to create connection with service:",
				"service", service.name,
				"error", err,
			)
			// Continue connecting to other services even if one fails
			continue
		}

		if conn == nil {
			s.Logger.Error("Connection is nil for service", "service", service.name)
			continue
		}

		s.connMutex.Lock()
		s.grpcConnections[service.name] = conn
		s.connMutex.Unlock()

		// Initialize the specific client
		service.init(conn)

		s.Logger.Info("Successfully connected to service",
			"service", service.name,
			"state", conn.GetState().String(),
		)
	}

	s.Logger.Info("Server initialization completed successfully")
	return nil
}

func (s *Server) connectToService(serviceName, address string) (*grpc.ClientConn, error) {
	s.Logger.Info("Connecting to gRPC service",
		"service", serviceName,
		"address", address)

	// Create connection with retry interceptor
	conn, err := grpc.NewClient(address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff: backoff.Config{
				BaseDelay:  1.0 * time.Second,
				Multiplier: 1.6,
				Jitter:     0.2,
				MaxDelay:   120 * time.Second,
			},
			MinConnectTimeout: 5 * time.Second,
		}),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             3 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	if err != nil {
		s.Logger.Error("failed to create gRPC client",
			"service", serviceName,
			"address", address,
			"error", err)
		return nil, err
	}

	s.Logger.Info("gRPC client created",
		"service", serviceName,
		"state", conn.GetState().String())

	return conn, nil
}

func (s *Server) runHTTPServer(ctx context.Context) error {
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%s", s.Config.HTTP.Host, s.Config.HTTP.Port),
		Handler:      s.router,
		ReadTimeout:  s.Config.HTTP.ReadTimeout,
		WriteTimeout: s.Config.HTTP.WriteTimeout,
		IdleTimeout:  s.Config.HTTP.IdleTimeout,
	}

	// Handle graceful shutdown
	go func() {
		<-ctx.Done()
		s.Logger.Info("Shutting down HTTP server")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.Config.HTTP.ShutdownTimeout)
		defer cancel()

		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			s.Logger.Error("HTTP server forced shutdown", "error", err)
		}
	}()

	s.Logger.Info("HTTP server started",
		"address", s.httpServer.Addr,
		"read_timeout", s.Config.HTTP.ReadTimeout,
		"write_timeout", s.Config.HTTP.WriteTimeout,
	)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTP server error: %w", err)
	}

	return nil
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

	// Start cleanup
	s.Logger.Info("Starting graceful shutdown")
	s.Logger.Info("Starting cleanup process")

	// Close GRPC connections
	s.connMutex.Lock()
	for name, conn := range s.grpcConnections {
		if conn != nil {
			if err := conn.Close(); err != nil {
				s.Logger.Error("Failed to close GRPC connection",
					"service", name,
					"error", err,
				)
			} else {
				s.Logger.Debug("GRPC connection closed", "service", name)
			}
		}
	}
	s.connMutex.Unlock()

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
