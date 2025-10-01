package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	cfg "remaster/shared"
)

type InterceptorConfig struct {
	EnableLogging  bool
	EnableRecovery bool
}

type GRPCServerConfig struct {
	Address           string
	Logger            *slog.Logger
	Config            *cfg.GRPCConfig
	EnableHealthCheck bool
	EnableReflection  bool
	InterceptorConfig InterceptorConfig
}

type GRPCServerManager struct {
	server   *grpc.Server
	listener net.Listener
	logger   *slog.Logger
	config   GRPCServerConfig
}

func (m *GRPCServerManager) GetGRPCServer() *grpc.Server {
	return m.server
}

func NewGRPCServer(cfg GRPCServerConfig) (*GRPCServerManager, error) {
	cfg.Logger.Info("Creating gRPC server",
		"address", cfg.Address,
		"max_recv_size", cfg.Config.MaxReceiveSize,
		"max_send_size", cfg.Config.MaxSendSize,
	)

	lis, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		cfg.Logger.Error("Failed to create gRPC listener", "address", cfg.Address, "error", err)
		return nil, fmt.Errorf("failed to create listener: %w", err)
	}

	// assemble unary + stream interceptors
	var unaryInterceptors []grpc.UnaryServerInterceptor
	var streamInterceptors []grpc.StreamServerInterceptor

	// Recovery first (outermost)
	if cfg.InterceptorConfig.EnableRecovery {
		unaryInterceptors = append(unaryInterceptors, RecoveryUnary(cfg.Logger))
		streamInterceptors = append(streamInterceptors, RecoveryStream(cfg.Logger))
		cfg.Logger.Info("Recovery interceptor enabled")
	}

	// correlation id
	unaryInterceptors = append(unaryInterceptors, CorrelationUnary(cfg.Logger))
	// logging
	if cfg.InterceptorConfig.EnableLogging {
		unaryInterceptors = append(unaryInterceptors, LoggingUnary(cfg.Logger))
		streamInterceptors = append(streamInterceptors, nil)
		cfg.Logger.Info("Logging interceptor enabled")
	}

	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(cfg.Config.MaxReceiveSize),
		grpc.MaxSendMsgSize(cfg.Config.MaxSendSize),
	}

	if len(unaryInterceptors) > 0 {
		opts = append(opts, grpc.ChainUnaryInterceptor(unaryInterceptors...))
	}
	if len(streamInterceptors) > 0 {
		opts = append(opts, grpc.ChainStreamInterceptor(streamInterceptors...))
	}

	grpcServer := grpc.NewServer(opts...)

	// health + reflection
	if cfg.EnableHealthCheck {
		hSrv := health.NewServer()
		grpc_health_v1.RegisterHealthServer(grpcServer, hSrv)
		// set NOT_SERVING by default, set SERVING in Start() after ListenOK
		cfg.Logger.Info("Health check service registered")
	}
	if cfg.EnableReflection {
		reflection.Register(grpcServer)
		cfg.Logger.Info("Reflection service registered")
	}

	return &GRPCServerManager{
		server:   grpcServer,
		listener: lis,
		logger:   cfg.Logger,
		config:   cfg,
	}, nil
}

// Start gRPC server
func (m *GRPCServerManager) Start(ctx context.Context) error {
	m.logger.Info("Starting gRPC server", "address", m.config.Address)

	// Handle graceful shutdown
	go m.handleShutdown(ctx)

	// Start serving (blocking)
	if err := m.server.Serve(m.listener); err != nil {
		if err == grpc.ErrServerStopped {
			m.logger.Info("gRPC server was stopped intentionally")
			return nil
		}
		m.logger.Error("gRPC server failed to serve", "error", err)
		return fmt.Errorf("gRPC server failed: %w", err)
	}

	return nil
}

func (m *GRPCServerManager) handleShutdown(ctx context.Context) {
	<-ctx.Done()
	m.logger.Info("Graceful shutdown initiated, stopping gRPC server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		m.server.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		m.logger.Info("gRPC server stopped gracefully")
	case <-shutdownCtx.Done():
		m.logger.Warn("Graceful shutdown timeout, forcing stop")
		m.server.Stop()
	}
}

func (m *GRPCServerManager) Stop() {
	m.server.Stop()
}

func (m *GRPCServerManager) GracefulStop() {
	m.server.GracefulStop()
}
