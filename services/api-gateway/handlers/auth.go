package handlers

import (
	"context"
	"log/slog"
	"time"

	m "remaster/services/api-gateway/models"
	u "remaster/services/api-gateway/utils"
	"remaster/shared/errors"
	auth_pb "remaster/shared/proto/auth"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	client       auth_pb.AuthServiceClient
	errorHandler *errors.ErrorHandler
	logger       *slog.Logger
	timeout      time.Duration
}

func NewAuthHandler(client auth_pb.AuthServiceClient, logger *slog.Logger, errorHandler *errors.ErrorHandler) *AuthHandler {
	return &AuthHandler{
		client:       client,
		errorHandler: errorHandler,
		logger:       logger.With(slog.String("api-gateway", "auth")),
		timeout:      10 * time.Second,
	}
}

func (h *AuthHandler) Register(c *gin.Context) {
	dto, ok := u.BindAndValidate[m.RegisterDTO](c, h.logger)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	h.logger.InfoContext(ctx, "Processing registration", "email", dto.Email, "user_type", dto.UserType)

	resp, err := h.client.Registration(c, &auth_pb.RegisterRequest{
		Email:     dto.Email,
		Password:  dto.Password,
		FirstName: dto.FirstName,
		LastName:  dto.LastName,
		Phone:     dto.Phone,
		UserType:  dto.UserType,
	})
	if err != nil {
		h.logger.ErrorContext(ctx, "gRPC registration failed", "error", err, "email", dto.Email)
		h.errorHandler.HandleGrpcToHttp(c, err)
		return
	}

	h.logger.InfoContext(ctx,
		"User registration successful",
		slog.String("user_id", resp.UserId),
		slog.String("email", dto.Email),
	)

	responseData := &m.AuthResponse{
		UserID:       resp.UserId,
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresAt:    resp.ExpiresAt,
		UserType:     resp.UserType,
	}

	u.SuccessResponse(c, resp.Message, responseData)
}

func (h *AuthHandler) Login(c *gin.Context) {
	dto, ok := u.BindAndValidate[m.LoginDTO](c, h.logger)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	h.logger.InfoContext(ctx, "Processing login", "email", dto.Email)

	resp, err := h.client.Login(c, &auth_pb.LoginRequest{
		Email:    dto.Email,
		Password: dto.Password,
	})
	if err != nil {
		h.logger.ErrorContext(ctx, "gRPC login failed", "error", err, "email", dto.Email)
		h.errorHandler.HandleGrpcToHttp(c, err)
		return
	}

	h.logger.InfoContext(ctx,
		"User login successful",
		slog.String("user_id", resp.UserId),
		slog.String("email", dto.Email),
	)

	responseData := &m.AuthResponse{
		UserID:       resp.UserId,
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresAt:    resp.ExpiresAt,
		UserType:     resp.UserType,
	}

	u.SuccessResponse(c, resp.Message, responseData)
}

func (h *AuthHandler) OAuthLogin(c *gin.Context) {
	req, ok := u.BindAndValidate[m.OAuthTokenRequest](c, h.logger)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	provider := req.Provider
	h.logger.InfoContext(ctx, "Processing OAuth login", "provider", provider)

	resp, err := h.client.OAuthLogin(c, &auth_pb.OAuthLoginRequest{
		Provider: provider,
		IdToken:  req.IDToken,
	})

	if err != nil {
		h.logger.ErrorContext(ctx, "OAuth login failed", "provider", provider, "error", err)
		h.errorHandler.HandleGrpcToHttp(c, err)
		return
	}

	h.logger.InfoContext(ctx,
		"OAuth login successful",
		slog.String("user_id", resp.UserId),
		slog.String("provider", provider),
	)

	responseData := &m.AuthResponse{
		UserID:       resp.UserId,
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresAt:    resp.ExpiresAt,
		UserType:     resp.UserType,
	}

	u.SuccessResponse(c, resp.Message, responseData)
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	type rt struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	req, ok := u.BindAndValidate[rt](c, h.logger)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	resp, err := h.client.RefreshToken(ctx, &auth_pb.RefreshTokenRequest{
		RefreshToken: req.RefreshToken,
	})

	if err != nil {
		h.errorHandler.HandleGrpcToHttp(c, err)
		return
	}
	h.logger.InfoContext(ctx, "Token refresh successful")

	responseData := &m.RefreshTokenResponse{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresAt:    resp.ExpiresAt,
	}

	u.SuccessResponse(c, resp.Message, responseData)
}
func (h *AuthHandler) ValidateToken(c *gin.Context) {
	type at struct {
		AccessToken string `json:"access_token" binding:"required"`
	}

	req, ok := u.BindAndValidate[at](c, h.logger)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	h.logger.InfoContext(ctx, "Processing token validation")

	resp, err := h.client.ValidateToken(ctx, &auth_pb.ValidateTokenRequest{
		AccessToken: req.AccessToken,
	})

	if err != nil {
		h.logger.ErrorContext(ctx, "Token validation failed", "error", err)
		h.errorHandler.HandleGrpcToHttp(c, err)
		return
	}

	h.logger.InfoContext(ctx, "Token validation successful")

	responseData := &m.ValidateTokenResponse{
		Valid:     resp.Valid,
		UserID:    resp.UserId,
		UserType:  resp.UserType,
		ExpiresAt: resp.ExpiresAt,
	}

	u.SuccessResponse(c, resp.Message, responseData)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	type rt struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	req, ok := u.BindAndValidate[rt](c, h.logger)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	h.logger.InfoContext(ctx, "Processing logout")

	resp, err := h.client.Logout(ctx, &auth_pb.LogoutRequest{
		RefreshToken: req.RefreshToken,
	})

	if err != nil {
		h.logger.ErrorContext(ctx, "Logout failed", "error", err)
		h.errorHandler.HandleGrpcToHttp(c, err)
		return
	}

	h.logger.InfoContext(ctx, "Logout successful")

	u.SuccessResponse(c, resp.Message, nil)
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	type cp struct {
		UserID      string `json:"user_id" binding:"required"`
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=8"`
	}

	req, ok := u.BindAndValidate[cp](c, h.logger)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	h.logger.InfoContext(ctx, "Processing password change")

	resp, err := h.client.ChangePassword(ctx, &auth_pb.ChangePasswordRequest{
		UserId:      req.UserID,
		OldPassword: req.OldPassword,
		NewPassword: req.NewPassword,
	})

	if err != nil {
		h.logger.ErrorContext(ctx, "Password change failed", "error", err)
		h.errorHandler.HandleGrpcToHttp(c, err)
		return
	}

	h.logger.InfoContext(ctx, "Password change successful")

	u.SuccessResponse(c, resp.Message, nil)
}

func (h *AuthHandler) Health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	h.logger.InfoContext(ctx, "Processing health check")

	resp, err := h.client.Health(ctx, &auth_pb.HealthRequest{})

	if err != nil {
		h.logger.ErrorContext(ctx, "Health check failed", "error", err)
		h.errorHandler.HandleGrpcToHttp(c, err)
		return
	}

	h.logger.InfoContext(ctx, "Health check successful")

	responseData := &m.HealthResponse{
		Status:    resp.Status,
		Timestamp: resp.Timestamp.Seconds,
		Checks:    resp.Checks,
	}

	u.SuccessResponse(c, "Service healthy", responseData)
}
