package middleware

import (
	"slices"
	"net/http"
	"remaster/services/api-gateway/models"
	auth_pb "remaster/shared/proto/auth"
	"strings"

	"github.com/gin-gonic/gin"
)

func RequireAuth(authClient auth_pb.AuthServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, models.Envelope{
				Success: false,
				Message: "Missing or invalid Authorization header",
				Code:    "UNAUTHORIZED",
			})
			c.Abort()
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			c.JSON(http.StatusUnauthorized, models.Envelope{
				Success: false,
				Message: "Token required",
				Code:    "UNAUTHORIZED",
			})
			c.Abort()
			return
		}

		resp, err := authClient.ValidateToken(c.Request.Context(), &auth_pb.ValidateTokenRequest{
			AccessToken: token,
		})
		if err != nil || !resp.Valid {
			c.JSON(http.StatusUnauthorized, models.Envelope{
				Success: false,
				Message: "Invalid or expired token",
				Code:    "UNAUTHORIZED",
			})
			c.Abort()
			return
		}

		c.Set("user_id", resp.UserId)

		c.Next()
	}
}
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("user_role")
		if !exists {
			c.JSON(http.StatusUnauthorized, models.Envelope{
				Success: false,
				Message: "User role not found",
				Code:    "UNAUTHORIZED",
			})
			c.Abort()
			return
		}

		roleStr, ok := userRole.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, models.Envelope{
				Success: false,
				Message: "Invalid role type",
				Code:    "INTERNAL_ERROR",
			})
			c.Abort()
			return
		}

		if slices.Contains(roles, roleStr) {
				c.Next()
				return
			}

		c.JSON(http.StatusForbidden, models.Envelope{
			Success: false,
			Message: "Insufficient permissions",
			Code:    "FORBIDDEN",
		})
		c.Abort()
	}
}
