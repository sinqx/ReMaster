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
		middleware.Logger(s.Logger),
		middleware.Recovery(s.Logger),
		middleware.CORS(),
		middleware.GRPCErrorHandler(s.Logger),
		// middleware.RateLimiter(s.RedisManager),
	)

	s.setupHealthEndpoints()
	s.setupAuthRoutes()

	s.Logger.Info("Routes configured successfully")
}

// Health check endpoints
func (s *Server) setupHealthEndpoints() {
	s.router.GET("/health", s.handleHealth)
	s.router.GET("/health/live", s.handleLiveness)
	s.router.GET("/health/ready", s.handleReadiness)

	s.Logger.Debug("Health endpoints configured")
}

func (s *Server) setupAuthRoutes() {
	auth := s.router.Group("/auth")

	authHandler := handlers.NewAuthHandler(s.authClient, s.Logger)

	{
		auth.POST("/register", authHandler.Register)
	}
	s.Logger.Debug("Auth routes registered")
}

// Health check handlers
func (s *Server) handleHealth(c *gin.Context) {
	health := s.checkServicesHealth()

	status := http.StatusOK
	if !health.Healthy {
		status = http.StatusServiceUnavailable
	}

	c.JSON(status, health)
}

func (s *Server) handleLiveness(c *gin.Context) {
	// Simple liveness check - is the server running?
	c.JSON(http.StatusOK, gin.H{
		"status":    "alive",
		"timestamp": time.Now().UTC(),
	})
}

func (s *Server) handleReadiness(c *gin.Context) {
	// Readiness check - are all critical services ready?
	health := s.checkServicesHealth()

	// Define critical services that must be ready
	criticalServices := []string{"auth"} // Add more as needed

	ready := true
	for _, service := range criticalServices {
		if serviceStatus, exists := health.Services[service]; exists {
			if serviceStatus != "READY" {
				ready = false
				break
			}
		} else {
			ready = false
			break
		}
	}

	status := http.StatusOK
	if !ready {
		status = http.StatusServiceUnavailable
	}

	c.JSON(status, gin.H{
		"ready":     ready,
		"services":  health.Services,
		"timestamp": time.Now().UTC(),
	})
}

// Helper function to check all services health
func (s *Server) checkServicesHealth() HealthStatus {
	s.connMutex.RLock()
	defer s.connMutex.RUnlock()

	healthy := true
	services := make(map[string]string)
	details := make(map[string]ServiceHealth)

	// Check GRPC connections
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

	// Check MongoDB
	if s.MongoManager != nil {
		// Implement MongoDB health check
		services["mongodb"] = "READY"
		details["mongodb"] = ServiceHealth{
			Status:  "READY",
			Healthy: true,
		}
	}

	// Check Redis
	if s.RedisManager != nil {
		// Implement Redis health check
		services["redis"] = "READY"
		details["redis"] = ServiceHealth{
			Status:  "READY",
			Healthy: true,
		}
	}

	return HealthStatus{
		Status:    getOverallStatus(healthy),
		Healthy:   healthy,
		Services:  services,
		Details:   details,
		Timestamp: time.Now().UTC(),
		Version:   s.Config.App.Version,
	}
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
