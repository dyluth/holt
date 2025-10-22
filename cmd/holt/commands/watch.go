package commands

import (
	"context"
	"fmt"
	"os"

	dockerpkg "github.com/dyluth/holt/internal/docker"
	"github.com/dyluth/holt/internal/instance"
	"github.com/dyluth/holt/internal/printer"
	"github.com/dyluth/holt/internal/watch"
	"github.com/dyluth/holt/pkg/blackboard"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
)

var (
	watchInstanceName string
	watchOutputFormat string
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Monitor real-time workflow activity",
	Long: `Monitor real-time workflow progress and agent activity.

Streams artefact creations, claim events, agent bids, and grant decisions
as they occur, providing complete visibility into workflow execution.

Output Formats:
  default - Human-readable output with timestamps and emojis
  json    - Line-delimited JSON for programmatic processing

Examples:
  # Watch all activity on inferred instance
  holt watch

  # Watch specific instance
  holt watch --name prod

  # Export events as JSON
  holt watch --output=json > events.jsonl`,
	RunE: runWatch,
}

func init() {
	watchCmd.Flags().StringVarP(&watchInstanceName, "name", "n", "", "Target instance name (auto-inferred if omitted)")
	watchCmd.Flags().StringVarP(&watchOutputFormat, "output", "o", "default", "Output format (default or json)")
	rootCmd.AddCommand(watchCmd)
}

func runWatch(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Validate output format
	var outputFormat watch.OutputFormat
	switch watchOutputFormat {
	case "default":
		outputFormat = watch.OutputFormatDefault
	case "json":
		outputFormat = watch.OutputFormatJSON
	default:
		return printer.Error(
			"invalid output format",
			fmt.Sprintf("Unknown format: %s", watchOutputFormat),
			[]string{"Valid formats: default, json"},
		)
	}

	// Phase 1: Instance discovery
	cli, err := dockerpkg.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer cli.Close()

	targetInstanceName := watchInstanceName
	if targetInstanceName == "" {
		targetInstanceName, err = instance.InferInstanceFromWorkspace(ctx, cli)
		if err != nil {
			if err.Error() == "no Holt instances found for this workspace" {
				return printer.Error(
					"no Holt instances found",
					"No running instances found for this workspace.",
					[]string{"Start an instance first:\n  holt up"},
				)
			}
			if err.Error() == "multiple instances found for this workspace, use --name to specify which one" {
				return printer.Error(
					"multiple instances found",
					"Found multiple running instances for this workspace.",
					[]string{
						"Specify which instance to watch:\n  holt watch --name <instance-name>",
						"List instances:\n  holt list",
					},
				)
			}
			return fmt.Errorf("failed to infer instance: %w", err)
		}
	}

	// Phase 2: Verify instance is running
	if err := instance.VerifyInstanceRunning(ctx, cli, targetInstanceName); err != nil {
		return printer.Error(
			fmt.Sprintf("instance '%s' is not running", targetInstanceName),
			fmt.Sprintf("Error: %v", err),
			[]string{fmt.Sprintf("Start the instance:\n  holt up --name %s", targetInstanceName)},
		)
	}

	// Phase 3: Get Redis port
	redisPort, err := instance.GetInstanceRedisPort(ctx, cli, targetInstanceName)
	if err != nil {
		return printer.ErrorWithContext(
			"Redis port not found",
			fmt.Sprintf("Instance '%s' exists but Redis port label is missing.", targetInstanceName),
			nil,
			[]string{fmt.Sprintf("Restart the instance:\n  holt down --name %s\n  holt up --name %s", targetInstanceName, targetInstanceName)},
		)
	}

	// Phase 4: Connect to blackboard
	redisURL := instance.GetRedisURL(redisPort)
	redisOpts, err := redis.ParseURL(redisURL)
	if err != nil {
		return fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	bbClient, err := blackboard.NewClient(redisOpts, targetInstanceName)
	if err != nil {
		return fmt.Errorf("failed to create blackboard client: %w", err)
	}
	defer bbClient.Close()

	// Verify Redis connectivity
	if err := bbClient.Ping(ctx); err != nil {
		return printer.ErrorWithContext(
			"Redis connection failed",
			fmt.Sprintf("Could not connect to Redis at %s", redisURL),
			nil,
			[]string{
				fmt.Sprintf("Check Redis container status:\n  docker logs holt-redis-%s", targetInstanceName),
				fmt.Sprintf("Restart if needed:\n  holt down --name %s\n  holt up --name %s", targetInstanceName, targetInstanceName),
			},
		)
	}

	// Phase 5: Stream workflow activity
	return watch.StreamActivity(ctx, bbClient, targetInstanceName, outputFormat, os.Stdout)
}
