package handlers

import (
	"context"
	"log/slog"

	auth_pb "remaster/shared/proto/auth"

	"remaster/services/auth/models"
	"remaster/services/auth/services"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
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

func (c *AuthHandler) Register(ctx context.Context, req *models.RegisterRequest) (*models.RegisterResponse, error) {
	// logger.Info("Registration request for email: %s", req.Email)

	metadata := extractRequestMetadata(ctx)

	registerReq := &models.RegisterRequest{
		Email:     req.Email,
		Password:  req.Password,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Phone:     req.Phone,
		UserType:  models.UserType(req.UserType),
	}

	authResp, err := c.authService.Register(ctx, registerReq, metadata)
	if err != nil {
		// logger.Printf("Registration failed for %s: %v", req.Email, err)
		return &models.RegisterResponse{
			Success: false,
			Message: "Registration failed",
		}, status.Error(codes.Internal, "internal server error")
	}

	// logger.Printf("User registered successfully: %s", authResp.User.ID)

	return &models.RegisterResponse{
		Success: true,
		Message: "Registration successful",
		AuthResponse: models.AuthResponse{
			User:         authResp.User,
			AccessToken:  authResp.AccessToken,
			RefreshToken: authResp.RefreshToken,
			ExpiresAt:    authResp.ExpiresAt,
		},
	}, nil
}

func extractRequestMetadata(ctx context.Context) *models.RequestMetadata {
	md, _ := metadata.FromIncomingContext(ctx)

	var userAgent, deviceID, ipAddress string

	if val, ok := md["user-agent"]; ok && len(val) > 0 {
		userAgent = val[0]
	}

	if val, ok := md["x-device-id"]; ok && len(val) > 0 {
		deviceID = val[0]
	}

	if p, ok := peer.FromContext(ctx); ok {
		ipAddress = p.Addr.String()
	}

	return &models.RequestMetadata{
		UserAgent: userAgent,
		IPAddress: ipAddress,
		DeviceID:  deviceID,
	}
}
