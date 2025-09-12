package main

import (
	"context"
	"log"
	"os"

	"remaster/services/api-gateway/server"
	config "remaster/shared"
	"remaster/shared/connection"
	logger "remaster/shared/logger"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger := logger.New(cfg.Log)
	logger.Info("Starting API Gateway", "log_level", cfg.Log.Level)

	mongoMgr := connection.NewMongoManager(&cfg.Mongo)
	if err := mongoMgr.Connect(context.Background()); err != nil {
		logger.Error("mongo connect error", "error", err)
		os.Exit(1)
	}

	redisMgr := connection.NewRedisManager(&cfg.Redis)
	if err := redisMgr.Connect(context.Background()); err != nil {
		logger.Error("redis connect error", "error", err)
		os.Exit(1)
	}

	srv := server.NewServer(cfg, logger, mongoMgr, redisMgr)
	if err := srv.Start(); err != nil {
		logger.Error("server stopped with error", "error", err)
		os.Exit(1)
	}

	logger.Info("API Gateway stopped gracefully")
}