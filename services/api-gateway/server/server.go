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
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	cfg "remaster/shared"
	"remaster/shared/connection"
	auth_pb "remaster/shared/proto/auth"
)

type Server struct {
	Config *cfg.Config
	Logger *slog.Logger

	httpServer *http.Server
	router     *gin.Engine

	// GRPC connections
	grpcConnections map[string]*grpc.ClientConn
	connMutex       sync.RWMutex

	// Database connections
	MongoManager *connection.MongoManager
	RedisManager *connection.RedisManager

	// GRPC clients
	authClient auth_pb.AuthServiceClient
}

func NewServer(config *cfg.Config, logger *slog.Logger) *Server {
	// Set Gin mode based on environment
	if config.App.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

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
	if err := s.initialize(); err != nil {
		s.Logger.Error("Failed to initialize server", "error", err)
		return fmt.Errorf("initialization failed: %w", err)
	}

	// Setup routes
	s.setupRoutes()

	// Start server with graceful shutdown
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return s.runHTTPServer(ctx)
	})

	return s.shutdown(ctx, cancel, g)
}

func (s *Server) initialize() error {
	s.Logger.Info("Initializing server components")

	// Initialize GRPC clients with retries
	if err := s.initializeGRPCClients(); err != nil {
		return fmt.Errorf("failed to initialize GRPC clients: %w", err)
	}

	s.Logger.Info("Server initialization completed successfully")
	return nil
}

func (s *Server) initializeGRPCClients() error {
	authAddr, err := s.Config.GetServiceGRPCAddr("auth")
	if err != nil {
		return fmt.Errorf("failed to get auth service address: %w", err)
	}
	services := []struct {
		name    string
		address string
		init    func(*grpc.ClientConn)
	}{
		{
			name:    "auth",
			address: authAddr,
			init: func(conn *grpc.ClientConn) {
				s.authClient = auth_pb.NewAuthServiceClient(conn)
			},
		},
	}

	for _, service := range services {
		s.Logger.Info("Connecting to GRPC service",
			"service", service.name,
			"address", service.address,
		)

		conn, err := s.connectToService(service.name, service.address)
		if err != nil {
			s.Logger.Error("Failed to connect to service",
				"service", service.name,
				"error", err,
			)
			// Continue connecting to other services even if one fails
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

	return nil
}

func (s *Server) connectToService(serviceName, address string) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create connection with retry interceptor
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", serviceName, err)
	}

	// Wait for connection to be ready
	for {
		state := conn.GetState()
		if state == connectivity.Ready {
			break
		}
		if state == connectivity.TransientFailure || state == connectivity.Shutdown {
			conn.Close()
			return nil, fmt.Errorf("connection failed with state: %v", state)
		}
		if !conn.WaitForStateChange(ctx, state) {
			conn.Close()
			return nil, fmt.Errorf("connection timeout for %s", serviceName)
		}
	}

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

	// Close MongoDB
	if s.MongoManager != nil {
		if err := s.MongoManager.Disconnect(ctx); err != nil {
			s.Logger.Error("Failed to disconnect MongoDB", "error", err)
		} else {
			s.Logger.Info("MongoDB disconnected successfully")
		}
	}

	// Close Redis
	if s.RedisManager != nil {
		if err := s.RedisManager.Disconnect(); err != nil {
			s.Logger.Error("Failed to disconnect Redis", "error", err)
		} else {
			s.Logger.Info("Redis disconnected successfully")
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
