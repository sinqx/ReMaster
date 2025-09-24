package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"remaster/shared/errors"
	"remaster/shared/logger"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func RequestLogger(baseLogger *slog.Logger, eh *errors.ErrorHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Get or generate correlation ID
		correlationID, _ := c.Get("correlation_id")
		if correlationID == nil {
			correlationID = uuid.New().String()
			c.Set("correlation_id", correlationID)
		}

		// Create request-specific logger
		requestLogger := logger.WithCorrelationID(baseLogger, correlationID.(string))

		// Add to context for handlers
		ctx := logger.ToContext(c.Request.Context(), requestLogger)
		c.Request = c.Request.WithContext(ctx)

		// Process request
		c.Next()

		// Calculate request metrics
		latency := time.Since(start)
		path := c.Request.URL.Path
		if raw := c.Request.URL.RawQuery; raw != "" {
			path = path + "?" + raw
		}

		// Prepare log attributes
		logAttrs := []slog.Attr{
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.Int("status", c.Writer.Status()),
			slog.String("client_ip", c.ClientIP()),
			slog.Duration("latency", latency),
			slog.Int("body_size", c.Writer.Size()),
		}

		// Handle errors if present
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			logAttrs = append(logAttrs, slog.String("error", err.Error()))

			// Use error handler to send proper response
			eh.HandleGinError(c, err)

			// Log as error
			requestLogger.LogAttrs(c.Request.Context(), slog.LevelError,
				"HTTP request completed with error", logAttrs...)
		} else {
			// Determine log level based on status
			logLevel := slog.LevelInfo
			if c.Writer.Status() >= 400 {
				logLevel = slog.LevelWarn
			}

			requestLogger.LogAttrs(c.Request.Context(), logLevel,
				"HTTP request completed", logAttrs...)
		}
	}
}

func Recovery(baseLogger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Get request logger from context
				requestLogger := logger.FromContext(c.Request.Context(), baseLogger)

				// Check for broken connection
				var brokenPipe bool
				if ne, ok := err.(*net.OpError); ok {
					if se, ok := ne.Err.(*os.SyscallError); ok {
						errStr := strings.ToLower(se.Error())
						if strings.Contains(errStr, "broken pipe") ||
							strings.Contains(errStr, "connection reset by peer") {
							brokenPipe = true
						}
					}
				}

				httpPath := c.Request.Method + " " + c.Request.URL.Path

				if brokenPipe {
					requestLogger.WarnContext(c.Request.Context(),
						"Connection broken during request",
						slog.String("path", httpPath),
						slog.Any("panic", err),
					)

					c.Error(errors.NewInternalError(
						fmt.Sprintf("Connection broken while handling %s", httpPath),
						fmt.Errorf("BROKEN_PIPE:%v", err),
					))
				} else {
					requestLogger.ErrorContext(c.Request.Context(),
						"Panic recovered during request",
						slog.String("path", httpPath),
						slog.Any("panic", err),
					)

					if !c.Writer.Written() {
						c.Error(errors.NewInternalError(
							fmt.Sprintf("Panic recovered while handling %s", httpPath),
							fmt.Errorf("%v", err),
						))
					}
				}

				c.Abort()
			}
		}()
		c.Next()
	}
}

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func GinErrorMiddleware(eh *errors.ErrorHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			eh.HandleGinError(c, err)
		}
	}
}

func RateLimiter(rdb *redis.Client, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()
		ip := c.ClientIP()
		key := "ratelimit:" + ip

		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			c.Error(errors.NewInternalError(
				"Rate limiter error",
				err,
			))
			c.Abort()
			return
		}

		if count == 1 {
			rdb.Expire(ctx, key, window)
		}

		if int(count) > limit {
			c.Error(errors.NewRateLimitError("Too many requests"))
			c.Abort()
			return
		}

		c.Next()
	}
}
