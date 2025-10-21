package pup

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/dyluth/holt/pkg/blackboard"
)

// HealthServer provides an HTTP health check endpoint for the agent pup.
// The health check verifies Redis connectivity via the blackboard client.
// The server runs in a background goroutine and can be gracefully shut down.
type HealthServer struct {
	server   *http.Server
	bbClient *blackboard.Client
}

// HealthResponse represents the JSON response from the /healthz endpoint.
type HealthResponse struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// NewHealthServer creates a new health check HTTP server.
// The server listens on all interfaces (0.0.0.0) at the specified port.
// This is required for Docker container networking.
//
// Parameters:
//   - bbClient: Blackboard client used to verify Redis connectivity
//   - port: Port number to listen on (typically 8080)
//
// Returns a configured HealthServer ready to be started.
func NewHealthServer(bbClient *blackboard.Client, port int) *HealthServer {
	mux := http.NewServeMux()
	hs := &HealthServer{
		server: &http.Server{
			Addr:         fmt.Sprintf(":%d", port),
			Handler:      mux,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
		},
		bbClient: bbClient,
	}

	// Register health check handler
	mux.HandleFunc("/healthz", hs.handleHealthz)

	return hs
}

// Start starts the HTTP server in a background goroutine.
// Returns immediately after the server starts listening.
// Returns an error if the server fails to start (e.g., port already in use).
//
// The server continues running until Shutdown() is called or an error occurs.
// Server errors are logged but do not crash the pup process.
func (hs *HealthServer) Start() error {
	// Start server in background goroutine
	go func() {
		log.Printf("[DEBUG] Health server starting on %s", hs.server.Addr)
		if err := hs.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[ERROR] Health server error: %v", err)
		}
		log.Printf("[DEBUG] Health server stopped")
	}()

	return nil
}

// Shutdown gracefully shuts down the HTTP server with a timeout.
// Waits for in-flight requests to complete before returning.
//
// The provided context controls the shutdown timeout. If the context
// expires before shutdown completes, the server is forcefully closed.
//
// Returns an error if shutdown fails or times out.
func (hs *HealthServer) Shutdown(ctx context.Context) error {
	log.Printf("[DEBUG] Shutting down health server...")
	return hs.server.Shutdown(ctx)
}

// handleHealthz handles HTTP GET requests to /healthz.
// Returns 200 OK if Redis is reachable, 503 Service Unavailable otherwise.
//
// Response format:
//   - Success: {"status": "healthy"}
//   - Failure: {"status": "unhealthy", "error": "connection failed"}
func (hs *HealthServer) handleHealthz(w http.ResponseWriter, r *http.Request) {
	// Create context with 5-second timeout for Redis PING
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Ping Redis via blackboard client
	err := hs.bbClient.Ping(ctx)

	var response HealthResponse
	var statusCode int

	if err != nil {
		// Redis unreachable - unhealthy
		response = HealthResponse{
			Status: "unhealthy",
			Error:  err.Error(),
		}
		statusCode = http.StatusServiceUnavailable
	} else {
		// Redis reachable - healthy
		response = HealthResponse{
			Status: "healthy",
		}
		statusCode = http.StatusOK
	}

	// Write JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("[ERROR] Failed to encode health response: %v", err)
	}
}
