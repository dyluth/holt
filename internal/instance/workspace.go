package instance

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	dockerpkg "github.com/dyluth/holt/internal/docker"
)

// GetCanonicalWorkspacePath gets the absolute, canonical workspace path from the Git repository.
// This path is used for workspace collision detection.
// In Docker-in-Docker scenarios, this automatically translates container paths to host paths.
func GetCanonicalWorkspacePath() (string, error) {
	// Get Git root
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get git root: %w", err)
	}

	gitRoot := strings.TrimSpace(string(output))

	// Resolve symlinks
	realPath, err := filepath.EvalSymlinks(gitRoot)
	if err != nil {
		return "", fmt.Errorf("failed to resolve symlinks: %w", err)
	}

	// Get absolute path
	absPath, err := filepath.Abs(realPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Docker-in-Docker translation: if we're in a container and the path is under /app,
	// translate to the host path for Docker bind mounts
	absPath = translateContainerPathToHost(absPath)

	return absPath, nil
}

// translateContainerPathToHost translates container paths to host paths for Docker-in-Docker scenarios.
// If the path starts with /app and we're running in Docker, it translates /app to the host mount source.
func translateContainerPathToHost(containerPath string) string {
	// Only translate if path starts with /app
	if !strings.HasPrefix(containerPath, "/app") {
		return containerPath
	}

	// Check if we're in Docker (Docker-in-Docker scenario)
	if _, err := os.Stat("/.dockerenv"); err != nil {
		// Not in Docker, no translation needed
		return containerPath
	}

	// Try to detect the host path that /app maps to
	hostPath := detectHostPathForAppMount()
	if hostPath == "" {
		// Detection failed, return original path
		return containerPath
	}

	// Replace /app with host path
	return filepath.Join(hostPath, containerPath[len("/app"):])
}

// detectHostPathForAppMount tries to find the host path that /app is mounted from
func detectHostPathForAppMount() string {
	// Try to get our container hostname
	hostname, err := os.Hostname()
	if err != nil {
		return ""
	}

	// Create Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return ""
	}
	defer cli.Close()

	// Inspect our own container
	inspect, err := cli.ContainerInspect(context.Background(), hostname)
	if err != nil {
		return ""
	}

	// Look for /app mount
	for _, mount := range inspect.Mounts {
		if mount.Destination == "/app" {
			return mount.Source
		}
	}

	return ""
}

// WorkspaceCollision represents a workspace collision with another instance
type WorkspaceCollision struct {
	InstanceName  string
	WorkspacePath string
	ContainerID   string
}

// CheckWorkspaceCollision checks if any other instance is using the given workspace path.
// Returns a collision object if found, or nil if no collision.
// The currentInstanceName parameter allows checking for collisions with other instances
// (excluding the current instance being created/updated).
func CheckWorkspaceCollision(ctx context.Context, cli *client.Client, workspacePath, currentInstanceName string) (*WorkspaceCollision, error) {
	// Find all Holt containers
	filter := filters.NewArgs()
	filter.Add("label", fmt.Sprintf("%s=true", dockerpkg.LabelProject))

	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: filter,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	// Check for workspace collision
	for _, container := range containers {
		containerWorkspace := container.Labels[dockerpkg.LabelWorkspacePath]
		containerInstance := container.Labels[dockerpkg.LabelInstanceName]

		// Skip if this is the current instance
		if containerInstance == currentInstanceName {
			continue
		}

		// Check for collision
		if containerWorkspace == workspacePath {
			return &WorkspaceCollision{
				InstanceName:  containerInstance,
				WorkspacePath: containerWorkspace,
				ContainerID:   container.ID,
			}, nil
		}
	}

	return nil, nil
}
