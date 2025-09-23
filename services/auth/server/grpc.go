package server

import (
	"context"
	"fmt"
	"net"
	"time"

	auth_pb "remaster/shared/proto/auth"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

func (s *Server) startGRPCServer(ctx context.Context, grpcAddr string) error {
	s.Logger.Info("gRPC server starting with settings",
		"MaxRecvMsgSize", s.Config.GRPC.MaxReceiveSize,
		"MaxSendMsgSize", s.Config.GRPC.MaxSendSize,
		"ConnectionTimeout", s.Config.GRPC.ConnectionTimeout,
		"EnableReflection", s.Config.GRPC.EnableReflection,
		"EnableHealthCheck", s.Config.GRPC.EnableReflection,
	)

	// Create TCP listener
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		s.Logger.Error("Failed to create gRPC listener", "address", grpcAddr, "error", err)
		return fmt.Errorf("failed to create listener: %w", err)
	}

	// gRPC server with keepalive enforcement
	kaep := keepalive.EnforcementPolicy{
		MinTime:             30 * time.Second,
		PermitWithoutStream: true,
	}

	// Configure gRPC server options
	opts := []grpc.ServerOption{
		grpc.KeepaliveEnforcementPolicy(kaep),
		grpc.MaxRecvMsgSize(s.Config.GRPC.MaxReceiveSize),
		grpc.MaxSendMsgSize(s.Config.GRPC.MaxSendSize),
	}
	s.grpcServer = grpc.NewServer(opts...)

	// Register Auth service
	auth_pb.RegisterAuthServiceServer(s.grpcServer, s.authHandler)
	s.Logger.Info("Auth service registered on gRPC server")

	// Register health check service
	if s.Config.GRPC.EnableHealthCheck {
		healthServer := health.NewServer()
		healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
		grpc_health_v1.RegisterHealthServer(s.grpcServer, healthServer)
		s.Logger.Info("gRPC health check service registered")
	}

	// Register reflection service for development
	if s.Config.GRPC.EnableReflection {
		reflection.Register(s.grpcServer)
		s.Logger.Info("gRPC reflection service registered")
	}

	// Handle graceful shutdown
	go func() {
		<-ctx.Done()
		s.Logger.Info("Graceful shutdown initiated, stopping gRPC server...")

		// Allow 30 seconds for graceful shutdown
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		done := make(chan struct{})
		go func() {
			s.grpcServer.GracefulStop()
			close(done)
		}()

		select {
		case <-done:
			s.Logger.Info("gRPC server stopped gracefully")
		case <-shutdownCtx.Done():
			s.Logger.Warn("Graceful shutdown timeout, forcing stop")
			s.grpcServer.Stop()
		}
	}()

	s.Logger.Info("gRPC server starting to listen")

	// Start the server
	if err := s.grpcServer.Serve(lis); err != nil {
		// If the server was stopped intentionally, this is not an error
		if err == grpc.ErrServerStopped {
			s.Logger.Info("gRPC server was stopped intentionally")
			return nil
		}
		s.Logger.Error("gRPC server failed to serve", "error", err)
		return fmt.Errorf("gRPC server failed: %w", err)
	}

	return nil
}
