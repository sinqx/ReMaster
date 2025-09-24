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
