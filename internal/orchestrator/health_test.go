package orchestrator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dyluth/sett/pkg/blackboard"
	"github.com/redis/go-redis/v9"
)

// TestHealthCheckEndpoint_MethodNotAllowed verifies non-GET requests are rejected.
func TestHealthCheckEndpoint_MethodNotAllowed(t *testing.T) {
	// Create a mock client (nil is fine for this test)
	server := NewHealthServer(nil)

	req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	w := httptest.NewRecorder()

	server.healthCheckHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

// TestHealthCheckResponse verifies the JSON response structure.
func TestHealthCheckResponse(t *testing.T) {
	// Create a minimal Redis client for testing
	// (it won't connect, but we can test the structure)
	client, err := blackboard.NewClient(&redis.Options{
		Addr: "localhost:6379",
	}, "test")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	server := NewHealthServer(client)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	// Use context with timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	server.healthCheckHandler(w, req)

	// Parse response
	var response HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Since Redis is not actually running, expect unhealthy status
	if response.Status != "unhealthy" {
		t.Errorf("Expected unhealthy status (Redis not running), got %s", response.Status)
	}

	if response.Redis != "disconnected" {
		t.Errorf("Expected redis=disconnected, got %s", response.Redis)
	}

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}

	// Verify Content-Type header
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", ct)
	}
}
