package main

import (
	"context"
	"log/slog"
	"os"

	"remaster/services/auth/handlers"
	"remaster/services/auth/oauth"
	"remaster/services/auth/repositories"
	"remaster/services/auth/services"
	"remaster/services/auth/utils"
	config "remaster/shared"
	auth_pb "remaster/shared/proto/auth"
	"remaster/shared/server"
)

func main() {
	// Load config
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		panic("failed to load config: " + err.Error())
	}

	// Logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Build server
	srv, err := server.NewServer(server.ServerConfig{
		Name:              "auth",
		Config:            cfg,
		Logger:            logger,
		EnableHealthCheck: true,
		EnableReflection:  true,
		InterceptorConfig: server.InterceptorConfig{
			EnableLogging:  true,
			EnableRecovery: true,
		},
		Dependencies: []server.ServerOption{
			server.WithMongo(context.Background()),
			// server.WithRedis(context.Background()),
		},
	})
	if err != nil {
		logger.Error("failed to initialize server", "error", err)
		os.Exit(1)
	}

	// Dependencies
	jwtUtils := utils.NewJWTUtils(&cfg.JWT)
	oauthFactory := oauth.NewProviderFactory(&cfg.OAuth)
	mongoMgr := srv.MongoMgr.GetDatabase()

	// Business logic
	authRepo := repositories.NewAuthRepository(mongoMgr, logger)
	authService := services.NewAuthService(authRepo, oauthFactory, jwtUtils, logger)
	authHandler := handlers.NewAuthHandler(authService, srv.ErrorHandler, srv.Logger)

	// Register gRPC service
	auth_pb.RegisterAuthServiceServer(srv.GetGRPCServer(), authHandler)
	logger.Info("auth service registered on gRPC server")

	// Start
	if err := srv.Start(context.Background()); err != nil {
		logger.Error("server exited with error", "error", err)
	}
}
