package server

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/connectivity"

	"remaster/services/api-gateway/handlers"
	"remaster/services/api-gateway/middleware"
)

func (s *Server) setupRoutes() {
	s.router = gin.New()

	s.router.Use(
		middleware.RequestLogger(s.Logger, s.errorHandler),
		middleware.RateLimiter(s.RedisManager.GetClient(), 100, 1*time.Minute),
		middleware.CORS(),
		middleware.GinErrorMiddleware(s.errorHandler),
		middleware.Recovery(s.Logger),
	)

	s.router.GET("/health", s.handleHealth)

	s.setupAuthRoutes()

	s.Logger.Info("Routes configured successfully")
}

func (s *Server) setupAuthRoutes() {
	auth := s.router.Group("/auth")

	authHandler := handlers.NewAuthHandler(s.authClient, s.Logger, s.errorHandler)

	auth.POST("/register", authHandler.Register)
	auth.POST("/login", authHandler.Login)

	s.Logger.Debug("Auth routes registered")
}

// Health check handlers
func (s *Server) handleHealth(c *gin.Context) {
	s.connMutex.RLock()
	defer s.connMutex.RUnlock()

	healthy := true
	services := make(map[string]string)
	details := make(map[string]ServiceHealth)

	for serviceName, conn := range s.grpcConnections {
		if conn == nil {
			services[serviceName] = "NOT_CONNECTED"
			details[serviceName] = ServiceHealth{
				Status:  "NOT_CONNECTED",
				Message: "Connection not established",
			}
			healthy = false
			continue
		}

		state := conn.GetState()
		services[serviceName] = state.String()

		serviceHealthy := state == connectivity.Ready
		if !serviceHealthy {
			healthy = false
		}

		details[serviceName] = ServiceHealth{
			Status:  state.String(),
			Healthy: serviceHealthy,
			Message: getStateMessage(state),
		}
	}

	health := HealthStatus{
		Status:    getOverallStatus(healthy),
		Healthy:   healthy,
		Services:  services,
		Details:   details,
		Timestamp: time.Now().UTC(),
		Version:   s.Config.App.Version,
	}

	status := http.StatusOK
	if !health.Healthy {
		status = http.StatusServiceUnavailable
	}

	c.JSON(status, health)
}

// Health status types
type HealthStatus struct {
	Status    string                   `json:"status"`
	Healthy   bool                     `json:"healthy"`
	Services  map[string]string        `json:"services"`
	Details   map[string]ServiceHealth `json:"details,omitempty"`
	Timestamp time.Time                `json:"timestamp"`
	Version   string                   `json:"version,omitempty"`
}

type ServiceHealth struct {
	Status  string `json:"status"`
	Healthy bool   `json:"healthy"`
	Message string `json:"message,omitempty"`
}

func getStateMessage(state connectivity.State) string {
	switch state {
	case connectivity.Idle:
		return "Connection is idle"
	case connectivity.Connecting:
		return "Connecting to service"
	case connectivity.Ready:
		return "Service is ready"
	case connectivity.TransientFailure:
		return "Temporary connection failure"
	case connectivity.Shutdown:
		return "Connection is shut down"
	default:
		return "Unknown state"
	}
}

func getOverallStatus(healthy bool) string {
	if healthy {
		return "healthy"
	}
	return "unhealthy"
}
