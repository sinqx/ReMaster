package handlers

import (
	"log/slog"
	"net/http"
	"strings"

	mw "remaster/services/api-gateway/middleware"
	m "remaster/services/api-gateway/models"
	auth_pb "remaster/shared/proto/auth"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/status"
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

	switch e := status.Convert(err).Message(); {
	case strings.Contains(e, "must be"):
		c.JSON(http.StatusBadRequest, gin.H{"error": e})
	case strings.Contains(e, "already exists"):
		c.JSON(http.StatusConflict, gin.H{"error": e})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}

	if err != nil {
		h.logger.Error("1registration failed", "error", err, "email", dto.Email)
		mw.HandleGRPCError(c, err, h.logger)
		return
	}

	c.JSON(http.StatusOK, auth_pb.RegisterResponse{
		Message: resp.Message,
		Success: true,
	})
}
