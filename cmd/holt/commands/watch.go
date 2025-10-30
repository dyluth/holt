package commands

import (
	"context"
	"fmt"
	"os"

	dockerpkg "github.com/dyluth/holt/internal/docker"
	"github.com/dyluth/holt/internal/instance"
	"github.com/dyluth/holt/internal/printer"
	"github.com/dyluth/holt/internal/timespec"
	"github.com/dyluth/holt/internal/watch"
	"github.com/dyluth/holt/pkg/blackboard"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
)

var (
	watchInstanceName     string
	watchOutputFormat     string
	watchSince            string
	watchUntil            string
	watchType             string
	watchAgent            string
	watchExitOnCompletion bool
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Monitor real-time workflow activity with filtering",
	Long: `Monitor real-time workflow progress and agent activity with powerful filtering.

Displays historical events matching filters, then streams live events as they occur.

Output Formats:
  default - Human-readable output with timestamps and emojis
  jsonl   - Line-delimited JSON, one event per line (streamable)

Time Filters:
  --since  - Show events after this time
             Duration: 1h, 30m, 1h30m, 2h45m30s
             Absolute: 2025-10-29T13:00:00Z (RFC3339)
  --until  - Show events before this time (same format as --since)

Content Filters:
  --type   - Filter by artefact type (glob pattern: "Code*", "*Result")
  --agent  - Filter by agent role (exact match: "coder", "reviewer")

Examples:
  # Watch all activity (historical + live)
  holt watch

  # Watch and exit when workflow completes
  holt watch --exit-on-completion

  # Filter for code commits in last hour
  holt watch --since=1h --type="CodeCommit"

  # Export events as JSONL for processing
  holt watch --output=jsonl --since=30m | jq -r 'select(.event=="artefact_created") | .data.id'

  # Monitor specific agent
  holt watch --agent=coder --since="2025-10-29T13:00:00Z"`,
	RunE: runWatch,
}

func init() {
	watchCmd.Flags().StringVarP(&watchInstanceName, "name", "n", "", "Target instance name (auto-inferred if omitted)")
	watchCmd.Flags().StringVarP(&watchOutputFormat, "output", "o", "default", "Output format (default or jsonl)")

	// Time-based filters
	watchCmd.Flags().StringVar(&watchSince, "since", "", "Show events after time (duration or RFC3339)")
	watchCmd.Flags().StringVar(&watchUntil, "until", "", "Show events before time (duration or RFC3339)")

	// Content-based filters
	watchCmd.Flags().StringVar(&watchType, "type", "", "Filter by artefact type (glob pattern)")
	watchCmd.Flags().StringVar(&watchAgent, "agent", "", "Filter by agent role (exact match)")

	// Behavior flags
	watchCmd.Flags().BoolVar(&watchExitOnCompletion, "exit-on-completion", false, "Exit with code 0 when Terminal artefact detected")

	rootCmd.AddCommand(watchCmd)
}

func runWatch(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Validate output format
	var outputFormat watch.OutputFormat
	switch watchOutputFormat {
	case "default":
		outputFormat = watch.OutputFormatDefault
	case "jsonl":
		outputFormat = watch.OutputFormatJSONL
	default:
		return printer.Error(
			"invalid output format",
			fmt.Sprintf("Unknown format: %s", watchOutputFormat),
			[]string{"Valid formats: default, jsonl"},
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

	// Phase 5: Parse time filters
	sinceMS, untilMS, err := parseTimeFilters()
	if err != nil {
		return printer.Error(
			"invalid time filter",
			err.Error(),
			[]string{"Use duration format like '1h30m' or RFC3339 like '2025-10-29T13:00:00Z'"},
		)
	}

	// Phase 6: Build filter criteria
	filterCriteria := &watch.FilterCriteria{
		SinceTimestampMs: sinceMS,
		UntilTimestampMs: untilMS,
		TypeGlob:         watchType,
		AgentRole:        watchAgent,
	}

	// Phase 7: Stream workflow activity
	return watch.StreamActivity(ctx, bbClient, targetInstanceName, outputFormat, filterCriteria, watchExitOnCompletion, os.Stdout)
}

// parseTimeFilters parses the --since and --until flags into millisecond timestamps.
// Returns (sinceMS, untilMS, error).
func parseTimeFilters() (int64, int64, error) {
	return timespec.ParseRange(watchSince, watchUntil)
}
