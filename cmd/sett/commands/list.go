package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	dockerpkg "github.com/dyluth/sett/internal/docker"
	"github.com/dyluth/sett/internal/instance"
	"github.com/spf13/cobra"
)

var (
	listJSON bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all Sett instances",
	Long: `List all Sett instances by querying Docker for containers with the sett.project label.

For each instance, displays:
  • Instance name
  • Status (Running/Degraded/Stopped)
  • Workspace path
  • Uptime (for running instances)

Use --json for machine-readable output.`,
	RunE: runList,
}

func init() {
	listCmd.Flags().BoolVar(&listJSON, "json", false, "Output in JSON format")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Create Docker client
	cli, err := dockerpkg.NewClient(ctx)
	if err != nil {
		return err
	}
	defer cli.Close()

	// Find all Sett containers
	containerFilters := filters.NewArgs()
	containerFilters.Add("label", fmt.Sprintf("%s=true", dockerpkg.LabelProject))

	containers, err := cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: containerFilters,
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	// Group by instance name
	instances := make(map[string][]types.Container)
	for _, c := range containers {
		instanceName := c.Labels[dockerpkg.LabelInstanceName]
		instances[instanceName] = append(instances[instanceName], c)
	}

	// Build instance info
	var infos []instance.InstanceInfo
	for name, containers := range instances {
		status := instance.DetermineStatus(containers)

		// Get metadata from first container (all have same labels)
		workspacePath := containers[0].Labels[dockerpkg.LabelWorkspacePath]
		createdAt := containers[0].Created

		// Calculate uptime (for Running status only)
		var uptime string
		if status == instance.StatusRunning {
			duration := time.Since(time.Unix(createdAt, 0))
			uptime = formatDuration(duration)
		} else {
			uptime = "-"
		}

		infos = append(infos, instance.InstanceInfo{
			Name:      name,
			Status:    status,
			Workspace: workspacePath,
			Uptime:    uptime,
		})
	}

	// Sort by name
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})

	// Output
	if len(infos) == 0 {
		if !listJSON {
			fmt.Println("No Sett instances found.")
			fmt.Println()
			fmt.Println("Run 'sett up' to start a new instance.")
		} else {
			fmt.Println("[]")
		}
		return nil
	}

	if listJSON {
		outputJSON(infos)
	} else {
		outputTable(infos)
	}

	return nil
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)

	hours := d / time.Hour
	d -= hours * time.Hour

	minutes := d / time.Minute
	d -= minutes * time.Minute

	seconds := d / time.Second

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	} else {
		return fmt.Sprintf("%ds", seconds)
	}
}

func outputJSON(infos []instance.InstanceInfo) {
	data, err := json.MarshalIndent(infos, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling JSON: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

func outputTable(infos []instance.InstanceInfo) {
	// Print header
	fmt.Printf("%-15s %-10s %-30s %s\n", "INSTANCE", "STATUS", "WORKSPACE", "UPTIME")

	// Print rows
	for _, info := range infos {
		// Truncate workspace if too long
		workspace := info.Workspace
		if len(workspace) > 30 {
			workspace = "..." + workspace[len(workspace)-27:]
		}

		fmt.Printf("%-15s %-10s %-30s %s\n", info.Name, info.Status, workspace, info.Uptime)
	}
}
