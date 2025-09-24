package utils

import (
	"log/slog"
	"net/http"
	"remaster/shared/errors"

	"github.com/gin-gonic/gin"
)

func BindAndValidate[T any](c *gin.Context, logger *slog.Logger) (*T, bool) {
	var dto T
	if err := c.ShouldBindJSON(&dto); err != nil {
		logger.WarnContext(c.Request.Context(),
			"Validation failed",
			slog.Any("validation_errors", err.Error()),
		)
		c.Error(errors.NewValidationError(
			"Request data is invalid",
			map[string]string{
				"field": "request_body",
				"issue": err.Error(),
			},
		))
		return nil, false
	}
	return &dto, true
}

func SuccessResponse(c *gin.Context, msg string, data any) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": msg,
		"data":    data,
	})
}
