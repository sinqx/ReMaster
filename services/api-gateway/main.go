package main

import (
	"log"
	"os"

	"remaster/services/api-gateway/server"
	config "remaster/shared"
	logger "remaster/shared/logger"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger := logger.New(cfg.Log)
	logger.Info("Starting API Gateway")

	srv := server.NewServer(cfg, logger)
	if err := srv.Start(); err != nil {
		logger.Error("server stopped with error", "error", err)
		os.Exit(1)
	}

	logger.Info("API Gateway stopped gracefully")
}
