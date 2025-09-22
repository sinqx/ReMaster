package main

import (
	"context"
	"log"
	"os"

	"remaster/services/api-gateway/server"
	config "remaster/shared"
	"remaster/shared/connection"
	"remaster/shared/errors"
	logger "remaster/shared/logger"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger := logger.New(cfg.Log)
	logger.Info("Starting API Gateway")

	redisMgr := connection.NewRedisManager(&cfg.Redis)
	if err := redisMgr.Connect(context.Background()); err != nil {
		logger.Error("mongo connect error", "error", err)
		os.Exit(1)
	}

	errorHandler := errors.NewErrorHandler(logger)

	srv := server.NewServer(cfg, logger, errorHandler, redisMgr)
	if err := srv.Start(); err != nil {
		logger.Error("server stopped with error", "error", err)
		os.Exit(1)
	}

	logger.Info("API Gateway stopped gracefully")
}


// func main() {
// 	cfg, err := config.LoadConfig()
// 	if err != nil {
// 		log.Fatalf("failed to load config: %v", err)
// 	}
// 	appLogger := logger.New(cfg.Log).With(slog.String("service", "api-gateway"))

// 	gRPCClients, err := server.InitializeGRPCClients(appLogger, cfg)
// 	if err != nil {
// 		appLogger.Error("failed to initialize gRPC clients", "error", err)
// 		os.Exit(1)
// 	}

// 	errorHandler := errors.NewErrorHandler(appLogger)
// 	authHandler := handlers.NewAuthHandler(gRPCClients.Auth, appLogger, errorHandler)

// 	ginRouter := router.Setup(appLogger, authHandler)

// 	srv := server.New(cfg, appLogger, ginRouter)

// 	if err := srv.Start(); err != nil {
// 		appLogger.Error("server stopped with error", "error", err)
// 		os.Exit(1)
// 	}

// 	appLogger.Info("API Gateway stopped gracefully")
// }
