package commands

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	dockerpkg "github.com/dyluth/sett/internal/docker"
	"github.com/dyluth/sett/internal/instance"
	"github.com/dyluth/sett/internal/printer"
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

The instance name is auto-inferred from the current workspace if not specified.
The command does not prompt for confirmation and executes immediately.

Examples:
  # Stop the instance for current workspace
  sett down

  # Stop a specific instance
  sett down --name prod-instance`,
	RunE: runDown,
}

func init() {
	downCmd.Flags().StringVarP(&downInstanceName, "name", "n", "", "Target instance name (auto-inferred if omitted)")
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

	// Phase 1: Instance discovery
	targetInstanceName := downInstanceName
	if targetInstanceName == "" {
		targetInstanceName, err = instance.InferInstanceFromWorkspace(ctx, cli)
		if err != nil {
			if err.Error() == "no Sett instances found for this workspace" {
				return printer.Error(
					"no Sett instances found",
					"No running instances found for this workspace.",
					[]string{"Start an instance first:\n  sett up"},
				)
			}
			if err.Error() == "multiple instances found for this workspace, use --name to specify which one" {
				return printer.Error(
					"multiple instances found",
					"Found multiple running instances for this workspace.",
					[]string{
						"Specify which instance to stop:\n  sett down --name <instance-name>",
						"List instances:\n  sett list",
					},
				)
			}
			return fmt.Errorf("failed to infer instance: %w", err)
		}
	}

	// Find all containers for this instance
	containerFilters := filters.NewArgs()
	containerFilters.Add("label", fmt.Sprintf("%s=%s", dockerpkg.LabelInstanceName, targetInstanceName))

	containers, err := cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: containerFilters,
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		return printer.Error(
			fmt.Sprintf("instance '%s' not found", targetInstanceName),
			fmt.Sprintf("No containers found with instance name '%s'.", targetInstanceName),
			[]string{"Run 'sett list' to see available instances"},
		)
	}

	// Stop containers (10s graceful timeout)
	timeout := 10
	for _, c := range containers {
		containerName := c.Names[0]
		printer.Step("Stopping %s...\n", containerName)
		if err := cli.ContainerStop(ctx, c.ID, container.StopOptions{Timeout: &timeout}); err != nil {
			// Log but continue - container might already be stopped
			printer.Warning("failed to stop %s: %v\n", containerName, err)
		}
	}

	// Remove containers
	for _, c := range containers {
		containerName := c.Names[0]
		printer.Step("Removing %s...\n", containerName)
		if err := cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true, RemoveVolumes: true}); err != nil {
			return fmt.Errorf("failed to remove %s: %w", containerName, err)
		}
	}

	// Find and remove network
	networkFilters := filters.NewArgs()
	networkFilters.Add("label", fmt.Sprintf("%s=%s", dockerpkg.LabelInstanceName, targetInstanceName))

	networks, err := cli.NetworkList(ctx, types.NetworkListOptions{
		Filters: networkFilters,
	})
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}

	for _, net := range networks {
		printer.Step("Removing network %s...\n", net.Name)
		if err := cli.NetworkRemove(ctx, net.ID); err != nil {
			return fmt.Errorf("failed to remove network %s: %w", net.Name, err)
		}
	}

	printer.Success("\nInstance '%s' removed successfully\n", targetInstanceName)

	return nil
}
