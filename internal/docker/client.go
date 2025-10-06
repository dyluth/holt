package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/client"
)

// NewClient creates a Docker client and validates daemon is accessible.
// Returns an error if the Docker daemon is not running or not accessible.
func NewClient(ctx context.Context) (*client.Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Validate daemon is accessible
	if _, err := cli.Ping(ctx); err != nil {
		return nil, fmt.Errorf(`Docker daemon not accessible: %w

Ensure Docker is running:
  • macOS: Docker Desktop
  • Linux: sudo systemctl start docker`, err)
	}

	return cli, nil
}
