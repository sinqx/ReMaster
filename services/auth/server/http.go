package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	auth_pb "remaster/shared/proto/auth"

	"github.com/gin-gonic/gin"
)

func (s *Server) startHTTPServer(ctx context.Context) error {
	if s.Config.App.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	router.POST("/register", s.handleHTTPRegister)
	router.GET("/health", func(c *gin.Context) {
		mongoErr := s.MongoManager.HealthCheck(ctx)
		redisErr := s.RedisManager.HealthCheck(ctx)

		status := "healthy"
		httpStatus := http.StatusOK

		checks := map[string]string{
			"mongodb": "ok",
			"redis":   "ok",
		}

		if mongoErr != nil {
			checks["mongodb"] = mongoErr.Error()
			status = "unhealthy"
			httpStatus = http.StatusServiceUnavailable
		}

		if redisErr != nil {
			checks["redis"] = redisErr.Error()
			status = "unhealthy"
			httpStatus = http.StatusServiceUnavailable
		}

		c.JSON(httpStatus, gin.H{
			"status":    status,
			"service":   "auth-service",
			"version":   s.Config.App.Version,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"checks":    checks,
		})
	})

	// Metrics endpoint (Prometheus)
	router.GET("/metrics", func(c *gin.Context) {
		c.String(http.StatusOK, "# Metrics endpoint - TODO: implement Prometheus metrics")
	})

	// Ready check (Kubernetes)
	router.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ready",
			"service": "auth-service",
		})
	})

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%s", s.Config.HTTP.Host, s.Config.HTTP.Port),
		Handler:      router,
		ReadTimeout:  s.Config.HTTP.ReadTimeout,
		WriteTimeout: s.Config.HTTP.WriteTimeout,
	}

	// Graceful shutdown
	go func() {
		<-ctx.Done()
		s.Logger.Info("Stopping HTTP server...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.Config.HTTP.ShutdownTimeout)
		defer cancel()

		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			s.Logger.Error("HTTP server shutdown error", "error", err)
		}
	}()

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.Logger.Error("failed to start HTTP server", "error", err)
		return err
	}

	return nil
}

func (s *Server) handleHTTPRegister(c *gin.Context) {
    var req struct {
        Email    string `json:"email"`
        Password string `json:"password"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
        return
    }

    resp, err := s.authHandler.Registration(c, &auth_pb.RegisterRequest{
        Email:    req.Email,
        Password: req.Password,
    })
    if err != nil {
        s.Logger.Error("registration error", "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "registration failed"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": resp.Message})
}