package commands

import (
	"context"
	"fmt"
	"os"

	dockerpkg "github.com/dyluth/sett/internal/docker"
	"github.com/dyluth/sett/internal/hoard"
	"github.com/dyluth/sett/internal/instance"
	"github.com/dyluth/sett/internal/printer"
	"github.com/dyluth/sett/pkg/blackboard"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
)

var (
	hoardInstanceName string
	hoardOutputFormat string
)

var hoardCmd = &cobra.Command{
	Use:   "hoard [ARTEFACT_ID]",
	Short: "Inspect blackboard artefacts",
	Long: `Inspect blackboard artefacts in list or get mode.

List Mode (no ARTEFACT_ID):
  Displays an overview of all artefacts on the blackboard as a table
  or JSON array. Use this to see what work products have been created.

Get Mode (with ARTEFACT_ID):
  Displays complete details of a single artefact as pretty-printed JSON.
  Use this to inspect a specific artefact in detail.

Output Formats (list mode only):
  default - Human-readable table with ID, Type, Produced By, and Payload
  json    - JSON array of complete artefact objects

Examples:
  # List all artefacts in table format
  sett hoard

  # List all artefacts for specific instance
  sett hoard --name prod-instance

  # Get artefacts as JSON for scripting
  sett hoard --output=json | jq '.[] | select(.type=="CodeCommit")'

  # Get full details of specific artefact
  sett hoard abc123-def456-...

  # Extract artefact IDs for processing
  sett hoard --output=json | jq -r '.[].id'`,
	RunE: runHoard,
}

func init() {
	hoardCmd.Flags().StringVarP(&hoardInstanceName, "name", "n", "", "Target instance name (auto-inferred if omitted)")
	hoardCmd.Flags().StringVarP(&hoardOutputFormat, "output", "o", "default", "Output format: default or json (ignored in get mode)")
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
		case "json":
			outputFormat = hoard.OutputFormatJSON
		default:
			return printer.Error(
				"invalid output format",
				fmt.Sprintf("Unknown format: %s", hoardOutputFormat),
				[]string{"Valid formats: default, json"},
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
						"Specify which instance to inspect:\n  sett hoard --name <instance-name>",
						"List instances:\n  sett list",
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
			[]string{fmt.Sprintf("Start the instance:\n  sett up --name %s", targetInstanceName)},
		)
	}

	// Phase 3: Get Redis port
	redisPort, err := instance.GetInstanceRedisPort(ctx, cli, targetInstanceName)
	if err != nil {
		return printer.ErrorWithContext(
			"Redis port not found",
			fmt.Sprintf("Instance '%s' exists but Redis port label is missing.", targetInstanceName),
			nil,
			[]string{fmt.Sprintf("Restart the instance:\n  sett down --name %s\n  sett up --name %s", targetInstanceName, targetInstanceName)},
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
				fmt.Sprintf("Check Redis container status:\n  docker logs sett-redis-%s", targetInstanceName),
				fmt.Sprintf("Restart if needed:\n  sett down --name %s\n  sett up --name %s", targetInstanceName, targetInstanceName),
			},
		)
	}

	// Phase 5: Execute appropriate mode
	if isGetMode {
		// Get mode: fetch and display single artefact
		artefactID := args[0]
		err := hoard.GetArtefact(ctx, bbClient, artefactID, os.Stdout)
		if err != nil {
			if hoard.IsNotFound(err) {
				return printer.Error(
					fmt.Sprintf("artefact with ID '%s' not found", artefactID),
					"The specified artefact does not exist on the blackboard.",
					[]string{
						"List all artefacts:\n  sett hoard",
						fmt.Sprintf("Verify instance:\n  sett hoard --name %s", targetInstanceName),
					},
				)
			}
			return fmt.Errorf("failed to get artefact: %w", err)
		}
	} else {
		// List mode: fetch and display all artefacts
		err := hoard.ListArtefacts(ctx, bbClient, targetInstanceName, outputFormat, os.Stdout)
		if err != nil {
			return fmt.Errorf("failed to list artefacts: %w", err)
		}
	}

	return nil
}
