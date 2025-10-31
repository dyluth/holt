package commands

import (
	"context"
	"fmt"
	"os"

	dockerpkg "github.com/dyluth/holt/internal/docker"
	"github.com/dyluth/holt/internal/hoard"
	"github.com/dyluth/holt/internal/instance"
	"github.com/dyluth/holt/internal/printer"
	"github.com/dyluth/holt/internal/resolver"
	"github.com/dyluth/holt/internal/timespec"
	"github.com/dyluth/holt/pkg/blackboard"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
)

var (
	hoardInstanceName string
	hoardOutputFormat string
	hoardSince        string
	hoardUntil        string
	hoardType         string
	hoardAgent        string
)

var hoardCmd = &cobra.Command{
	Use:   "hoard [ARTEFACT_ID]",
	Short: "Inspect blackboard artefacts with filtering",
	Long: `Inspect blackboard artefacts in list or get mode.

List Mode (no ARTEFACT_ID):
  Displays artefacts matching filters as a table or JSONL stream.

Get Mode (with ARTEFACT_ID):
  Displays complete details of a single artefact as pretty-printed JSON.
  Supports short IDs (e.g., "abc123" instead of full UUID).

Output Formats (list mode only):
  default - Human-readable table with ID, Type, Produced By, and Payload
  jsonl   - Line-delimited JSON, one artefact per line

Time Filters (list mode only):
  --since  - Show artefacts created after this time
  --until  - Show artefacts created before this time

Content Filters (list mode only):
  --type   - Filter by artefact type (glob pattern: "Code*", "*Result")
  --agent  - Filter by agent role (exact match: "coder", "reviewer")

Examples:
  # List all artefacts
  holt hoard

  # Filter by type and time
  holt hoard --type="CodeCommit" --since=2h

  # Get artefacts as JSONL for piping to jq
  holt hoard --output=jsonl --since=1h | jq 'select(.structural_type=="Terminal") | .id'

  # Get specific artefact by short ID
  holt hoard abc123

  # Filter by agent
  holt hoard --agent=reviewer --since="2025-10-29T00:00:00Z"`,
	RunE: runHoard,
}

func init() {
	hoardCmd.Flags().StringVarP(&hoardInstanceName, "name", "n", "", "Target instance name (auto-inferred if omitted)")
	hoardCmd.Flags().StringVarP(&hoardOutputFormat, "output", "o", "default", "Output format: default or jsonl (ignored in get mode)")

	// Time-based filters
	hoardCmd.Flags().StringVar(&hoardSince, "since", "", "Show artefacts after time (duration or RFC3339)")
	hoardCmd.Flags().StringVar(&hoardUntil, "until", "", "Show artefacts before time (duration or RFC3339)")

	// Content-based filters
	hoardCmd.Flags().StringVar(&hoardType, "type", "", "Filter by artefact type (glob pattern)")
	hoardCmd.Flags().StringVar(&hoardAgent, "agent", "", "Filter by agent role (exact match)")

	rootCmd.AddCommand(hoardCmd)
}

func runHoard(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Determine mode based on arguments
	isGetMode := len(args) > 0

	// Validate output format (only applies to list mode)
	var outputFormat hoard.OutputFormat
	if !isGetMode {
		switch hoardOutputFormat {
		case "default":
			outputFormat = hoard.OutputFormatDefault
		case "jsonl":
			outputFormat = hoard.OutputFormatJSONL
		default:
			return printer.Error(
				"invalid output format",
				fmt.Sprintf("Unknown format: %s", hoardOutputFormat),
				[]string{"Valid formats: default, jsonl"},
			)
		}
	}

	// Phase 1: Instance discovery
	cli, err := dockerpkg.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer cli.Close()

	targetInstanceName := hoardInstanceName
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
						"Specify which instance to inspect:\n  holt hoard --name <instance-name>",
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

	// Phase 5: Execute appropriate mode
	if isGetMode {
		// Get mode: resolve short ID and fetch artefact
		shortID := args[0]

		// Resolve short ID to full UUID
		fullID, err := resolver.ResolveArtefactID(ctx, bbClient, shortID)
		if err != nil {
			// Handle resolver-specific errors
			if resolver.IsNotFoundError(err) {
				return printer.Error(
					fmt.Sprintf("artefact with ID '%s' not found", shortID),
					"The specified artefact does not exist on the blackboard.",
					[]string{
						"List all artefacts:\n  holt hoard",
						fmt.Sprintf("Verify instance:\n  holt hoard --name %s", targetInstanceName),
					},
				)
			}
			if resolver.IsAmbiguousError(err) {
				ambigErr := err.(*resolver.AmbiguousError)
				fmt.Fprintln(os.Stderr, resolver.FormatAmbiguousError(ambigErr))
				return fmt.Errorf("ambiguous short ID")
			}
			return fmt.Errorf("failed to resolve artefact ID: %w", err)
		}

		// Fetch and display artefact
		err = hoard.GetArtefact(ctx, bbClient, fullID, os.Stdout)
		if err != nil {
			if hoard.IsNotFound(err) {
				return printer.Error(
					fmt.Sprintf("artefact with ID '%s' not found", fullID),
					"The artefact was resolved but could not be fetched.",
					[]string{
						"This might indicate a race condition. Try again.",
					},
				)
			}
			return fmt.Errorf("failed to get artefact: %w", err)
		}
	} else {
		// List mode: parse filters and fetch artefacts
		sinceMS, untilMS, err := timespec.ParseRange(hoardSince, hoardUntil)
		if err != nil {
			return printer.Error(
				"invalid time filter",
				err.Error(),
				[]string{"Use duration format like '1h30m' or RFC3339 like '2025-10-29T13:00:00Z'"},
			)
		}

		// Build filter criteria
		filterCriteria := &hoard.FilterCriteria{
			SinceTimestampMs: sinceMS,
			UntilTimestampMs: untilMS,
			TypeGlob:         hoardType,
			AgentRole:        hoardAgent,
		}

		// List artefacts with filtering
		err = hoard.ListArtefacts(ctx, bbClient, targetInstanceName, outputFormat, filterCriteria, os.Stdout)
		if err != nil {
			return fmt.Errorf("failed to list artefacts: %w", err)
		}
	}

	return nil
}
