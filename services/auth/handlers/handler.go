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
		return nil, h.errorHandler.HandleGrpcError(err)
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
		return nil, h.errorHandler.HandleGrpcError(err)
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

func (h *AuthHandler) OAuthLogin(ctx context.Context, req *pb.OAuthLoginRequest) (*pb.OAuthLoginResponse, error) {
	h.logger.Info("OAuth login request", "provider", req.Provider)

	metadata := extractRequestMetadata(ctx)

	oauthReq := &models.OAuthLoginRequest{
		Provider: req.Provider,
		IDToken:  req.IdToken,
	}

	resp, err := h.authService.OAuthLogin(ctx, oauthReq, metadata)
	if err != nil {
		h.logger.Error("OAuth login failed", "provider", req.Provider, "error", err)
		return nil, h.errorHandler.HandleGrpcError(err)
	}

	return &pb.OAuthLoginResponse{
		Success:      true,
		Message:      "OAuth login successful",
		UserId:       resp.User.ID,
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresAt:    resp.ExpiresAt,
		UserType:     string(resp.User.UserType),
	}, nil
}

func (h *AuthHandler) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	h.logger.Info("Token refresh request", "refresh_token", req.RefreshToken)

	metadata := extractRequestMetadata(ctx)

	refreshReq := &models.RefreshTokenRequest{
		RefreshToken: req.RefreshToken,
	}

	resp, err := h.authService.RefreshToken(ctx, refreshReq, metadata)
	if err != nil {
		h.logger.Error("Token refresh failed", "error", err)
		return nil, h.errorHandler.HandleGrpcError(err)
	}

	return &pb.RefreshTokenResponse{
		Success:      true,
		Message:      "Token refresh successful",
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresAt:    resp.ExpiresAt,
	}, nil
}

func (h *AuthHandler) ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
	h.logger.Info("Token validation request", "access_token", req.AccessToken)

	validateReq := &models.ValidateTokenRequest{
		AccessToken: req.AccessToken,
	}

	resp, err := h.authService.ValidateToken(ctx, validateReq)
	if err != nil {
		h.logger.Error("Token validation failed", "error", err)
		return nil, h.errorHandler.HandleGrpcError(err)
	}

	return &pb.ValidateTokenResponse{
		Valid:      resp.Valid,
		UserId:     resp.UserID,
		UserType:   string(resp.UserType),
		IsActive:   resp.IsActive,
		IsVerified: resp.IsVerified,
		ExpiresAt:  resp.ExpiresAt,
		Message:    "Token validated",
	}, nil
}

func (h *AuthHandler) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	h.logger.Info("Logout request")

	logoutReq := &models.LogoutRequest{
		RefreshToken: req.RefreshToken,
	}

	err := h.authService.Logout(ctx, logoutReq)
	if err != nil {
		h.logger.Error("Logout failed", "error", err)
		return nil, h.errorHandler.HandleGrpcError(err)
	}

	return &pb.LogoutResponse{
		Success: true,
		Message: "Logout successful",
	}, nil
}

func (h *AuthHandler) ChangePassword(ctx context.Context, req *pb.ChangePasswordRequest) (*pb.ChangePasswordResponse, error) {
	h.logger.Info("Change password request")

	changeReq := &models.ChangePasswordRequest{
		UserId:      req.UserId,
		OldPassword: req.OldPassword,
		NewPassword: req.NewPassword,
	}

	err := h.authService.ChangePassword(ctx, changeReq)
	if err != nil {
		h.logger.Error("Password change failed", "error", err)
		return nil, h.errorHandler.HandleGrpcError(err)
	}

	return &pb.ChangePasswordResponse{
		Success: true,
		Message: "Password changed successfully",
	}, nil
}

func (h *AuthHandler) Health(ctx context.Context, _ *pb.HealthRequest) (*pb.HealthResponse, error) {
	return &pb.HealthResponse{
		Status:    "ok",
		Timestamp: timestamppb.Now(),
		Checks:    map[string]string{},
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
