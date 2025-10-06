package commands

import (
	"context"
	"fmt"
	"time"

	dockerpkg "github.com/dyluth/sett/internal/docker"
	"github.com/dyluth/sett/internal/git"
	"github.com/dyluth/sett/internal/instance"
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
	forageCmd.Flags().StringVar(&forageInstanceName, "name", "", "Target instance name (auto-inferred if omitted)")
	forageCmd.Flags().BoolVar(&forageWatch, "watch", false, "Wait for orchestrator to create claim (Phase 1 validation)")
	forageCmd.Flags().StringVar(&forageGoal, "goal", "", "Goal description (required)")
	forageCmd.MarkFlagRequired("goal")
	rootCmd.AddCommand(forageCmd)
}

func runForage(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Phase 1: Validate goal input
	if forageGoal == "" {
		return fmt.Errorf(`required flag --goal not provided

Usage:
  sett forage --goal "description of what you want to build"

Example:
  sett forage --goal "Create a REST API for user management"

For immediate validation:
  sett forage --watch --goal "your goal"`)
	}

	// Phase 2: Git workspace validation
	checker := git.NewChecker()

	isRepo, err := checker.IsGitRepository()
	if err != nil {
		return err
	}
	if !isRepo {
		return fmt.Errorf(`not a Git repository

Sett requires a Git repository to manage workflows.

Initialize Git first:
  git init
  sett init
  sett up`)
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

		return fmt.Errorf(`Git workspace is not clean

%s

Please commit or stash changes before running sett forage:
  git add .
  git commit -m "your message"

Or to stash temporarily:
  git stash`, dirtyFiles)
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
				return fmt.Errorf(`no Sett instances found

No running instances found for workspace:
  %s

Start an instance first:
  sett up

Then retry:
  sett forage --goal "%s"`, mustGetGitRoot(), forageGoal)
			}
			if err.Error() == "multiple instances found for this workspace, use --name to specify which one" {
				return fmt.Errorf(`multiple instances found

Found multiple running instances for this workspace.

Specify which instance to use:
  sett forage --name <instance-name> --goal "%s"

List instances:
  sett list`, forageGoal)
			}
			return fmt.Errorf("failed to infer instance: %w", err)
		}
	}

	// Phase 4: Verify instance is running
	if err := instance.VerifyInstanceRunning(ctx, cli, targetInstanceName); err != nil {
		return fmt.Errorf(`instance '%s' is not running

Container exists but is stopped.

Start the instance:
  sett up --name %s

Or if stuck, restart:
  sett down --name %s
  sett up --name %s`, targetInstanceName, targetInstanceName, targetInstanceName, targetInstanceName)
	}

	// Phase 5: Get Redis port
	redisPort, err := instance.GetInstanceRedisPort(ctx, cli, targetInstanceName)
	if err != nil {
		return fmt.Errorf(`Redis port not found

Instance '%s' exists but Redis port label is missing.

This may indicate:
  - Instance was created with older sett version
  - Manual container manipulation

Restart the instance:
  sett down --name %s
  sett up --name %s`, targetInstanceName, targetInstanceName, targetInstanceName)
	}

	// Phase 6: Connect to blackboard
	redisURL := fmt.Sprintf("redis://localhost:%d", redisPort)
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
		return fmt.Errorf(`Redis connection failed

Could not connect to Redis at %s

Check Redis container status:
  docker logs sett-redis-%s

Restart if needed:
  sett down --name %s
  sett up --name %s`, redisURL, targetInstanceName, targetInstanceName, targetInstanceName)
	}

	// Phase 7: Create GoalDefined artefact
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

	fmt.Printf("✓ Goal artefact created: %s\n", artefactID)

	// Phase 8: Optionally wait for claim (--watch)
	if forageWatch {
		fmt.Printf("⏳ Waiting for orchestrator to create claim...\n")

		claim, err := watch.PollForClaim(ctx, bbClient, artefactID, 5*time.Second)
		if err != nil {
			return fmt.Errorf(`timeout waiting for claim

No claim created after 5 seconds.

Possible causes:
  - Orchestrator container not running
  - Orchestrator not subscribed to artefact_events
  - Redis Pub/Sub issue

Check orchestrator status:
  docker ps | grep orchestrator
  docker logs sett-orchestrator-%s

Check artefact was created:
  # Connect to Redis and verify
  redis-cli -p %d HGETALL sett:%s:artefact:%s`, targetInstanceName, redisPort, targetInstanceName, artefactID)
		}

		fmt.Printf("✓ Claim created: %s (status: %s)\n", claim.ID, claim.Status)
	}

	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  • Agents will process this goal in Phase 2+\n")
	fmt.Printf("  • View all artefacts: sett hoard --name %s\n", targetInstanceName)
	fmt.Printf("  • Monitor workflow: sett watch --name %s (Phase 2+)\n", targetInstanceName)

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
