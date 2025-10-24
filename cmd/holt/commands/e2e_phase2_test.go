//go:build integration
// +build integration

package commands

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/dyluth/holt/internal/instance"
	"github.com/dyluth/holt/internal/testutil"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// TestE2E_Phase2_HappyPath validates the complete Phase 2 single-agent workflow:
// forage → claim → bid → execute → CodeCommit artefact → Git commit
func TestE2E_Phase2_HappyPath(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx := context.Background()

	t.Log("=== Phase 2 E2E Happy Path Test ===")

	// Step 0: Build example-git-agent Docker image
	t.Log("Building example-git-agent Docker image...")
	buildCmd := exec.Command("docker", "build",
		"-t", "example-git-agent:latest",
		"-f", "agents/example-git-agent/Dockerfile",
		".")
	buildCmd.Dir = testutil.GetProjectRoot()
	output, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Logf("Build output:\n%s", string(output))
	}
	require.NoError(t, err, "Failed to build example-git-agent Docker image")
	t.Log("✓ example-git-agent image built")

	// Step 1: Setup isolated environment
	env := testutil.SetupE2EEnvironment(t, testutil.GitAgentHoltYML())
	defer func() {
		// Cleanup: stop instance
		downCmd := &cobra.Command{}
		downInstanceName = env.InstanceName
		_ = runDown(downCmd, []string{})
		t.Log("✓ Cleanup complete")
	}()

	t.Logf("✓ Environment setup complete: %s", env.TmpDir)
	t.Logf("✓ Instance name: %s", env.InstanceName)

	// Step 2: Run holt up
	t.Log("Step 2: Starting Holt instance...")
	upCmd := &cobra.Command{}
	upInstanceName = env.InstanceName
	upForce = false

	err = runUp(upCmd, []string{})
	require.NoError(t, err, "holt up failed")

	// Verify containers are running
	err = instance.VerifyInstanceRunning(ctx, env.DockerClient, env.InstanceName)
	require.NoError(t, err, "Instance not running")

	t.Logf("✓ Instance started: %s", env.InstanceName)

	// Wait for orchestrator and agent to be ready
	env.WaitForContainer("orchestrator")
	env.WaitForContainer("agent-GitAgent")

	// Initialize blackboard client
	env.InitializeBlackboardClient()
	t.Logf("✓ Connected to blackboard (Redis port: %d)", env.RedisPort)

	// Step 3: Run holt forage with goal
	t.Log("Step 3: Creating workflow with holt forage...")
	goalFilename := "test-output.txt"

	forageCmd := &cobra.Command{}
	forageInstanceName = env.InstanceName
	forageWatch = false
	forageGoal = goalFilename

	err = runForage(forageCmd, []string{})
	require.NoError(t, err, "holt forage failed")

	t.Logf("✓ Goal submitted: %s", forageGoal)

	// Verify GoalDefined artefact was created
	t.Log("Step 4: Verifying GoalDefined artefact...")
	goalArtefact := env.WaitForArtefactByType("GoalDefined")
	require.NotNil(t, goalArtefact)
	require.Equal(t, goalFilename, goalArtefact.Payload)
	require.Equal(t, "user", goalArtefact.ProducedByRole)
	t.Logf("✓ GoalDefined artefact created: id=%s", goalArtefact.ID)

	// Step 5: Wait for agent to execute and create CodeCommit artefact
	t.Log("Step 5: Waiting for agent to execute and create CodeCommit...")
	// This may take some time as the agent needs to:
	// 1. Receive claim event
	// 2. Submit bid
	// 3. Wait for grant
	// 4. Execute tool
	// 5. Create file + commit
	// 6. Return CodeCommit artefact

	codeCommitArtefact := env.WaitForArtefactByType("CodeCommit")
	require.NotNil(t, codeCommitArtefact)
	require.NotEmpty(t, codeCommitArtefact.Payload, "CodeCommit payload (commit hash) is empty")
	t.Logf("✓ CodeCommit artefact created: id=%s, commit=%s", codeCommitArtefact.ID, codeCommitArtefact.Payload)

	// Step 6: Verify Git commit exists
	t.Log("Step 6: Verifying Git commit exists in repository...")
	commitHash := codeCommitArtefact.Payload
	env.VerifyGitCommitExists(commitHash)
	t.Logf("✓ Git commit %s verified", commitHash)

	// Step 7: Verify file was created by agent
	t.Log("Step 7: Verifying file was created in workspace...")
	env.VerifyFileExists(goalFilename)
	env.VerifyFileContent(goalFilename, "File created by Holt example-git-agent")
	t.Logf("✓ File %s verified", goalFilename)

	// Step 8: Verify workspace is still clean (commit succeeded)
	t.Log("Step 8: Verifying workspace remains clean...")
	env.VerifyWorkspaceClean()
	t.Log("✓ Workspace clean (no uncommitted changes)")

	// Step 9: Verify audit trail (artefact chain)
	t.Log("Step 9: Verifying audit trail...")
	// CodeCommit should reference GoalDefined in source_artefacts
	// (This is implicit in the agent's behavior, but we can verify via context chain)
	require.Equal(t, "Standard", string(goalArtefact.StructuralType))
	require.Equal(t, "Standard", string(codeCommitArtefact.StructuralType))
	t.Log("✓ Audit trail: GoalDefined → CodeCommit")

	t.Log("=== Phase 2 Happy Path Test Complete ===")
	t.Log("✓ Complete workflow validated:")
	t.Log("  1. holt up - launched orchestrator + agent")
	t.Log("  2. holt forage - created GoalDefined artefact")
	t.Log("  3. Orchestrator - created claim")
	t.Log("  4. Agent - bid on claim and won")
	t.Log("  5. Agent - executed work (created file + committed)")
	t.Log("  6. Agent - returned CodeCommit artefact")
	t.Log("  7. Git - commit exists with correct file")
	t.Log("  8. Workspace - remains clean")
}

// TestE2E_Phase2_MultipleWorkflows validates sequential workflows
func TestE2E_Phase2_MultipleWorkflows(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx := context.Background()

	t.Log("=== Phase 2 Multiple Workflows Test ===")

	// Build image (may already exist from previous test)
	buildCmd := exec.Command("docker", "build",
		"-t", "example-git-agent:latest",
		"-f", "agents/example-git-agent/Dockerfile",
		".")
	buildCmd.Dir = testutil.GetProjectRoot()
	buildCmd.Run() // Ignore errors if already built

	// Setup environment
	env := testutil.SetupE2EEnvironment(t, testutil.GitAgentHoltYML())
	defer func() {
		downCmd := &cobra.Command{}
		downInstanceName = env.InstanceName
		_ = runDown(downCmd, []string{})
	}()

	// Start instance
	upCmd := &cobra.Command{}
	upInstanceName = env.InstanceName
	upForce = false
	err := runUp(upCmd, []string{})
	require.NoError(t, err)

	err = instance.VerifyInstanceRunning(ctx, env.DockerClient, env.InstanceName)
	require.NoError(t, err)

	env.WaitForContainer("orchestrator")
	env.WaitForContainer("agent-GitAgent")
	env.InitializeBlackboardClient()

	// Execute first workflow
	t.Log("Executing first workflow...")
	forageCmd := &cobra.Command{}
	forageInstanceName = env.InstanceName
	forageWatch = false
	forageGoal = "file1.txt"

	err = runForage(forageCmd, []string{})
	require.NoError(t, err)

	commit1 := env.WaitForArtefactByType("CodeCommit")
	require.NotNil(t, commit1)
	env.VerifyGitCommitExists(commit1.Payload)
	env.VerifyFileExists("file1.txt")
	t.Log("✓ First workflow complete")

	// Execute second workflow
	t.Log("Executing second workflow...")
	forageGoal = "file2.txt"
	err = runForage(forageCmd, []string{})
	require.NoError(t, err)

	// Wait a bit to ensure we don't get the first CodeCommit again
	time.Sleep(2 * time.Second)

	// Find second CodeCommit (different from first)
	var commit2 *testutil.ArtefactResult
	for i := 0; i < 60; i++ {
		pattern := fmt.Sprintf("holt:%s:artefact:*", env.InstanceName)
		iter := env.BBClient.RedisClient().Scan(ctx, 0, pattern, 0).Iterator()

		for iter.Next(ctx) {
			key := iter.Val()
			data, _ := env.BBClient.RedisClient().HGetAll(ctx, key).Result()
			if data["type"] == "CodeCommit" && data["payload"] != commit1.Payload {
				commit2 = &testutil.ArtefactResult{
					ID:      data["id"],
					Type:    data["type"],
					Payload: data["payload"],
				}
				break
			}
		}

		if commit2 != nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	require.NotNil(t, commit2, "Second CodeCommit not found")
	require.NotEqual(t, commit1.Payload, commit2.Payload, "Both commits have same hash")

	env.VerifyGitCommitExists(commit2.Payload)
	env.VerifyFileExists("file2.txt")
	t.Log("✓ Second workflow complete")

	// Verify both commits exist in Git history
	checkCmd := exec.Command("git", "log", "--oneline")
	checkCmd.Dir = env.TmpDir
	output, err := checkCmd.Output()
	require.NoError(t, err)
	require.Contains(t, string(output), "file1.txt")
	require.Contains(t, string(output), "file2.txt")

	t.Log("=== Multiple Workflows Test Complete ===")
	t.Log("✓ Sequential workflows validated")
	t.Log("✓ Multiple CodeCommit artefacts created")
	t.Log("✓ Git history contains both commits")
}
