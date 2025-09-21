package handlers

import (
	"log/slog"
	"net/http"

	m "remaster/services/api-gateway/models"
	"remaster/shared/errors"
	auth_pb "remaster/shared/proto/auth"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	client       auth_pb.AuthServiceClient
	errorHandler *errors.ErrorHandler
	logger       *slog.Logger
}

func NewAuthHandler(client auth_pb.AuthServiceClient, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{
		client:       client,
		logger:       logger.With(slog.String("api-gateway", "auth")),
		errorHandler: errors.NewErrorHandler(logger),
	}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var dto m.RegisterDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.client.Registration(c, &auth_pb.RegisterRequest{
		Email:     dto.Email,
		Password:  dto.Password,
		FirstName: dto.FirstName,
		LastName:  dto.LastName,
		Phone:     dto.Phone,
		UserType:  dto.UserType,
	})
	if err != nil {
		h.logger.Error("registration failed", "error", err, "email", dto.Email)
		h.errorHandler.HandleGrpcToHttp(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": resp.Message,
		"data": map[string]any{
			"user_id":      resp.UserId,
			"access_token": resp.AccessToken,
			"expires_at":   resp.ExpiresAt,
		},
	})
}
