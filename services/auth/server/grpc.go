package server

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func (s *Server) startGRPCServer(ctx context.Context) error {
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%s", s.Config.GRPC.Host, s.Config.GRPC.Port))
	if err != nil {
		s.Logger.Error("failed to listen on gRPC port", "error", err)
		return err
	}

	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(s.Config.GRPC.MaxReceiveSize),
		grpc.MaxSendMsgSize(s.Config.GRPC.MaxSendSize),
	}

	s.grpcServer = grpc.NewServer(opts...)

	// authController := controllers.NewAuthController(s.authService)
	// auth.RegisterAuthServiceServer(s.grpcServer, authController)

	if s.Config.GRPC.EnableHealthCheck {
		healthServer := health.NewServer()
		healthServer.SetServingStatus("auth-service", grpc_health_v1.HealthCheckResponse_SERVING)
		grpc_health_v1.RegisterHealthServer(s.grpcServer, healthServer)
	}

	// reflection + debugging
	if s.Config.GRPC.EnableReflection {
		reflection.Register(s.grpcServer)
	}

	// start goroutine for graceful shutdown
	go func() {
		<-ctx.Done()
		s.Logger.Info("Stopping gRPC server...")
		s.grpcServer.GracefulStop()
	}()

	if err := s.grpcServer.Serve(lis); err != nil {
		s.Logger.Error("failed to serve gRPC", "error", err)
		return err
	}

	return nil
}
