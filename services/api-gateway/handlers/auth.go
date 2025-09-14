package handlers

import (
	"log/slog"
	"net/http"

	mw "remaster/services/api-gateway/middleware"
	models "remaster/services/api-gateway/models"
	auth_pb "remaster/shared/proto/auth"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	client auth_pb.AuthServiceClient
	logger *slog.Logger
}

func NewAuthHandler(client auth_pb.AuthServiceClient, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{
		client: client,
		logger: logger.With(slog.String("handler", "auth")),
	}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req auth_pb.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   err.Error(),
			Success: false,
		})
		return
	}

	resp, err := h.client.Registration(c, &auth_pb.RegisterRequest{
		Email:     req.Email,
		Password:  req.Password,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		UserType:  req.UserType,
	})

	if err != nil {
		h.logger.Error("registration failed", "error", err, "email", req.Email)
		mw.HandleGRPCError(c, err, h.logger)
		return
	}

	c.JSON(http.StatusOK, auth_pb.RegisterResponse{
		Message: resp.Message,
		Success: true,
	})
}
