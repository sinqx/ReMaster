package handlers

import (
	"context"
	"log/slog"

	"remaster/shared/errors"
	pb "remaster/shared/proto/auth"

	"remaster/services/auth/models"
	"remaster/services/auth/services"

	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type AuthHandler struct {
	pb.UnimplementedAuthServiceServer
	errorHandler *errors.ErrorHandler
	authService  *services.AuthService
	logger       *slog.Logger
}

func NewAuthHandler(authService *services.AuthService, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{
		authService:  authService,
		logger:       logger.With(slog.String("auth", "handler")),
		errorHandler: errors.NewErrorHandler(logger),
	}
}

func (h *AuthHandler) Registration(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	h.logger.Info("Registration request", "email", req.Email)

	metadata := extractRequestMetadata(ctx)

	registerReq := &models.RegisterRequest{
		Email:     req.Email,
		Password:  req.Password,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Phone:     req.Phone,
		UserType:  models.UserType(req.UserType),
	}

	resp, err := h.authService.CreateUser(ctx, registerReq, metadata)
	if err != nil {
		h.logger.Error("Registration failed", "error", err, "email", req.Email)
		return &pb.RegisterResponse{
			Success: false,
			Message: err.Error(),
		}, h.errorHandler.HandleGrpcError(err)
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

	h.logger.Info("User registered successfully:", req.Email, resp.User.ID)

	return pbResp, nil
}

func (h *AuthHandler) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	h.logger.Info("Login request", "email", req.Email)

	metadata := extractRequestMetadata(ctx)

	loginReq := &models.LoginRequest{
		Email:    req.Email,
		Password: req.Password,
	}

	resp, err := h.authService.AuthenticateUser(ctx, loginReq, metadata)
	if err != nil {
		h.logger.Error("Login failed", "error", err, "email", req.Email)
		return &pb.LoginResponse{
			Success: false,
			Message: err.Error(),
		}, h.errorHandler.HandleGrpcError(err)
	}

	pbResp := &pb.LoginResponse{
		Success:      true,
		Message:      "Login successful",
		UserId:       resp.User.ID,
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresAt:    resp.ExpiresAt,
		UserType:     string(resp.User.UserType),
		IsActive:     resp.User.IsActive,
		IsVerified:   resp.User.IsVerified,
	}

	h.logger.Info("User logged in successfully:", req.Email, resp.User.ID)

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
