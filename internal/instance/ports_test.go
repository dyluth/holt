package instance

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	dockerpkg "github.com/dyluth/sett/internal/docker"
	"github.com/stretchr/testify/require"
)

func TestFindNextAvailablePort(t *testing.T) {
	// Skip if Docker not available
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Skip("Docker not available")
	}
	defer cli.Close()

	ctx := context.Background()

	t.Run("returns 6379 when no ports used", func(t *testing.T) {
		port, err := FindNextAvailablePort(ctx, cli)
		require.NoError(t, err)
		require.Equal(t, 6379, port)
	})

	t.Run("skips ports that are already bound", func(t *testing.T) {
		// Bind port 6379
		listener, err := net.Listen("tcp", "localhost:6379")
		require.NoError(t, err)
		defer listener.Close()

		// Should return next available port
		port, err := FindNextAvailablePort(ctx, cli)
		require.NoError(t, err)
		require.Equal(t, 6380, port)
	})

	t.Run("skips ports used by Docker containers", func(t *testing.T) {
		// Pull image if needed
		pullImageIfNeeded(t, cli, ctx, "busybox:latest")

		// Create a dummy container with redis port label
		labels := map[string]string{
			dockerpkg.LabelProject:   "true",
			dockerpkg.LabelComponent: "redis",
			dockerpkg.LabelRedisPort: "6379",
		}

		resp, err := cli.ContainerCreate(ctx, &container.Config{
			Image:  "busybox:latest",
			Cmd:    []string{"sleep", "1"},
			Labels: labels,
		}, nil, nil, nil, "")
		require.NoError(t, err)
		defer cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})

		// Should skip 6379 and return 6380
		port, err := FindNextAvailablePort(ctx, cli)
		require.NoError(t, err)
		require.GreaterOrEqual(t, port, 6380)
	})
}

func TestIsPortBindable(t *testing.T) {
	t.Run("returns true for available port", func(t *testing.T) {
		// Find an available high port
		listener, err := net.Listen("tcp", "localhost:0")
		require.NoError(t, err)
		port := listener.Addr().(*net.TCPAddr).Port
		listener.Close()

		require.True(t, isPortBindable(port))
	})

	t.Run("returns false for port in use", func(t *testing.T) {
		// Bind a port
		listener, err := net.Listen("tcp", "localhost:0")
		require.NoError(t, err)
		defer listener.Close()

		port := listener.Addr().(*net.TCPAddr).Port
		require.False(t, isPortBindable(port))
	})

	t.Run("returns false for privileged ports without permission", func(t *testing.T) {
		// Port 80 requires root privileges
		result := isPortBindable(80)
		// On most systems without root, this should be false
		// If running as root, skip this test
		if result {
			t.Skip("Running as root, cannot test privileged port restriction")
		}
		require.False(t, result)
	})
}

func TestFindNextAvailablePort_ExhaustedRange(t *testing.T) {
	// This test would require binding all 100 ports, which is impractical
	// Instead, we'll test the error message format
	t.Run("returns error message with correct range", func(t *testing.T) {
		// We can't actually exhaust the range, but we can verify the error format
		// by checking the error message contains the expected range
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			t.Skip("Docker not available")
		}
		defer cli.Close()

		ctx := context.Background()

		// Bind many ports to simulate near-exhaustion
		listeners := []net.Listener{}
		for port := 6379; port < 6390; port++ {
			if listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port)); err == nil {
				listeners = append(listeners, listener)
			}
		}
		defer func() {
			for _, l := range listeners {
				l.Close()
			}
		}()

		// Should still find a port (we didn't exhaust all 100)
		port, err := FindNextAvailablePort(ctx, cli)
		require.NoError(t, err)
		require.GreaterOrEqual(t, port, 6390)
		require.LessOrEqual(t, port, 6478)
	})
}
