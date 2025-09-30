package main

import (
	"context"
	"log"
	"os"

	"remaster/services/auth/server"
	config "remaster/shared"
	"remaster/shared/connection"
	logger "remaster/shared/logger"
	srv "remaster/shared/server"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger := logger.Get(cfg.Log)
	logger.Info("Starting Auth micro-service")

	// dependencies initialization
	mongoMgr := connection.NewMongoManager(&cfg.Mongo)
	redisMgr := connection.NewRedisManager(&cfg.Redis)

	base, _ := srv.NewBaseServer("auth", cfg, logger,
		srv.WithMongoManager(mongoMgr),
		srv.WithRedisManager(redisMgr),
	)

	// server
	authServer, err := server.NewServer(base)
	if err != nil {
		logger.Error("Failed to create auth server", "error", err)
		os.Exit(1)
	}

	// Start
	if err := authServer.Start(context.Background()); err != nil {
		logger.Error("Auth service stopped with error", "error", err)
		os.Exit(1)
	}
}
