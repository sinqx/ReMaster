package handlers

import (
	"context"
	"log/slog"

	pb "remaster/shared/proto/auth"

	"remaster/services/auth/models"
	"remaster/services/auth/services"

	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type AuthHandler struct {
	pb.UnimplementedAuthServiceServer
	authService *services.AuthService
	logger      *slog.Logger
}

func NewAuthHandler(authService *services.AuthService, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		logger:      logger,
	}
}

func (c *AuthHandler) Registration(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	c.logger.Info("Registration request for email", "registration", req.Email)

	metadata := extractRequestMetadata(ctx)

	registerReq := &models.RegisterRequest{
		Email:     req.Email,
		Password:  req.Password,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Phone:     req.Phone,
		UserType:  models.UserType(req.UserType),
	}

	resp, err := c.authService.CreateUser(ctx, registerReq, metadata)
	if err != nil {
		c.logger.Error("Registration failed", "error", err)
		return &pb.RegisterResponse{
			Success: false,
			Message: "3Registration failed",
		}, err
	}

	pbResp := &pb.RegisterResponse{
		Success:      true,
		Message:      "Registration successful",
		UserId:       resp.User.ID,
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresAt:    resp.ExpiresAt,
		UserType:     string(resp.User.UserType),
		IsActive:     resp.User.IsActive,
		IsVerified:   resp.User.IsVerified,
		CreatedAt:    timestamppb.New(resp.User.CreatedAt),
	}

	c.logger.Info("User registered successfully:", req.Email, resp.User.ID)

	return pbResp, nil
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
