package pup

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"sync/atomic"
	"time"

	"github.com/dyluth/holt/internal/config"
	"github.com/dyluth/holt/pkg/blackboard"
)

// HealthServer provides an HTTP health check endpoint for the agent pup.
// M3.9: Supports both Redis PING (default) and custom health checks.
// The server runs in a background goroutine and can be gracefully shut down.
type HealthServer struct {
	server        *http.Server
	bbClient      *blackboard.Client
	healthChecker *HealthChecker // M3.9: Optional custom health checker
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
// M3.9: Uses custom health checker if configured, falls back to Redis PING.
// Returns 200 OK if healthy, 503 Service Unavailable otherwise.
//
// Response format:
//   - Success: {"status": "healthy"}
//   - Failure: {"status": "unhealthy", "error": "connection failed"}
func (hs *HealthServer) handleHealthz(w http.ResponseWriter, r *http.Request) {
	var healthy bool
	var err error

	// M3.9: Use custom health check if configured
	if hs.healthChecker != nil {
		healthy = hs.healthChecker.IsHealthy()
		if !healthy {
			err = fmt.Errorf("custom health check failed")
		}
	} else {
		// Backward compatible: Redis PING
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		err = hs.bbClient.Ping(ctx)
		healthy = (err == nil)
	}

	var response HealthResponse
	var statusCode int

	if !healthy {
		response = HealthResponse{
			Status: "unhealthy",
			Error:  err.Error(),
		}
		statusCode = http.StatusServiceUnavailable
	} else {
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

// HealthChecker executes custom health check commands periodically (M3.9).
// Runs in background goroutine with configurable interval and timeout.
type HealthChecker struct {
	config       *config.HealthCheckConfig
	lastResult   atomic.Value  // stores bool: true = healthy, false = unhealthy
	checkRunning atomic.Bool
	workspace    string
	env          []string
	stopChan     chan struct{}
}

// NewHealthChecker creates a new health checker with the given configuration.
// workspace and env are used to run the health check command in the agent's context.
func NewHealthChecker(cfg *config.HealthCheckConfig, workspace string, env []string) *HealthChecker {
	hc := &HealthChecker{
		config:    cfg,
		workspace: workspace,
		env:       env,
		stopChan:  make(chan struct{}),
	}

	// Initialize as healthy (fail-open until first check)
	hc.lastResult.Store(true)

	return hc
}

// Start begins periodic health check execution in a background goroutine.
func (hc *HealthChecker) Start() {
	// Apply default values
	interval := "30s"
	if hc.config.Interval != "" {
		interval = hc.config.Interval
	}

	intervalDuration, err := time.ParseDuration(interval)
	if err != nil {
		log.Printf("[ERROR] Invalid health check interval '%s', using 30s", interval)
		intervalDuration = 30 * time.Second
	}

	ticker := time.NewTicker(intervalDuration)

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				hc.runCheck()
			case <-hc.stopChan:
				return
			}
		}
	}()

	log.Printf("[INFO] Health checker started (interval: %s)", interval)
}

// Stop stops the health checker background goroutine.
func (hc *HealthChecker) Stop() {
	close(hc.stopChan)
}

// runCheck executes the health check command and updates the result.
// Skips if a previous check is still running (non-blocking).
func (hc *HealthChecker) runCheck() {
	// Skip if previous check still running
	if !hc.checkRunning.CompareAndSwap(false, true) {
		log.Printf("[WARN] Skipping health check - previous check still running")
		return
	}
	defer hc.checkRunning.Store(false)

	// Apply default timeout
	timeout := "5s"
	if hc.config.Timeout != "" {
		timeout = hc.config.Timeout
	}

	timeoutDuration, err := time.ParseDuration(timeout)
	if err != nil {
		log.Printf("[ERROR] Invalid health check timeout '%s', using 5s", timeout)
		timeoutDuration = 5 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
	defer cancel()

	cmd := exec.CommandContext(ctx, hc.config.Command[0], hc.config.Command[1:]...)
	cmd.Dir = hc.workspace
	cmd.Env = hc.env

	err = cmd.Run()
	healthy := err == nil

	hc.lastResult.Store(healthy)

	if !healthy {
		log.Printf("[WARN] Health check failed: %v", err)
	}
}

// IsHealthy returns the result of the last health check.
// Returns true if no checks have run yet (fail-open).
func (hc *HealthChecker) IsHealthy() bool {
	result := hc.lastResult.Load()
	if result == nil {
		return true // No checks run yet, assume healthy
	}
	return result.(bool)
}
