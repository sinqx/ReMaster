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

func NewAuthHandler(client auth_pb.AuthServiceClient, logger *slog.Logger, errorHandler *errors.ErrorHandler) *AuthHandler {
	return &AuthHandler{
		client:       client,
		errorHandler: errorHandler,
		logger:       logger.With(slog.String("api-gateway", "auth")),
	}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var dto m.RegisterDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		valErr := errors.NewValidationError("Invalid registration data", map[string]string{"details": err.Error()})
		h.logger.Warn("Registration validation failed", "error", valErr)
		c.Error(valErr)
		return
	}

	h.logger.Info("Processing registration", "email", dto.Email, "user_type", dto.UserType)

	resp, err := h.client.Registration(c, &auth_pb.RegisterRequest{
		Email:     dto.Email,
		Password:  dto.Password,
		FirstName: dto.FirstName,
		LastName:  dto.LastName,
		Phone:     dto.Phone,
		UserType:  dto.UserType,
	})
	if err != nil {
		h.logger.Error("gRPC registration failed", "error", err, "email", dto.Email)
		if h.errorHandler == nil {
			h.logger.Error("Error handler is nil - fallback")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Internal server error", "code": "INTERNAL_ERROR"})
			return
		}
		h.errorHandler.HandleGrpcToHttp(c, err)
		return
	}

	h.logger.Info("Registration successful", "user_id", resp.UserId, "email", dto.Email)

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

// func NewAuthHandler(client auth_pb.AuthServiceClient, logger *slog.Logger, errorHandler *errors.ErrorHandler) *AuthHandler {
// 	return &AuthHandler{
// 		client:       client,
// 		errorHandler: errorHandler,
// 		logger:       logger.With(slog.String("api-gateway", "auth-handler")),
// 	}
// }

// func (h *AuthHandler) Register(c *gin.Context) {
// 	requestLogger := logger.FromContext(c.Request.Context(), nil)

// 	var dto m.RegisterDTO
// 	if err := c.ShouldBindJSON(&dto); err != nil {
// 		requestLogger.WarnContext(c.Request.Context(),
// 			"Registration validation failed",
// 			slog.Any("validation_errors", err.Error()),
// 		)

// 		c.Error(errors.NewValidationError(
// 			"Registration data is invalid",
// 			map[string]string{
// 				"field": "request_body",
// 				"issue": err.Error(),
// 			},
// 		))
// 		return
// 	}

// 	requestLogger.InfoContext(c.Request.Context(),
// 		"Processing user registration",
// 		slog.String("email", dto.Email),
// 		slog.String("user_type", dto.UserType),
// 		slog.String("phone", dto.Phone),
// 	)

// 	resp, err := h.client.Registration(c, &auth_pb.RegisterRequest{
// 		Email:     dto.Email,
// 		Password:  dto.Password,
// 		FirstName: dto.FirstName,
// 		LastName:  dto.LastName,
// 		Phone:     dto.Phone,
// 		UserType:  dto.UserType,
// 	})

// 	if err != nil {
// 		requestLogger.Error(
// 			"gRPC registration failed",
// 			"error", err,
// 			"email", dto.Email)
// 		if h.errorHandler == nil {
// 			requestLogger.Error("Error handler is nil - fallback")
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal error"})
// 			return
// 		}
// 		h.errorHandler.HandleGrpcToHttp(c, err)
// 		return
// 	}

// 	requestLogger.InfoContext(c.Request.Context(),
// 		"User registration successful",
// 		slog.String("user_id", resp.UserId),
// 		slog.String("email", dto.Email),
// 	)

// 	c.JSON(http.StatusOK, m.SuccessResponse{
// 		Success: true,
// 		Message: resp.Message,
// 		Data: m.RegisterResponse{
// 			UserID:      resp.UserId,
// 			AccessToken: resp.AccessToken,
// 			ExpiresAt:   resp.ExpiresAt,
// 		},
// 	})
// }
