package handlers

import (
	"log/slog"

	auth_pb "remaster/shared/proto/auth"

	"remaster/services/auth/services"
)

type AuthHandler struct {
	auth_pb.UnimplementedAuthServiceServer
	authService *services.AuthService
	logger      *slog.Logger
}

func NewAuthHandler(authService *services.AuthService, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		logger:      logger,
	}
}
