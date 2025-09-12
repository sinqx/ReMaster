package server

import (
	"context"
	"net/http"

	auth_pb "remaster/shared/proto/auth"

	"github.com/gin-gonic/gin"
)

func (s *Server) startHTTPServer(ctx context.Context) error {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	router.POST("/register", s.handleRegister)

	s.httpServer = &http.Server{
		Addr:         s.Config.HTTP.Host + ":" + s.Config.HTTP.Port,
		Handler:      router,
		ReadTimeout:  s.Config.HTTP.ReadTimeout,
		WriteTimeout: s.Config.HTTP.WriteTimeout,
	}

	

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

func (s *Server) handleRegister(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	resp, err := s.authClient.Registration(c, &auth_pb.RegisterRequest{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		s.Logger.Error("grpc call error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "auth service error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": resp.Message})
}
