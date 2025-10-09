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

// TestCubLifecycle tests the full lifecycle of the cub binary.
// This is an integration test that:
//  1. Compiles the cub binary
//  2. Starts Redis
//  3. Runs cub as a subprocess
//  4. Verifies health check works
//  5. Sends SIGTERM
//  6. Verifies clean shutdown
func TestCubLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Build the cub binary
	binPath := buildCubBinary(t)
	defer os.Remove(binPath)

	// Start mock Redis
	mr := miniredis.RunT(t)
	defer mr.Close()

	// Set environment variables
	env := []string{
		"SETT_INSTANCE_NAME=test-instance",
		"SETT_AGENT_NAME=test-agent",
		"REDIS_URL=redis://" + mr.Addr(),
	}

	// Start cub process
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
	t.Logf("Cub process started with PID: %d", cmd.Process.Pid)

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

	// Give cub time to start
	time.Sleep(500 * time.Millisecond)

	// Verify health check works
	resp, err := http.Get("http://localhost:8080/healthz")
	require.NoError(t, err, "Health check should be accessible")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Health check should return 200")

	// Send SIGTERM to cub process
	t.Logf("Sending SIGTERM to cub process...")
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
		t.Logf("Cub shutdown completed in %v", shutdownDuration)

		// Verify clean exit (exit code 0)
		assert.NoError(t, err, "Cub should exit cleanly with code 0")

		// Verify shutdown completed within 5 seconds (spec requirement)
		assert.Less(t, shutdownDuration, 5*time.Second, "Shutdown should complete within 5 seconds")

	case <-time.After(6 * time.Second):
		// Force kill if not shut down
		_ = cmd.Process.Kill()
		t.Fatal("Cub did not shut down within 6 seconds (spec requires < 5s)")
	}
}

// TestCubMissingConfig tests that cub exits with error when required config is missing.
func TestCubMissingConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Build the cub binary
	binPath := buildCubBinary(t)
	defer os.Remove(binPath)

	// Run cub without setting SETT_INSTANCE_NAME
	cmd := exec.Command(binPath)
	cmd.Env = []string{
		// Missing SETT_INSTANCE_NAME
		"SETT_AGENT_NAME=test-agent",
		"REDIS_URL=redis://localhost:6379",
	}

	// Run and capture error
	err := cmd.Run()

	// Verify cub exited with non-zero code
	assert.Error(t, err, "Cub should exit with error when config is missing")

	exitErr, ok := err.(*exec.ExitError)
	require.True(t, ok, "Error should be ExitError")
	assert.NotEqual(t, 0, exitErr.ExitCode(), "Exit code should be non-zero")
}

// TestCubInvalidRedisURL tests that cub exits with error when Redis URL is invalid.
func TestCubInvalidRedisURL(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Build the cub binary
	binPath := buildCubBinary(t)
	defer os.Remove(binPath)

	// Set environment with invalid Redis URL
	env := []string{
		"SETT_INSTANCE_NAME=test-instance",
		"SETT_AGENT_NAME=test-agent",
		"REDIS_URL=not-a-valid-url",
	}

	cmd := exec.Command(binPath)
	cmd.Env = append(os.Environ(), env...)

	// Run and capture error
	err := cmd.Run()

	// Verify cub exited with non-zero code
	assert.Error(t, err, "Cub should exit with error when Redis URL is invalid")

	exitErr, ok := err.(*exec.ExitError)
	require.True(t, ok, "Error should be ExitError")
	assert.NotEqual(t, 0, exitErr.ExitCode(), "Exit code should be non-zero")
}

// TestCubRedisUnavailable tests that cub exits when Redis is not available.
func TestCubRedisUnavailable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Build the cub binary
	binPath := buildCubBinary(t)
	defer os.Remove(binPath)

	// Set environment with Redis URL pointing to non-existent Redis
	env := []string{
		"SETT_INSTANCE_NAME=test-instance",
		"SETT_AGENT_NAME=test-agent",
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

	// Verify cub exited with non-zero code (Redis connection failed)
	assert.Error(t, err, "Cub should exit with error when Redis is unavailable")
}

// TestCubSIGINT tests that cub responds to SIGINT (Ctrl+C) signal.
func TestCubSIGINT(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Build the cub binary
	binPath := buildCubBinary(t)
	defer os.Remove(binPath)

	// Start mock Redis
	mr := miniredis.RunT(t)
	defer mr.Close()

	// Set environment variables
	env := []string{
		"SETT_INSTANCE_NAME=test-instance",
		"SETT_AGENT_NAME=test-agent",
		"REDIS_URL=redis://" + mr.Addr(),
	}

	// Start cub process
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath)
	cmd.Env = append(os.Environ(), env...)

	err := cmd.Start()
	require.NoError(t, err)

	// Give cub time to start
	time.Sleep(500 * time.Millisecond)

	// Send SIGINT (Ctrl+C) to cub process
	t.Logf("Sending SIGINT to cub process...")
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
		assert.NoError(t, err, "Cub should exit cleanly after SIGINT")

	case <-time.After(6 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("Cub did not shut down after SIGINT within timeout")
	}
}

// buildCubBinary compiles the cub binary and returns the path to it.
func buildCubBinary(t *testing.T) string {
	t.Helper()

	// Create temporary binary path
	binPath := t.TempDir() + "/cub"

	// Build the binary
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	require.NoError(t, err, "Failed to build cub binary")

	t.Logf("Built cub binary at: %s", binPath)
	return binPath
}
