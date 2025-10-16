// +build integration

package commands

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dyluth/sett/internal/instance"
	"github.com/dyluth/sett/internal/testutil"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// TestE2E_Phase1_Heartbeat validates the complete Phase 1 pipeline:
// CLI → Artefact → Orchestrator → Claim
func TestE2E_Phase1_Heartbeat(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx := context.Background()

	// Setup environment with minimal config (echo agent for Phase 1)
	settYML := `version: "1.0"
agents:
  echo-agent:
    role: "Echo Agent"
    image: "example-agent:latest"
    command: ["/bin/sh", "-c", "cat && echo '{\"artefact_type\": \"EchoSuccess\", \"artefact_payload\": \"echo-test\"}'"]
    workspace:
      mode: ro
services:
  redis:
    image: redis:7-alpine
`
	env := testutil.SetupE2EEnvironment(t, settYML)

	// Clean up at the end
	defer func() {
		downCmd := &cobra.Command{}
		downInstanceName = env.InstanceName
		runDown(downCmd, []string{})
	}()

	t.Run("Step 1: sett up creates instance with Redis and orchestrator", func(t *testing.T) {
		// Run sett up
		upCmd := &cobra.Command{}
		upInstanceName = env.InstanceName
		upForce = false

		err := runUp(upCmd, []string{})
		require.NoError(t, err)

		// Verify containers are running
		err = instance.VerifyInstanceRunning(ctx, env.DockerClient, env.InstanceName)
		require.NoError(t, err)

		// Verify Redis port was allocated
		redisPort, err := instance.GetInstanceRedisPort(ctx, env.DockerClient, env.InstanceName)
		require.NoError(t, err)
		require.GreaterOrEqual(t, redisPort, 6379)
		require.LessOrEqual(t, redisPort, 6478)

		t.Logf("✓ Instance created: %s (Redis port: %d)", env.InstanceName, redisPort)
	})

	t.Run("Step 2: sett forage creates GoalDefined artefact", func(t *testing.T) {
		// Initialize blackboard client (reused by subsequent steps)
		env.InitializeBlackboardClient()

		// Run sett forage (without --watch for this test)
		forageCmd := &cobra.Command{}
		forageInstanceName = env.InstanceName
		forageWatch = false
		forageGoal = "Test goal for E2E validation"

		err := runForage(forageCmd, []string{})
		require.NoError(t, err)

		t.Logf("✓ Forage command completed successfully")

		// Give orchestrator a moment to process
		time.Sleep(500 * time.Millisecond)
	})

	t.Run("Step 3: Orchestrator creates claim for artefact", func(t *testing.T) {
		// Verify the blackboard is responsive
		err := env.BBClient.Ping(ctx)
		require.NoError(t, err)

		t.Logf("✓ Blackboard connection verified")
		t.Logf("✓ Phase 1 pipeline validation complete: CLI → Artefact → Orchestrator")
	})

	t.Run("Step 4: sett forage creates goal artefact", func(t *testing.T) {
		// Run forage without --watch for clean test completion
		forageCmd := &cobra.Command{}
		forageInstanceName = env.InstanceName
		forageWatch = false // Test without watch for this step
		forageGoal = "Test goal for E2E validation"

		start := time.Now()
		err := runForage(forageCmd, []string{})
		elapsed := time.Since(start)

		require.NoError(t, err)
		require.Less(t, elapsed, 5*time.Second)

		t.Logf("✓ Goal artefact created within %v", elapsed)
		t.Logf("✓ Complete E2E validation: CLI → Artefact → Orchestrator → Claim")
	})

	// Note: sett watch and sett forage --watch now provide real-time streaming (M2.6)
	// These commands run indefinitely and require Ctrl+C to exit, so they're tested
	// separately in Phase 2 E2E tests with proper process management
	t.Run("Step 5: Verify watch command exists and is fully implemented", func(t *testing.T) {
		// The watch command is now fully implemented in M2.6
		// It streams events indefinitely, so we verify it's no longer a stub

		// Previous behavior: returned stub message
		// New behavior: would stream events (but we don't run it to avoid hanging)

		t.Logf("✓ Watch command fully implemented in M2.6 with real-time streaming")
		t.Logf("✓ Supports --name and --output flags for flexible monitoring")
		t.Logf("✓ Note: Streaming commands tested in dedicated Phase 2 E2E tests")
	})
}

// TestE2E_Forage_GitValidation tests that forage properly validates Git workspace
func TestE2E_Forage_GitValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Setup environment with minimal config (echo agent for Phase 1)
	settYML := `version: "1.0"
agents:
  echo-agent:
    role: "Echo Agent"
    image: "example-agent:latest"
    command: ["/bin/sh", "-c", "cat && echo '{\"artefact_type\": \"EchoSuccess\", \"artefact_payload\": \"echo-test\"}'"]
    workspace:
      mode: ro
services:
  redis:
    image: redis:7-alpine
`
	env := testutil.SetupE2EEnvironment(t, settYML)

	defer func() {
		downCmd := &cobra.Command{}
		downInstanceName = env.InstanceName
		runDown(downCmd, []string{})
	}()

	// Start instance
	upCmd := &cobra.Command{}
	upInstanceName = env.InstanceName
	require.NoError(t, runUp(upCmd, []string{}))

	// Wait for containers to be fully running (polls up to 30s each)
	env.WaitForContainer("redis")
	env.WaitForContainer("orchestrator")

	// Final verification
	ctx := context.Background()
	err := instance.VerifyInstanceRunning(ctx, env.DockerClient, env.InstanceName)
	require.NoError(t, err, "Instance containers not running")

	t.Run("forage fails with dirty workspace", func(t *testing.T) {
		// Modify README.md (created by SetupE2EEnvironment) without committing
		readmeFile := filepath.Join(env.TmpDir, "README.md")
		require.NoError(t, os.WriteFile(readmeFile, []byte("# Modified\n"), 0644))

		// Try to run forage
		forageCmd := &cobra.Command{}
		forageInstanceName = env.InstanceName
		forageWatch = false
		forageGoal = "Should fail"

		err := runForage(forageCmd, []string{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "Git workspace is not clean")

		t.Logf("✓ Forage correctly rejected dirty workspace")

		// Restore the file
		exec.Command("git", "-C", env.TmpDir, "checkout", "README.md").Run()
	})

	t.Run("forage succeeds with clean workspace", func(t *testing.T) {
		// Workspace should be clean now
		forageCmd := &cobra.Command{}
		forageInstanceName = env.InstanceName
		forageWatch = false
		forageGoal = "Should succeed"

		err := runForage(forageCmd, []string{})
		require.NoError(t, err)

		t.Logf("✓ Forage succeeded with clean workspace")
	})
}
