package main

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPupLifecycle tests the full lifecycle of the pup binary.
// This is an integration test that:
//  1. Compiles the pup binary
//  2. Starts Redis
//  3. Runs pup as a subprocess
//  4. Verifies health check works
//  5. Sends SIGTERM
//  6. Verifies clean shutdown
func TestPupLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Build the pup binary
	binPath := buildPupBinary(t)
	defer os.Remove(binPath)

	// Start mock Redis
	mr := miniredis.RunT(t)
	defer mr.Close()

	// Set environment variables
	env := []string{
		"HOLT_INSTANCE_NAME=test-instance",
		"HOLT_AGENT_NAME=test-agent",
		"HOLT_AGENT_ROLE=test-role",
		`HOLT_AGENT_COMMAND=["/bin/sh", "-c", "echo test"]`,
		"HOLT_BIDDING_STRATEGY=exclusive", // M3.1: Required
		"REDIS_URL=redis://" + mr.Addr(),
	}

	// Start pup process
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath)
	cmd.Env = append(os.Environ(), env...)

	// Capture output for debugging
	output, err := cmd.StdoutPipe()
	require.NoError(t, err)
	errOutput, err := cmd.StderrPipe()
	require.NoError(t, err)

	// Start the process
	err = cmd.Start()
	require.NoError(t, err)
	t.Logf("Pup process started with PID: %d", cmd.Process.Pid)

	// Log output in background
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := output.Read(buf)
			if n > 0 {
				t.Logf("[STDOUT] %s", buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := errOutput.Read(buf)
			if n > 0 {
				t.Logf("[STDERR] %s", buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	// Wait for health check to become available (with retries)
	var resp *http.Response
	var healthErr error
	for i := 0; i < 20; i++ {
		resp, healthErr = http.Get("http://localhost:8080/healthz")
		if healthErr == nil {
			defer resp.Body.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	require.NoError(t, healthErr, "Health check should be accessible after startup")
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Health check should return 200")

	// Send SIGTERM to pup process
	t.Logf("Sending SIGTERM to pup process...")
	err = cmd.Process.Signal(syscall.SIGTERM)
	require.NoError(t, err)

	// Wait for process to exit with timeout
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	startTime := time.Now()
	select {
	case err := <-done:
		shutdownDuration := time.Since(startTime)
		t.Logf("Pup shutdown completed in %v", shutdownDuration)

		// Verify clean exit (exit code 0)
		assert.NoError(t, err, "Pup should exit cleanly with code 0")

		// Verify shutdown completed within 5 seconds (spec requirement)
		assert.Less(t, shutdownDuration, 5*time.Second, "Shutdown should complete within 5 seconds")

	case <-time.After(6 * time.Second):
		// Force kill if not shut down
		_ = cmd.Process.Kill()
		t.Fatal("Pup did not shut down within 6 seconds (spec requires < 5s)")
	}
}

// TestPupMissingConfig tests that pup exits with error when required config is missing.
func TestPupMissingConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Build the pup binary
	binPath := buildPupBinary(t)
	defer os.Remove(binPath)

	// Run pup without holting HOLT_INSTANCE_NAME
	cmd := exec.Command(binPath)
	cmd.Env = []string{
		// Missing HOLT_INSTANCE_NAME
		"HOLT_AGENT_NAME=test-agent",
		"REDIS_URL=redis://localhost:6379",
	}

	// Run and capture error
	err := cmd.Run()

	// Verify pup exited with non-zero code
	assert.Error(t, err, "Pup should exit with error when config is missing")

	exitErr, ok := err.(*exec.ExitError)
	require.True(t, ok, "Error should be ExitError")
	assert.NotEqual(t, 0, exitErr.ExitCode(), "Exit code should be non-zero")
}

// TestPupInvalidRedisURL tests that pup exits with error when Redis URL is invalid.
func TestPupInvalidRedisURL(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Build the pup binary
	binPath := buildPupBinary(t)
	defer os.Remove(binPath)

	// Set environment with invalid Redis URL
	env := []string{
		"HOLT_INSTANCE_NAME=test-instance",
		"HOLT_AGENT_NAME=test-agent",
		"REDIS_URL=not-a-valid-url",
	}

	cmd := exec.Command(binPath)
	cmd.Env = append(os.Environ(), env...)

	// Run and capture error
	err := cmd.Run()

	// Verify pup exited with non-zero code
	assert.Error(t, err, "Pup should exit with error when Redis URL is invalid")

	exitErr, ok := err.(*exec.ExitError)
	require.True(t, ok, "Error should be ExitError")
	assert.NotEqual(t, 0, exitErr.ExitCode(), "Exit code should be non-zero")
}

// TestPupRedisUnavailable tests that pup exits when Redis is not available.
func TestPupRedisUnavailable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Build the pup binary
	binPath := buildPupBinary(t)
	defer os.Remove(binPath)

	// Set environment with Redis URL pointing to non-existent Redis
	env := []string{
		"HOLT_INSTANCE_NAME=test-instance",
		"HOLT_AGENT_NAME=test-agent",
		"REDIS_URL=redis://localhost:16379", // Non-existent Redis
	}

	cmd := exec.Command(binPath)
	cmd.Env = append(os.Environ(), env...)

	// Set a timeout for the command
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd = exec.CommandContext(ctx, binPath)
	cmd.Env = append(os.Environ(), env...)

	// Run and capture error
	err := cmd.Run()

	// Verify pup exited with non-zero code (Redis connection failed)
	assert.Error(t, err, "Pup should exit with error when Redis is unavailable")
}

// TestPupSIGINT tests that pup responds to SIGINT (Ctrl+C) signal.
func TestPupSIGINT(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Build the pup binary
	binPath := buildPupBinary(t)
	defer os.Remove(binPath)

	// Start mock Redis
	mr := miniredis.RunT(t)
	defer mr.Close()

	// Set environment variables
	env := []string{
		"HOLT_INSTANCE_NAME=test-instance",
		"HOLT_AGENT_NAME=test-agent",
		"HOLT_AGENT_ROLE=test-role",
		`HOLT_AGENT_COMMAND=["/bin/sh", "-c", "echo test"]`,
		"HOLT_BIDDING_STRATEGY=exclusive", // M3.1: Required
		"REDIS_URL=redis://" + mr.Addr(),
	}

	// Start pup process
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath)
	cmd.Env = append(os.Environ(), env...)

	err := cmd.Start()
	require.NoError(t, err)

	// Wait for health check to become available (with retries)
	var healthResp *http.Response
	var healthErr error
	for i := 0; i < 20; i++ {
		healthResp, healthErr = http.Get("http://localhost:8080/healthz")
		if healthErr == nil {
			healthResp.Body.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	require.NoError(t, healthErr, "Health check should be accessible before sending SIGINT")

	// Send SIGINT (Ctrl+C) to pup process
	t.Logf("Sending SIGINT to pup process...")
	err = cmd.Process.Signal(syscall.SIGINT)
	require.NoError(t, err)

	// Wait for process to exit
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		// Verify clean exit
		assert.NoError(t, err, "Pup should exit cleanly after SIGINT")

	case <-time.After(6 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("Pup did not shut down after SIGINT within timeout")
	}
}

// buildPupBinary compiles the pup binary and returns the path to it.
func buildPupBinary(t *testing.T) string {
	t.Helper()

	// Create temporary binary path
	binPath := t.TempDir() + "/pup"

	// Build the binary
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	require.NoError(t, err, "Failed to build pup binary")

	t.Logf("Built pup binary at: %s", binPath)
	return binPath
}
