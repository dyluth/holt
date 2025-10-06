package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/dyluth/sett/pkg/blackboard"
)

// HealthServer provides HTTP health check endpoints for the orchestrator.
type HealthServer struct {
	client *blackboard.Client
	server *http.Server
}

// NewHealthServer creates a new health check server.
func NewHealthServer(client *blackboard.Client) *HealthServer {
	return &HealthServer{
		client: client,
	}
}

// Start starts the HTTP health check server on port 8080.
func (h *HealthServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", h.healthCheckHandler)

	h.server = &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	// Start server in background
	go func() {
		if err := h.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Health server error: %v\n", err)
		}
	}()

	return nil
}

// Shutdown gracefully shuts down the health check server.
func (h *HealthServer) Shutdown(ctx context.Context) error {
	if h.server == nil {
		return nil
	}
	return h.server.Shutdown(ctx)
}

// healthCheckHandler handles GET /healthz requests.
// Returns 200 OK if Redis is accessible, 503 Service Unavailable otherwise.
func (h *HealthServer) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check Redis connectivity with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	response := HealthResponse{
		Status: "healthy",
	}

	err := h.client.Ping(ctx)
	if err != nil {
		response.Status = "unhealthy"
		response.Redis = "disconnected"
		response.Error = err.Error()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(response)
		return
	}

	response.Redis = "connected"

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// HealthResponse is the JSON response structure for health checks.
type HealthResponse struct {
	Status string `json:"status"`
	Redis  string `json:"redis,omitempty"`
	Error  string `json:"error,omitempty"`
}
