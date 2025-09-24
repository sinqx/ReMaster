package middleware

import (
	"remaster/shared/errors"
	auth_pb "remaster/shared/proto/auth"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
)

func RequireAuth(authClient auth_pb.AuthServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.Error(errors.NewUnauthorizedError("Missing or invalid Authorization header"))
			c.Abort()
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			c.Error(errors.NewUnauthorizedError("Token is empty"))
			c.Abort()
			return
		}

		resp, err := authClient.ValidateToken(c.Request.Context(), &auth_pb.ValidateTokenRequest{
			AccessToken: token,
		})
		if err != nil || !resp.Valid {
			c.Error(errors.NewUnauthorizedError("Invalid token"))
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
			c.Error(errors.NewUnauthorizedError("Missing user role"))
			c.Abort()
			return
		}

		roleStr, ok := userRole.(string)
		if !ok {
			c.Error(errors.NewUnauthorizedError("Invalid user role"))
			c.Abort()
			return
		}

		if slices.Contains(roles, roleStr) {
			c.Next()
			return
		}

		c.Error(errors.NewForbiddenError("Forbidden access, missing required role"))
		c.Abort()
	}
}
