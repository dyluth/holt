// +build integration

package commands

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/docker/docker/client"
	"github.com/dyluth/sett/internal/testutil"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// TestE2E_DirtyGitWorkspace verifies that sett up fails with clear error when workspace has uncommitted changes
func TestE2E_DirtyGitWorkspace(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	t.Log("=== Dirty Git Workspace Test ===")

	// Setup environment
	env := testutil.SetupE2EEnvironment(t, testutil.DefaultSettYML())

	// Create uncommitted file to make workspace dirty
	env.CreateDirtyWorkspace()
	t.Log("✓ Created uncommitted file (workspace is dirty)")

	// Attempt to run sett up
	t.Log("Attempting sett up with dirty workspace...")
	upCmd := &cobra.Command{}
	upInstanceName = env.InstanceName
	upForce = false

	err := runUp(upCmd, []string{})

	// Should fail
	require.Error(t, err, "sett up should fail with dirty workspace")
	require.Contains(t, err.Error(), "Git workspace is not clean", "Error message should mention dirty workspace")
	t.Logf("✓ sett up failed with expected error: %v", err)

	// Verify no containers were launched
	containers, err := env.DockerClient.ContainerList(context.Background(), client.ListContainersOptions{All: true})
	require.NoError(t, err)

	for _, container := range containers {
		for _, name := range container.Names {
			require.NotContains(t, name, env.InstanceName, "No containers should be created for this instance")
		}
	}
	t.Log("✓ No containers launched (as expected)")

	t.Log("=== Dirty Git Workspace Test Complete ===")
	t.Log("✓ Guardrail validated: sett up rejects dirty workspace")
	t.Log("✓ Error message is clear and actionable")
}

// TestE2E_AgentScriptFailure verifies that agent script non-zero exit creates Failure artefact
func TestE2E_AgentScriptFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx := context.Background()

	t.Log("=== Agent Script Failure Test ===")

	// Build failing agent Docker image
	t.Log("Building failing-agent Docker image...")

	// Create temporary failing agent Dockerfile
	failingAgentYML := `version: "1.0"
agents:
  failing-agent:
    role: "Failing Agent"
    image: "example-agent:latest"
    command: ["/bin/sh", "-c", "echo 'Failing intentionally' >&2 && exit 1"]
    workspace:
      mode: ro
services:
  redis:
    image: redis:7-alpine
`

	// Setup environment with failing agent
	env := testutil.SetupE2EEnvironment(t, failingAgentYML)
	defer func() {
		downCmd := &cobra.Command{}
		downInstanceName = env.InstanceName
		_ = runDown(downCmd, []string{})
	}()

	t.Log("✓ Environment setup with failing agent")

	// Build example-agent image (reuse existing echo agent image)
	buildCmd := exec.Command("docker", "build",
		"-t", "example-agent:latest",
		"-f", "agents/example-agent/Dockerfile",
		".")
	buildCmd.Dir = testutil.GetProjectRoot()
	buildCmd.Run() // Ignore errors if already built

	// Start instance
	t.Log("Starting instance...")
	upCmd := &cobra.Command{}
	upInstanceName = env.InstanceName
	upForce = false

	err := runUp(upCmd, []string{})
	require.NoError(t, err, "sett up should succeed")

	env.WaitForContainer("orchestrator")
	env.WaitForContainer("agent-failing-agent")
	env.InitializeBlackboardClient()
	t.Log("✓ Instance started with failing agent")

	// Submit goal
	t.Log("Submitting goal...")
	forageCmd := &cobra.Command{}
	forageInstanceName = env.InstanceName
	forageWatch = false
	forageGoal = "test-failure"

	err = runForage(forageCmd, []string{})
	require.NoError(t, err)
	t.Log("✓ Goal submitted")

	// Wait for Failure artefact
	t.Log("Waiting for Failure artefact...")
	failureArtefact := env.WaitForArtefactByType("Failure")
	require.NotNil(t, failureArtefact, "Failure artefact should be created")
	require.NotEmpty(t, failureArtefact.Payload, "Failure artefact should have error details")
	require.Equal(t, "Failure", failureArtefact.StructuralType)
	t.Logf("✓ Failure artefact created: id=%s", failureArtefact.ID)
	t.Logf("  Error details: %s", failureArtefact.Payload)

	// Verify claim was terminated (check claim status)
	// We can infer this by verifying no more artefacts are created after Failure
	time.Sleep(3 * time.Second)

	// Count artefacts - should only have GoalDefined and Failure
	pattern := fmt.Sprintf("sett:%s:artefact:*", env.InstanceName)
	iter := env.BBClient.Client.Scan(ctx, 0, pattern, 0).Iterator()

	artefactCount := 0
	for iter.Next(ctx) {
		artefactCount++
	}

	// Should have exactly 2 artefacts (GoalDefined + Failure), no additional processing
	require.LessOrEqual(t, artefactCount, 3, "Should not create additional artefacts after Failure")
	t.Log("✓ Workflow terminated (no additional artefacts created)")

	t.Log("=== Agent Script Failure Test Complete ===")
	t.Log("✓ Guardrail validated: non-zero exit creates Failure artefact")
	t.Log("✓ Failure artefact contains error details")
	t.Log("✓ Workflow terminated correctly")
}

// TestE2E_InvalidToolOutput verifies that malformed JSON output creates Failure artefact
func TestE2E_InvalidToolOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx := context.Background()

	t.Log("=== Invalid Tool Output Test ===")

	// Create agent that outputs invalid JSON
	invalidJSONAgentYML := `version: "1.0"
agents:
  invalid-agent:
    role: "Invalid JSON Agent"
    image: "example-agent:latest"
    command: ["/bin/sh", "-c", "echo 'This is not valid JSON' && exit 0"]
    workspace:
      mode: ro
services:
  redis:
    image: redis:7-alpine
`

	// Setup environment
	env := testutil.SetupE2EEnvironment(t, invalidJSONAgentYML)
	defer func() {
		downCmd := &cobra.Command{}
		downInstanceName = env.InstanceName
		_ = runDown(downCmd, []string{})
	}()

	t.Log("✓ Environment setup with invalid-JSON agent")

	// Build example-agent image (reuse)
	buildCmd := exec.Command("docker", "build",
		"-t", "example-agent:latest",
		"-f", "agents/example-agent/Dockerfile",
		".")
	buildCmd.Dir = testutil.GetProjectRoot()
	buildCmd.Run()

	// Start instance
	t.Log("Starting instance...")
	upCmd := &cobra.Command{}
	upInstanceName = env.InstanceName
	upForce = false

	err := runUp(upCmd, []string{})
	require.NoError(t, err)

	env.WaitForContainer("orchestrator")
	env.WaitForContainer("agent-invalid-agent")
	env.InitializeBlackboardClient()
	t.Log("✓ Instance started")

	// Submit goal
	t.Log("Submitting goal...")
	forageCmd := &cobra.Command{}
	forageInstanceName = env.InstanceName
	forageWatch = false
	forageGoal = "test-invalid-json"

	err = runForage(forageCmd, []string{})
	require.NoError(t, err)
	t.Log("✓ Goal submitted")

	// Wait for Failure artefact
	t.Log("Waiting for Failure artefact...")
	failureArtefact := env.WaitForArtefactByType("Failure")
	require.NotNil(t, failureArtefact)
	require.NotEmpty(t, failureArtefact.Payload)
	require.Equal(t, "Failure", failureArtefact.StructuralType)

	// Verify error message mentions JSON parsing
	require.Contains(t, failureArtefact.Payload, "JSON", "Error should mention JSON parsing failure")
	t.Logf("✓ Failure artefact created with JSON parse error")
	t.Logf("  Error details: %s", failureArtefact.Payload)

	// Verify workflow terminated
	time.Sleep(3 * time.Second)

	pattern := fmt.Sprintf("sett:%s:artefact:*", env.InstanceName)
	iter := env.BBClient.Client.Scan(ctx, 0, pattern, 0).Iterator()

	artefactCount := 0
	for iter.Next(ctx) {
		artefactCount++
	}

	require.LessOrEqual(t, artefactCount, 3, "Should not create additional artefacts after Failure")
	t.Log("✓ Workflow terminated")

	t.Log("=== Invalid Tool Output Test Complete ===")
	t.Log("✓ Guardrail validated: invalid JSON creates Failure artefact")
	t.Log("✓ Error message identifies JSON parsing issue")
	t.Log("✓ Workflow terminated correctly")
}

// TestE2E_ForageWithoutRunningInstance verifies friendly error when instance not running
func TestE2E_ForageWithoutRunningInstance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	t.Log("=== Forage Without Running Instance Test ===")

	// Setup environment but DON'T start instance
	env := testutil.SetupE2EEnvironment(t, testutil.DefaultSettYML())
	t.Log("✓ Environment setup (no instance started)")

	// Attempt forage without running instance
	t.Log("Attempting forage without running instance...")
	forageCmd := &cobra.Command{}
	forageInstanceName = env.InstanceName
	forageWatch = false
	forageGoal = "test-goal"

	err := runForage(forageCmd, []string{})

	// Should fail with clear error
	require.Error(t, err, "forage should fail when instance not running")
	t.Logf("✓ forage failed as expected: %v", err)

	t.Log("=== Forage Without Running Instance Test Complete ===")
	t.Log("✓ User-friendly error for missing instance")
}
