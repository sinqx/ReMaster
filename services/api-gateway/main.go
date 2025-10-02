package main

import (
	"context"
	"os"

	"remaster/services/api-gateway/server"
	config "remaster/shared"
	"remaster/shared/connection"
	"remaster/shared/errors"
	"remaster/shared/logger"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		panic("failed to load config: " + err.Error())
	}

	logger := logger.Get(cfg.Log)
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
