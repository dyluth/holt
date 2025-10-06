package instance

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	dockerpkg "github.com/dyluth/sett/internal/docker"
)

const (
	// DefaultNamePrefix is the prefix for auto-generated instance names
	DefaultNamePrefix = "default-"

	// MaxNameLength is the maximum length for an instance name (DNS-compatible)
	MaxNameLength = 63
)

var (
	// NamePattern is the regex pattern for valid instance names
	// Must be DNS-compatible: lowercase alphanumeric, hyphens allowed (but not at start/end)
	// Allows single character or multiple characters with optional hyphens in between
	NamePattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
)

// ValidateName checks if an instance name is valid according to DNS naming rules.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("instance name cannot be empty")
	}

	if len(name) > MaxNameLength {
		return fmt.Errorf("instance name too long: %d characters (max: %d)", len(name), MaxNameLength)
	}

	if !NamePattern.MatchString(name) {
		return fmt.Errorf("invalid instance name '%s': must be lowercase alphanumeric with hyphens (not at start/end)", name)
	}

	return nil
}

// GenerateDefaultName generates the next available default-N instance name.
// It queries Docker for all existing sett containers and finds the highest N in default-N names.
func GenerateDefaultName(ctx context.Context, cli *client.Client) (string, error) {
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

	// Find highest default-N number
	highestN := 0
	for _, container := range containers {
		instanceName := container.Labels[dockerpkg.LabelInstanceName]
		if strings.HasPrefix(instanceName, DefaultNamePrefix) {
			// Extract number after "default-"
			numStr := strings.TrimPrefix(instanceName, DefaultNamePrefix)
			if n, err := strconv.Atoi(numStr); err == nil {
				if n > highestN {
					highestN = n
				}
			}
		}
	}

	// Return next number
	return fmt.Sprintf("%s%d", DefaultNamePrefix, highestN+1), nil
}

// CheckNameCollision checks if an instance with the given name already exists.
// Returns true if a collision exists (name is in use).
func CheckNameCollision(ctx context.Context, cli *client.Client, instanceName string) (bool, error) {
	filter := filters.NewArgs()
	filter.Add("label", fmt.Sprintf("%s=%s", dockerpkg.LabelInstanceName, instanceName))

	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: filter,
	})
	if err != nil {
		return false, fmt.Errorf("failed to check for name collision: %w", err)
	}

	return len(containers) > 0, nil
}
