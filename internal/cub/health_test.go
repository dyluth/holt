package cub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/dyluth/sett/pkg/blackboard"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewHealthServer verifies that NewHealthServer creates a properly configured server.
func TestNewHealthServer(t *testing.T) {
	// Setup mock Redis
	mr := miniredis.RunT(t)
	defer mr.Close()

	client, err := blackboard.NewClient(&redis.Options{Addr: mr.Addr()}, "test-instance")
	require.NoError(t, err)
	defer client.Close()

	// Create health server
	hs := NewHealthServer(client, 8080)

	// Verify server is configured
	assert.NotNil(t, hs)
	assert.NotNil(t, hs.server)
	assert.NotNil(t, hs.bbClient)
	assert.Equal(t, ":8080", hs.server.Addr)
}

// TestHealthzHandler_Success tests the /healthz endpoint when Redis is available.
func TestHealthzHandler_Success(t *testing.T) {
	// Setup mock Redis
	mr := miniredis.RunT(t)
	defer mr.Close()

	client, err := blackboard.NewClient(&redis.Options{Addr: mr.Addr()}, "test-instance")
	require.NoError(t, err)
	defer client.Close()

	// Create health server
	hs := NewHealthServer(client, 8080)

	// Create test HTTP request
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	// Call handler directly
	hs.handleHealthz(rec, req)

	// Verify response
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	// Parse JSON response
	var response HealthResponse
	err = json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "healthy", response.Status)
	assert.Empty(t, response.Error)
}

// TestHealthzHandler_RedisUnavailable tests the /healthz endpoint when Redis is down.
func TestHealthzHandler_RedisUnavailable(t *testing.T) {
	// Create client with invalid Redis address (no server running)
	client, err := blackboard.NewClient(&redis.Options{
		Addr:        "localhost:16379", // Non-existent Redis
		DialTimeout: 100 * time.Millisecond,
	}, "test-instance")
	require.NoError(t, err)
	defer client.Close()

	// Create health server
	hs := NewHealthServer(client, 8080)

	// Create test HTTP request
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	// Call handler directly
	hs.handleHealthz(rec, req)

	// Verify response
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	// Parse JSON response
	var response HealthResponse
	err = json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "unhealthy", response.Status)
	assert.NotEmpty(t, response.Error) // Should contain error message
}

// TestHealthzHandler_RedisConnectionLost tests when Redis connection is lost during operation.
func TestHealthzHandler_RedisConnectionLost(t *testing.T) {
	// Setup mock Redis
	mr := miniredis.RunT(t)

	client, err := blackboard.NewClient(&redis.Options{Addr: mr.Addr()}, "test-instance")
	require.NoError(t, err)
	defer client.Close()

	// Create health server
	hs := NewHealthServer(client, 8080)

	// First request - Redis is up
	req1 := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec1 := httptest.NewRecorder()
	hs.handleHealthz(rec1, req1)
	assert.Equal(t, http.StatusOK, rec1.Code)

	// Stop Redis
	mr.Close()

	// Second request - Redis is down
	req2 := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec2 := httptest.NewRecorder()
	hs.handleHealthz(rec2, req2)
	assert.Equal(t, http.StatusServiceUnavailable, rec2.Code)

	// Parse response
	var response HealthResponse
	err = json.NewDecoder(rec2.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "unhealthy", response.Status)
}

// TestHealthServer_StartAndShutdown tests the full lifecycle of the health server.
func TestHealthServer_StartAndShutdown(t *testing.T) {
	// Setup mock Redis
	mr := miniredis.RunT(t)
	defer mr.Close()

	client, err := blackboard.NewClient(&redis.Options{Addr: mr.Addr()}, "test-instance")
	require.NoError(t, err)
	defer client.Close()

	// Create health server on a random available port
	hs := NewHealthServer(client, 0) // Port 0 = OS assigns random port

	// For testing, we need to get the actual port, but since we can't easily do that
	// with the current implementation, let's use a fixed test port
	hs.server.Addr = "localhost:18080"

	// Start server
	err = hs.Start()
	require.NoError(t, err)

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Make HTTP request to health endpoint
	resp, err := http.Get("http://localhost:18080/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Shutdown server with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = hs.Shutdown(ctx)
	assert.NoError(t, err)

	// Verify server is no longer accepting requests
	time.Sleep(50 * time.Millisecond)
	_, err = http.Get("http://localhost:18080/healthz")
	assert.Error(t, err) // Should fail because server is shut down
}

// TestHealthServer_ShutdownTimeout tests shutdown behavior when timeout is exceeded.
func TestHealthServer_ShutdownTimeout(t *testing.T) {
	// Setup mock Redis
	mr := miniredis.RunT(t)
	defer mr.Close()

	client, err := blackboard.NewClient(&redis.Options{Addr: mr.Addr()}, "test-instance")
	require.NoError(t, err)
	defer client.Close()

	// Create health server
	hs := NewHealthServer(client, 0)
	hs.server.Addr = "localhost:18081"

	// Start server
	err = hs.Start()
	require.NoError(t, err)

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Shutdown with very short timeout (almost immediate)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Shutdown should complete (may return context deadline exceeded, but that's okay)
	_ = hs.Shutdown(ctx)

	// Server should be stopped (wait a bit for cleanup)
	time.Sleep(50 * time.Millisecond)
}

// TestHealthServer_MultipleShutdowns tests that calling Shutdown multiple times is safe.
func TestHealthServer_MultipleShutdowns(t *testing.T) {
	// Setup mock Redis
	mr := miniredis.RunT(t)
	defer mr.Close()

	client, err := blackboard.NewClient(&redis.Options{Addr: mr.Addr()}, "test-instance")
	require.NoError(t, err)
	defer client.Close()

	// Create health server
	hs := NewHealthServer(client, 0)
	hs.server.Addr = "localhost:18082"

	// Start server
	err = hs.Start()
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	// First shutdown
	ctx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel1()
	err1 := hs.Shutdown(ctx1)
	assert.NoError(t, err1)

	// Second shutdown (should not panic or cause issues)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	err2 := hs.Shutdown(ctx2)
	// May return ErrServerClosed, which is expected and acceptable
	if err2 != nil {
		assert.Equal(t, http.ErrServerClosed, err2)
	}
}

// TestHealthzHandler_ContextTimeout tests that the handler respects context timeouts.
func TestHealthzHandler_ContextTimeout(t *testing.T) {
	// This test is more conceptual - in practice, the 5-second timeout in handleHealthz
	// should be sufficient for Redis PING operations.
	// We're just verifying the handler doesn't panic with context cancellation.

	// Setup mock Redis
	mr := miniredis.RunT(t)
	defer mr.Close()

	client, err := blackboard.NewClient(&redis.Options{Addr: mr.Addr()}, "test-instance")
	require.NoError(t, err)
	defer client.Close()

	// Create health server
	hs := NewHealthServer(client, 8080)

	// Create test HTTP request with cancelled context
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	ctx, cancel := context.WithCancel(req.Context())
	cancel() // Cancel immediately
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	// Call handler - should not panic
	hs.handleHealthz(rec, req)

	// Should get a response (likely unhealthy due to context cancellation)
	assert.NotEqual(t, 0, rec.Code)
}
