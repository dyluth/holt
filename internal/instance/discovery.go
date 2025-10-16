package instance

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	dockerpkg "github.com/dyluth/sett/internal/docker"
)

// FindInstanceByWorkspace finds the Sett instance running on the given workspace path.
// Returns the instance name or an error if 0 or 2+ instances are found.
// Canonicalizes the workspace path before comparison to handle symlinks.
func FindInstanceByWorkspace(ctx context.Context, cli *client.Client, workspacePath string) (string, error) {
	// Canonicalize the provided workspace path
	canonicalPath, err := filepath.EvalSymlinks(workspacePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve workspace path: %w", err)
	}
	canonicalPath, err = filepath.Abs(canonicalPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute workspace path: %w", err)
	}

	// Find all Sett containers
	filter := filters.NewArgs()
	filter.Add("label", fmt.Sprintf("%s=true", dockerpkg.LabelProject))

	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: filter,
	})
	if err != nil {
		return "", fmt.Errorf("failed to list containers: %w", err)
	}

	// Find containers matching the workspace
	var matchingInstances []string
	for _, container := range containers {
		containerWorkspace := container.Labels[dockerpkg.LabelWorkspacePath]
		if containerWorkspace == canonicalPath {
			instanceName := container.Labels[dockerpkg.LabelInstanceName]
			// Check if we've already seen this instance
			found := false
			for _, existing := range matchingInstances {
				if existing == instanceName {
					found = true
					break
				}
			}
			if !found {
				matchingInstances = append(matchingInstances, instanceName)
			}
		}
	}

	// Return based on matches
	if len(matchingInstances) == 0 {
		return "", fmt.Errorf("no instances found")
	}
	if len(matchingInstances) > 1 {
		return "", fmt.Errorf("multiple instances found: %v", matchingInstances)
	}

	return matchingInstances[0], nil
}

// GetInstanceRedisPort retrieves the Redis port for the given instance from Docker labels.
// Returns an error if the Redis container is not found or the port label is missing.
func GetInstanceRedisPort(ctx context.Context, cli *client.Client, instanceName string) (int, error) {
	// Find Redis container for this instance
	filter := filters.NewArgs()
	filter.Add("label", fmt.Sprintf("%s=%s", dockerpkg.LabelInstanceName, instanceName))
	filter.Add("label", fmt.Sprintf("%s=redis", dockerpkg.LabelComponent))

	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: filter,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		return 0, fmt.Errorf("Redis container not found for instance '%s'", instanceName)
	}

	// Get port from label
	redisContainer := containers[0]
	portStr, ok := redisContainer.Labels[dockerpkg.LabelRedisPort]
	if !ok {
		return 0, fmt.Errorf("Redis port label missing for instance '%s'", instanceName)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("invalid Redis port '%s': %w", portStr, err)
	}

	return port, nil
}

// VerifyInstanceRunning checks if the given instance's containers are running.
// Returns an error if any required container (Redis, orchestrator) is not running.
// Note: Agent containers may exit after completing work and are not checked.
func VerifyInstanceRunning(ctx context.Context, cli *client.Client, instanceName string) error {
	// Find all containers for this instance
	filter := filters.NewArgs()
	filter.Add("label", fmt.Sprintf("%s=%s", dockerpkg.LabelInstanceName, instanceName))

	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: filter,
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		return fmt.Errorf("instance '%s' not found", instanceName)
	}

	// Check that essential containers (Redis, orchestrator) are running
	// Agent containers may exit after completing work, so they are not checked
	essentialComponents := map[string]bool{
		"redis":        false,
		"orchestrator": false,
	}

	for _, container := range containers {
		component := container.Labels[dockerpkg.LabelComponent]

		// If this is an essential component, mark it as found and check if running
		if _, isEssential := essentialComponents[component]; isEssential {
			essentialComponents[component] = true
			if container.State != "running" {
				return fmt.Errorf("instance '%s' is not running (component '%s' is %s)", instanceName, component, container.State)
			}
		}
	}

	// Verify that all essential components were found
	for component, found := range essentialComponents {
		if !found {
			return fmt.Errorf("instance '%s' is missing essential component '%s'", instanceName, component)
		}
	}

	return nil
}

// InferInstanceFromWorkspace infers the instance name from the current working directory.
// Returns the instance name or a helpful error if inference fails.
func InferInstanceFromWorkspace(ctx context.Context, cli *client.Client) (string, error) {
	// Get Git root as canonical workspace path
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not in a Git repository")
	}

	gitRoot := strings.TrimSpace(string(output))

	// Canonicalize path
	canonicalPath, err := filepath.EvalSymlinks(gitRoot)
	if err != nil {
		return "", fmt.Errorf("failed to resolve workspace path: %w", err)
	}
	canonicalPath, err = filepath.Abs(canonicalPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute workspace path: %w", err)
	}

	// Find instance
	instanceName, err := FindInstanceByWorkspace(ctx, cli, canonicalPath)
	if err != nil {
		if strings.Contains(err.Error(), "no instances found") {
			return "", fmt.Errorf("no Sett instances found for this workspace")
		}
		if strings.Contains(err.Error(), "multiple instances found") {
			return "", fmt.Errorf("multiple instances found for this workspace, use --name to specify which one")
		}
		return "", err
	}

	return instanceName, nil
}
