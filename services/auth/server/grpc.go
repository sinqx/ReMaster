package server

import (
	"context"
	"net"

	auth_pb "remaster/shared/proto/auth"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/reflection"
)

func (s *Server) startGRPCServer(ctx context.Context, grpcAddr string) error {
	// Create a TCP listener on the specified address
	grpcAddr = "0.0.0.0:9090"
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		s.Logger.Error("failed to create gRPC listener", "address", grpcAddr, "error", err)
		return err
	}
	s.Logger.Info("gRPC listener created", "address", grpcAddr)

	// Set gRPC server options
	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(s.Config.GRPC.MaxReceiveSize),
		grpc.MaxSendMsgSize(s.Config.GRPC.MaxSendSize),
		grpc.UnaryInterceptor(s.loggingInterceptor()),
	}
	s.grpcServer = grpc.NewServer(opts...)

	// Register the Auth service with the gRPC server
	auth_pb.RegisterAuthServiceServer(s.grpcServer, s.authHandler)
	s.Logger.Info("Auth service registered on gRPC server")

	// Enable health check service if configured
	if s.Config.GRPC.EnableHealthCheck {
		healthServer := health.NewServer()
		healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
		grpc_health_v1.RegisterHealthServer(s.grpcServer, healthServer)
		s.Logger.Info("gRPC health check service registered")
	}

	// Enable reflection service if configured
	if s.Config.GRPC.EnableReflection {
		reflection.Register(s.grpcServer)
		s.Logger.Info("gRPC reflection service registered")
	}

	// Handle graceful shutdown
	go func() {
		<-ctx.Done()
		s.Logger.Info("Graceful shutdown signal received, stopping gRPC server...")
		s.grpcServer.GracefulStop()
		s.Logger.Info("gRPC server stopped gracefully")
	}()

	// Start serving gRPC requests
	s.Logger.Info("gRPC server starting to listen")
	if err := s.grpcServer.Serve(lis); err != nil {
		s.Logger.Error("gRPC server failed to serve", "error", err)
		if err == grpc.ErrServerStopped {
			s.Logger.Info("gRPC server was stopped intentionally")
			return nil
		}
		return err
	}
	s.Logger.Info("gRPC server successfully listening on", "address", lis.Addr().String())

	return nil
}

func (s *Server) loggingInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		peer, ok := peer.FromContext(ctx)
		if ok {
			s.Logger.Info("Incoming gRPC request",
				"method", info.FullMethod,
				"peer", peer.Addr.String())
		}

		resp, err := handler(ctx, req)

		if err != nil {
			s.Logger.Error("gRPC request failed",
				"method", info.FullMethod,
				"error", err)
		} else {
			s.Logger.Debug("gRPC request completed",
				"method", info.FullMethod)
		}

		return resp, err
	}
}
