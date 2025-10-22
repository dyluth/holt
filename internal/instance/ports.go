package instance

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	dockerpkg "github.com/dyluth/holt/internal/docker"
)

const (
	// Port range for Redis containers (allows 100 concurrent instances)
	startPort = 6379
	endPort   = 6478
)

// FindNextAvailablePort finds the next available port for Redis, starting from 6379.
// Returns the port number or error if all ports in range (6379-6478) are exhausted.
// Checks both Docker container labels and actual port bindability on the host.
func FindNextAvailablePort(ctx context.Context, cli *client.Client) (int, error) {
	// Query Docker for existing holt.redis.port labels
	filter := filters.NewArgs()
	filter.Add("label", fmt.Sprintf("%s=true", dockerpkg.LabelProject))
	filter.Add("label", fmt.Sprintf("%s=redis", dockerpkg.LabelComponent))

	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: filter,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to query Docker containers: %w", err)
	}

	// Build set of used ports from Docker labels
	usedPorts := make(map[int]bool)
	for _, container := range containers {
		if portStr, ok := container.Labels[dockerpkg.LabelRedisPort]; ok {
			if port, err := strconv.Atoi(portStr); err == nil {
				usedPorts[port] = true
			}
		}
	}

	// Find first available port
	for port := startPort; port <= endPort; port++ {
		if usedPorts[port] {
			continue
		}

		// Verify port is bindable on host
		if isPortBindable(port) {
			return port, nil
		}
	}

	return 0, fmt.Errorf("no available Redis ports (range %d-%d exhausted)", startPort, endPort)
}

// isPortBindable checks if a port can be bound on localhost.
// Returns true if port is available, false if in use.
func isPortBindable(port int) bool {
	addr := fmt.Sprintf("localhost:%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}
