package server

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/connectivity"

	"remaster/services/api-gateway/handlers"
	"remaster/services/api-gateway/middleware"
)

func (s *Server) startHTTPServer(ctx context.Context) error {
	router := gin.New()

	// Global middleware
	router.Use(
		middleware.Logger(s.Logger),
		middleware.Recovery(s.Logger),
		middleware.CORS(),
		middleware.GRPCErrorHandler(s.Logger),
	)

	router.GET("/health", s.handleHealth)
	v1 := router.Group("/api/v1")
	authHandler := handlers.NewAuthHandler(s.authClient, s.Logger)
	s.registerAuthRoutes(v1, authHandler)

	s.httpServer = &http.Server{
		Addr:         s.Config.HTTP.Host + ":" + s.Config.HTTP.Port,
		Handler:      router,
		ReadTimeout:  s.Config.HTTP.ReadTimeout,
		WriteTimeout: s.Config.HTTP.WriteTimeout,
	}

	return s.runServer(ctx)
}

func (s *Server) runServer(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		s.Logger.Info("Stopping HTTP server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.Config.HTTP.ShutdownTimeout)
		defer cancel()

		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			s.Logger.Error("HTTP server shutdown error", "error", err)
		}
	}()

	s.Logger.Info("HTTP server starting",
		"host", s.Config.HTTP.Host,
		"port", s.Config.HTTP.Port,
	)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.Logger.Error("failed to start HTTP server", "error", err)
		return err
	}

	return nil
}

func (s *Server) handleHealth(c *gin.Context) {
	healthy := true
	services := make(map[string]string)

	for serviceName, conn := range s.grpcConnections {
		state := conn.GetState()
		services[serviceName] = state.String()
		if state != connectivity.Ready {
			healthy = false
		}
	}

	status := "healthy"
	httpStatus := http.StatusOK
	if !healthy {
		status = "unhealthy"
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, gin.H{
		"status":    status,
		"services":  services,
		"timestamp": time.Now().UTC(),
	})
}

func (s *Server) registerAuthRoutes(rg *gin.RouterGroup, h *handlers.AuthHandler) {
	auth := rg.Group("/auth")
	{
		auth.POST("/register", h.Register)

	}
}
