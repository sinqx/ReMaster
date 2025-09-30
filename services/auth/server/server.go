package server

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"golang.org/x/sync/errgroup"

	"remaster/services/auth/handlers"
	oauth "remaster/services/auth/oauth"
	"remaster/services/auth/repositories"
	"remaster/services/auth/services"
	"remaster/services/auth/utils"
	auth_pb "remaster/shared/proto/auth"
	"remaster/shared/server"
)

type Server struct {
	Base        *server.BaseServer
	authHandler *handlers.AuthHandler
	grpcMgr     *server.GRPCServerManager
}

func NewServer(base *server.BaseServer) (*Server, error) {
	serviceLogger := base.Logger.With(slog.String("service", base.Name))
	jwtUtils := utils.NewJWTUtils(&base.Config.JWT)
	oauthFactory := oauth.NewProviderFactory(&base.Config.OAuth)

	authRepo := repositories.NewAuthRepository(base.MongoMgr.GetDatabase(), serviceLogger)
	authService := services.NewAuthService(authRepo, base.RedisMgr, oauthFactory, jwtUtils)
	authHandler := handlers.NewAuthHandler(authService, serviceLogger)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := authRepo.EnsureIndexes(ctx); err != nil {
		serviceLogger.Error("Failed to create database indexes", "error", err)
		os.Exit(1)
	}
	serviceLogger.Info("Database indexes created successfully")

	return &Server{
		Base:        base,
		authHandler: authHandler,
	}, nil
}

func (s *Server) Start(ctx context.Context) error {
	grpcAddr, err := s.Base.Config.GetServiceGRPCAddr(s.Base.Name)
	if err != nil {
		return fmt.Errorf("failed to get gRPC service address: %w", err)
	}

	s.Base.Logger.Info("Creating gRPC server", "address", grpcAddr)

	grpcCfg := server.GRPCServerConfig{
		Address:           grpcAddr,
		Logger:            s.Base.Logger,
		Config:            &s.Base.Config.GRPC,
		EnableHealthCheck: true,
		EnableReflection:  s.Base.Config.GRPC.EnableReflection,
		InterceptorConfig: server.InterceptorConfig{
			EnableLogging:  true,
			EnableRecovery: true,
		},
	}
	grpcMgr, err := server.NewGRPCServer(grpcCfg)
	if err != nil {
		return err
	}
	s.grpcMgr = grpcMgr

	// register Auth
	auth_pb.RegisterAuthServiceServer(grpcMgr.GetGRPCServer(), s.authHandler)
	s.Base.Logger.Info("Auth service registered")

	// graceful shutdown
	g, gCtx := errgroup.WithContext(ctx)
	gCtx, cancel := context.WithCancel(gCtx)

	g.Go(func() error {
		return grpcMgr.Start(gCtx)
	})

	g.Go(func() error {
		s.Base.WaitForShutdownSignal(func() { cancel() })
		s.Base.Cleanup(context.Background())
		return nil
	})

	return g.Wait()
}
