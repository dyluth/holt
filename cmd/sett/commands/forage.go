package commands

import (
	"context"
	"fmt"
	"os"
	"time"

	dockerpkg "github.com/dyluth/sett/internal/docker"
	"github.com/dyluth/sett/internal/git"
	"github.com/dyluth/sett/internal/instance"
	"github.com/dyluth/sett/internal/printer"
	"github.com/dyluth/sett/internal/watch"
	"github.com/dyluth/sett/pkg/blackboard"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
)

var (
	forageInstanceName string
	forageWatch        bool
	forageGoal         string
)

var forageCmd = &cobra.Command{
	Use:   "forage",
	Short: "Create a new workflow by submitting a goal",
	Long: `Create a new workflow by submitting a goal description.

The forage command creates a GoalDefined artefact on the blackboard,
which the orchestrator will process to begin coordinating agents.

Prerequisites:
  • Git repository with clean workspace (no uncommitted changes)
  • Running Sett instance (start with 'sett up')

Examples:
  # Create workflow on inferred instance
  sett forage --goal "Build a REST API for user management"

  # Target specific instance
  sett forage --name prod --goal "Refactor authentication module"

  # Validate orchestrator response (Phase 1)
  sett forage --watch --goal "Add logging to all endpoints"`,
	RunE: runForage,
}

func init() {
	forageCmd.Flags().StringVarP(&forageInstanceName, "name", "n", "", "Target instance name (auto-inferred if omitted)")
	forageCmd.Flags().BoolVarP(&forageWatch, "watch", "w", false, "Wait for orchestrator to create claim (Phase 1 validation)")
	forageCmd.Flags().StringVarP(&forageGoal, "goal", "g", "", "Goal description (required)")
	forageCmd.MarkFlagRequired("goal")
	rootCmd.AddCommand(forageCmd)
}

func runForage(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Phase 1: Validate goal input
	if forageGoal == "" {
		return printer.Error(
			"required flag --goal not provided",
			"Usage:\n  sett forage --goal \"description of what you want to build\"\n\nExample:\n  sett forage --goal \"Create a REST API for user management\"",
			[]string{"For immediate validation:\n  sett forage --watch --goal \"your goal\""},
		)
	}

	// Phase 2: Git workspace validation
	checker := git.NewChecker()

	isRepo, err := checker.IsGitRepository()
	if err != nil {
		return err
	}
	if !isRepo {
		return printer.Error(
			"not a Git repository",
			"Sett requires a Git repository to manage workflows.",
			[]string{"Initialize Git first:\n  git init\n  sett init\n  sett up"},
		)
	}

	isClean, err := checker.IsWorkspaceClean()
	if err != nil {
		return fmt.Errorf("failed to check Git workspace: %w", err)
	}
	if !isClean {
		dirtyFiles, err := checker.GetDirtyFiles()
		if err != nil {
			return fmt.Errorf("failed to get dirty files: %w", err)
		}

		return printer.Error(
			"Git workspace is not clean",
			dirtyFiles,
			[]string{
				"Commit changes:\n  git add .\n  git commit -m \"your message\"",
				"Stash temporarily:\n  git stash",
			},
		)
	}

	// Phase 3: Instance discovery
	cli, err := dockerpkg.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer cli.Close()

	targetInstanceName := forageInstanceName
	if targetInstanceName == "" {
		targetInstanceName, err = instance.InferInstanceFromWorkspace(ctx, cli)
		if err != nil {
			if err.Error() == "no Sett instances found for this workspace" {
				return printer.ErrorWithContext(
					"no Sett instances found",
					"No running instances found for workspace:",
					map[string]string{"Workspace": mustGetGitRoot()},
					[]string{
						"Start an instance first:\n  sett up",
						fmt.Sprintf("Then retry:\n  sett forage --goal \"%s\"", forageGoal),
					},
				)
			}
			if err.Error() == "multiple instances found for this workspace, use --name to specify which one" {
				return printer.Error(
					"multiple instances found",
					"Found multiple running instances for this workspace.",
					[]string{
						fmt.Sprintf("Specify which instance to use:\n  sett forage --name <instance-name> --goal \"%s\"", forageGoal),
						"List instances:\n  sett list",
					},
				)
			}
			return fmt.Errorf("failed to infer instance: %w", err)
		}
	}

	// Phase 4: Verify instance is running
	if err := instance.VerifyInstanceRunning(ctx, cli, targetInstanceName); err != nil {
		return printer.Error(
			fmt.Sprintf("instance '%s' is not running", targetInstanceName),
			fmt.Sprintf("Error: %v", err),
			[]string{
				fmt.Sprintf("Start the instance:\n  sett up --name %s", targetInstanceName),
				fmt.Sprintf("Or if stuck, restart:\n  sett down --name %s\n  sett up --name %s", targetInstanceName, targetInstanceName),
			},
		)
	}

	// Phase 5: Get Redis port
	redisPort, err := instance.GetInstanceRedisPort(ctx, cli, targetInstanceName)
	if err != nil {
		return printer.ErrorWithContext(
			"Redis port not found",
			fmt.Sprintf("Instance '%s' exists but Redis port label is missing.", targetInstanceName),
			map[string]string{
				"This may indicate": "Instance was created with older sett version\n  - Manual container manipulation",
			},
			[]string{
				fmt.Sprintf("Restart the instance:\n  sett down --name %s\n  sett up --name %s", targetInstanceName, targetInstanceName),
			},
		)
	}

	// Phase 6: Connect to blackboard
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

	// Phase 7: If --watch mode, start streaming BEFORE creating artefact to catch all events
	if forageWatch {
		printer.Info("Starting watch mode...\n")

		// Start streaming in a goroutine
		streamDone := make(chan error, 1)
		go func() {
			streamDone <- watch.StreamActivity(ctx, bbClient, targetInstanceName, watch.OutputFormatDefault, os.Stdout)
		}()

		// Give subscription time to set up before publishing artefact
		time.Sleep(100 * time.Millisecond)

		// Now create the artefact - all subsequent events will be captured
		artefactID := uuid.New().String()
		logicalID := uuid.New().String()

		artefact := &blackboard.Artefact{
			ID:              artefactID,
			LogicalID:       logicalID,
			Version:         1,
			StructuralType:  blackboard.StructuralTypeStandard,
			Type:            "GoalDefined",
			Payload:         forageGoal,
			SourceArtefacts: []string{},
			ProducedByRole:  "user",
		}

		if err := bbClient.CreateArtefact(ctx, artefact); err != nil {
			return fmt.Errorf("failed to create artefact: %w", err)
		}

		// Wait for streaming to complete (typically on Ctrl+C)
		return <-streamDone
	}

	// Non-watch mode: create artefact and return
	artefactID := uuid.New().String()
	logicalID := uuid.New().String()

	artefact := &blackboard.Artefact{
		ID:              artefactID,
		LogicalID:       logicalID,
		Version:         1,
		StructuralType:  blackboard.StructuralTypeStandard,
		Type:            "GoalDefined",
		Payload:         forageGoal,
		SourceArtefacts: []string{},
		ProducedByRole:  "user",
	}

	if err := bbClient.CreateArtefact(ctx, artefact); err != nil {
		return fmt.Errorf("failed to create artefact: %w", err)
	}

	printer.Success("Goal artefact created: %s\n", artefactID)

	printer.Info("\nNext steps:\n")
	printer.Info("  • Agents will process this goal in Phase 2+\n")
	printer.Info("  • View all artefacts: sett hoard --name %s\n", targetInstanceName)
	printer.Info("  • Monitor workflow: sett watch --name %s\n", targetInstanceName)

	return nil
}

func mustGetGitRoot() string {
	checker := git.NewChecker()
	root, err := checker.GetGitRoot()
	if err != nil {
		return "<unknown>"
	}
	return root
}
