package commands

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	dockerpkg "github.com/dyluth/sett/internal/docker"
	"github.com/spf13/cobra"
)

var (
	downInstanceName string
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop a Sett instance",
	Long: `Stop and remove all Docker resources associated with a Sett instance.

This includes:
  • All containers (Redis, orchestrator, agents)
  • Docker network

The command does not prompt for confirmation and executes immediately.`,
	RunE: runDown,
}

func init() {
	downCmd.Flags().StringVar(&downInstanceName, "name", "", "Instance name (required)")
	downCmd.MarkFlagRequired("name")
	rootCmd.AddCommand(downCmd)
}

func runDown(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Create Docker client
	cli, err := dockerpkg.NewClient(ctx)
	if err != nil {
		return err
	}
	defer cli.Close()

	// Find all containers for this instance
	containerFilters := filters.NewArgs()
	containerFilters.Add("label", fmt.Sprintf("%s=%s", dockerpkg.LabelInstanceName, downInstanceName))

	containers, err := cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: containerFilters,
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		return fmt.Errorf(`instance '%s' not found

No containers found with instance name '%s'.

Run 'sett list' to see available instances.`, downInstanceName, downInstanceName)
	}

	// Stop containers (10s graceful timeout)
	timeout := 10
	for _, c := range containers {
		containerName := c.Names[0]
		fmt.Printf("Stopping %s...\n", containerName)
		if err := cli.ContainerStop(ctx, c.ID, container.StopOptions{Timeout: &timeout}); err != nil {
			// Log but continue - container might already be stopped
			fmt.Printf("Warning: failed to stop %s: %v\n", containerName, err)
		}
	}

	// Remove containers
	for _, c := range containers {
		containerName := c.Names[0]
		fmt.Printf("Removing %s...\n", containerName)
		if err := cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true, RemoveVolumes: true}); err != nil {
			return fmt.Errorf("failed to remove %s: %w", containerName, err)
		}
	}

	// Find and remove network
	networkFilters := filters.NewArgs()
	networkFilters.Add("label", fmt.Sprintf("%s=%s", dockerpkg.LabelInstanceName, downInstanceName))

	networks, err := cli.NetworkList(ctx, types.NetworkListOptions{
		Filters: networkFilters,
	})
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}

	for _, net := range networks {
		fmt.Printf("Removing network %s...\n", net.Name)
		if err := cli.NetworkRemove(ctx, net.ID); err != nil {
			return fmt.Errorf("failed to remove network %s: %w", net.Name, err)
		}
	}

	fmt.Printf("\n✓ Instance '%s' removed successfully\n", downInstanceName)

	return nil
}
