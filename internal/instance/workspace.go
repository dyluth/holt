package instance

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	dockerpkg "github.com/dyluth/sett/internal/docker"
)

// GetCanonicalWorkspacePath gets the absolute, canonical workspace path from the Git repository.
// This path is used for workspace collision detection.
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

	return absPath, nil
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
	// Find all Sett containers
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
