package middleware

import (
	"net/http"
	"remaster/services/api-gateway/models"
	"strings"

	"github.com/gin-gonic/gin"
)

func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{
				Error:   "Authorization header required",
				Success: false,
			})
			c.Abort()
			return
		}

		// Bearer token validation
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{
				Error:   "Invalid authorization header format",
				Success: false,
			})
			c.Abort()
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{
				Error:   "Token required",
				Success: false,
			})
			c.Abort()
			return
		}

		// TODO: Validate JWT token

		c.Set("user_id", "some_user_id")
		c.Set("user_role", "student")

		c.Next()
	}
}

func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("user_role")
		if !exists {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{
				Error:   "User role not found",
				Success: false,
			})
			c.Abort()
			return
		}

		roleStr, ok := userRole.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error:   "Invalid user role type",
				Success: false,
			})
			c.Abort()
			return
		}

		for _, role := range roles {
			if roleStr == role {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error:   "Insufficient permissions",
			Success: false,
		})
		c.Abort()
	}
}