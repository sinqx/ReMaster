package main

import (
	"context"
	"log"
	"os"

	"remaster/services/auth/server"
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
	logger.Info("Starting Auth micro-service")

	// dependencies initialization
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

	// server initialization
	srv := server.NewServer(cfg, logger, mongoMgr, redisMgr)
	if err := srv.Start(); err != nil {
		logger.Error("server stopped with error", "error", err)
		os.Exit(1)
	}

	logger.Info("Auth Service stopped gracefully")
}
