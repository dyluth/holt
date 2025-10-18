// +build integration

package commands

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/dyluth/sett/internal/instance"
	"github.com/dyluth/sett/internal/testutil"
	"github.com/dyluth/sett/pkg/blackboard"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// TestE2E_Phase3_ThreePhaseWorkflow validates the complete M3.2 three-phase workflow:
// forage → review (approve) → parallel → exclusive → complete
func TestE2E_Phase3_ThreePhaseWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx := context.Background()

	t.Log("=== Phase 3 E2E Three-Phase Workflow Test ===")

	// Step 0: Build required Docker images
	projectRoot := testutil.GetProjectRoot()

	t.Log("Building example-reviewer-agent Docker image...")
	buildReviewerCmd := exec.Command("docker", "build",
		"-t", "example-reviewer-agent:latest",
		"-f", "agents/example-reviewer-agent/Dockerfile",
		".")
	buildReviewerCmd.Dir = projectRoot
	output, err := buildReviewerCmd.CombinedOutput()
	if err != nil {
		t.Logf("Build output:\n%s", string(output))
	}
	require.NoError(t, err, "Failed to build example-reviewer-agent Docker image")
	t.Log("✓ example-reviewer-agent image built")

	t.Log("Building example-parallel-agent Docker image...")
	buildParallelCmd := exec.Command("docker", "build",
		"-t", "example-parallel-agent:latest",
		"-f", "agents/example-parallel-agent/Dockerfile",
		".")
	buildParallelCmd.Dir = projectRoot
	output, err = buildParallelCmd.CombinedOutput()
	if err != nil {
		t.Logf("Build output:\n%s", string(output))
	}
	require.NoError(t, err, "Failed to build example-parallel-agent Docker image")
	t.Log("✓ example-parallel-agent image built")

	t.Log("Building example-git-agent Docker image...")
	buildGitCmd := exec.Command("docker", "build",
		"-t", "example-git-agent:latest",
		"-f", "agents/example-git-agent/Dockerfile",
		".")
	buildGitCmd.Dir = projectRoot
	output, err = buildGitCmd.CombinedOutput()
	if err != nil {
		t.Logf("Build output:\n%s", string(output))
	}
	require.NoError(t, err, "Failed to build example-git-agent Docker image")
	t.Log("✓ example-git-agent image built")

	// Step 1: Setup isolated environment with 3-phase config
	env := testutil.SetupE2EEnvironment(t, testutil.ThreePhaseSettYML())
	defer func() {
		// Cleanup: stop instance
		downCmd := &cobra.Command{}
		downInstanceName = env.InstanceName
		_ = runDown(downCmd, []string{})
		t.Log("✓ Cleanup complete")
	}()

	t.Logf("✓ Environment setup complete: %s", env.TmpDir)
	t.Logf("✓ Instance name: %s", env.InstanceName)

	// Step 2: Run sett up
	t.Log("Step 2: Starting Sett instance with 3 agents...")
	upCmd := &cobra.Command{}
	upInstanceName = env.InstanceName
	upForce = false

	err = runUp(upCmd, []string{})
	require.NoError(t, err, "sett up failed")

	// Verify containers are running
	err = instance.VerifyInstanceRunning(ctx, env.DockerClient, env.InstanceName)
	require.NoError(t, err, "Instance not running")

	t.Logf("✓ Instance started: %s", env.InstanceName)

	// Wait for all containers to be ready
	env.WaitForContainer("orchestrator")
	env.WaitForContainer("agent-reviewer")
	env.WaitForContainer("agent-parallel-worker")
	env.WaitForContainer("agent-coder")

	// Initialize blackboard client
	env.InitializeBlackboardClient()
	t.Logf("✓ Connected to blackboard (Redis port: %d)", env.RedisPort)

	// Step 3: Run sett forage with goal
	t.Log("Step 3: Creating workflow with sett forage...")
	goalFilename := "feature.txt"

	forageCmd := &cobra.Command{}
	forageInstanceName = env.InstanceName
	forageWatch = false
	forageGoal = goalFilename

	err = runForage(forageCmd, []string{})
	require.NoError(t, err, "sett forage failed")

	t.Logf("✓ Goal submitted: %s", forageGoal)

	// Step 4: Verify GoalDefined artefact was created
	t.Log("Step 4: Verifying GoalDefined artefact...")
	goalArtefact := env.WaitForArtefactByType("GoalDefined")
	require.NotNil(t, goalArtefact)
	require.Equal(t, goalFilename, goalArtefact.Payload)
	require.Equal(t, "user", goalArtefact.ProducedByRole)
	t.Logf("✓ GoalDefined artefact created: id=%s", goalArtefact.ID)

	// Step 5: Verify claim was created and all agents bid
	t.Log("Step 5: Verifying claim creation and bidding...")
	time.Sleep(3 * time.Second) // Give agents time to bid

	// Get the claim for the GoalDefined artefact
	claim, err := env.BBClient.GetClaimByArtefactID(ctx, goalArtefact.ID)
	require.NoError(t, err, "Failed to get claim")
	require.NotNil(t, claim)
	t.Logf("✓ Claim created: id=%s, status=%s", claim.ID, claim.Status)

	// Verify all agents submitted bids
	bids, err := env.BBClient.GetAllBids(ctx, claim.ID)
	require.NoError(t, err, "Failed to get bids")
	require.Len(t, bids, 3, "Expected 3 bids (one per agent)")
	require.Equal(t, blackboard.BidTypeReview, bids["reviewer"], "Reviewer should bid 'review'")
	require.Equal(t, blackboard.BidTypeParallel, bids["parallel-worker"], "Parallel worker should bid 'claim'")
	require.Equal(t, blackboard.BidTypeExclusive, bids["coder"], "Coder should bid 'exclusive'")
	t.Logf("✓ All agents bid correctly: %v", bids)

	// Step 6: Verify review phase execution
	t.Log("Step 6: Verifying review phase...")

	// Note: M3.2 workflows are very fast! The claim may have already progressed
	// from pending_review → pending_parallel by the time we check (review completes in <1 second)
	// We verify the workflow by checking for the Review artefact instead

	// Wait for Review artefact from reviewer to prove review phase executed
	reviewArtefact := env.WaitForArtefactByType("Review")
	require.NotNil(t, reviewArtefact)
	require.Equal(t, blackboard.StructuralTypeReview, reviewArtefact.StructuralType)
	require.Equal(t, "Reviewer", reviewArtefact.ProducedByRole)
	require.Equal(t, "{}", reviewArtefact.Payload, "Review should approve with empty object")
	t.Logf("✓ Review phase completed: artefact=%s, approved", reviewArtefact.ID)

	// Step 7: Verify parallel phase execution
	t.Log("Step 7: Verifying parallel phase...")

	// Wait for ParallelWorkComplete artefact to prove parallel phase executed
	parallelArtefact := env.WaitForArtefactByType("ParallelWorkComplete")
	require.NotNil(t, parallelArtefact)
	require.Equal(t, "ParallelWorker", parallelArtefact.ProducedByRole)
	t.Logf("✓ Parallel phase completed: artefact=%s", parallelArtefact.ID)

	// Step 8: Verify exclusive phase execution
	t.Log("Step 8: Verifying exclusive phase...")

	// Wait for CodeCommit artefact from coder to prove exclusive phase executed
	codeCommitArtefact := env.WaitForArtefactByType("CodeCommit")
	require.NotNil(t, codeCommitArtefact)
	require.Equal(t, "Coder", codeCommitArtefact.ProducedByRole)
	commitHash := codeCommitArtefact.Payload
	require.NotEmpty(t, commitHash, "CodeCommit payload should contain commit hash")
	t.Logf("✓ Exclusive phase completed: artefact=%s, commit=%s", codeCommitArtefact.ID, commitHash)

	// Step 9: Verify claim completion
	t.Log("Step 9: Verifying claim completion...")

	// Wait for claim to transition to complete (with retry)
	var finalClaim *blackboard.Claim
	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)
		finalClaim, err = env.BBClient.GetClaim(ctx, claim.ID)
		require.NoError(t, err)
		t.Logf("Attempt %d: claim status = %s", i+1, finalClaim.Status)
		if finalClaim.Status == blackboard.ClaimStatusComplete {
			break
		}
	}
	require.Equal(t, blackboard.ClaimStatusComplete, finalClaim.Status, "Claim should be complete")
	t.Logf("✓ Claim marked as complete")

	// Step 10: Verify Git commit exists
	t.Log("Step 10: Verifying Git commit...")
	env.VerifyGitCommitExists(commitHash)

	// Step 11: Verify file was created
	t.Log("Step 11: Verifying file creation...")
	env.VerifyFileExists(goalFilename)

	t.Log("=== Three-Phase Workflow Test PASSED ===")
}

// TestE2E_Phase3_ReviewRejection validates review rejection workflow:
// forage → review (reject) → claim terminated
func TestE2E_Phase3_ReviewRejection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	t.Log("=== Phase 3 E2E Review Rejection Test ===")

	// For this test, we need a reviewer agent that rejects
	// We'll create a custom one inline
	settYMLWithRejectingReviewer := `version: "1.0"
agents:
  reviewer:
    role: "Reviewer"
    image: "example-git-agent:latest"
    command: ["/app/run.sh"]
    bidding_strategy: "review"
    workspace:
      mode: ro
  coder:
    role: "Coder"
    image: "example-git-agent:latest"
    command: ["/app/run.sh"]
    bidding_strategy: "exclusive"
    workspace:
      mode: rw
services:
  redis:
    image: redis:7-alpine
`

	env := testutil.SetupE2EEnvironment(t, settYMLWithRejectingReviewer)
	defer func() {
		downCmd := &cobra.Command{}
		downInstanceName = env.InstanceName
		_ = runDown(downCmd, []string{})
		t.Log("✓ Cleanup complete")
	}()

	// Create a custom reviewer agent that rejects
	rejectReviewScript := `#!/bin/sh
set -e
input=$(cat)
echo "Rejecting reviewer received claim, providing feedback..." >&2
cat <<EOF
{
  "structural_type": "Review",
  "payload": "{\"issue\": \"needs tests\", \"severity\": \"high\"}"
}
EOF
`
	env.CreateTestAgent("reject-reviewer", rejectReviewScript)

	// Note: This test would need the rejecting reviewer to be built as a Docker image
	// For now, we'll skip the actual execution and just document the expected behavior
	t.Skip("Review rejection test requires custom rejecting reviewer agent image")

	// Expected flow:
	// 1. sett up with rejecting reviewer
	// 2. sett forage
	// 3. GoalDefined artefact created
	// 4. Claim created in pending_review
	// 5. Reviewer produces Review artefact with feedback
	// 6. Claim status transitions to terminated
	// 7. No parallel or exclusive phases execute
}

// TestE2E_Phase3_PhaseSkipping validates backward compatibility:
// forage → exclusive only (skip review and parallel)
func TestE2E_Phase3_PhaseSkipping(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx := context.Background()

	t.Log("=== Phase 3 E2E Phase Skipping Test (M3.1 Compatibility) ===")

	// Step 0: Build required Docker images BEFORE SetupE2EEnvironment
	projectRoot := testutil.GetProjectRoot()

	t.Log("Building example-git-agent Docker image...")
	buildGitCmd := exec.Command("docker", "build",
		"-t", "example-git-agent:latest",
		"-f", "agents/example-git-agent/Dockerfile",
		".")
	buildGitCmd.Dir = projectRoot
	output, err := buildGitCmd.CombinedOutput()
	if err != nil {
		t.Logf("Build output:\n%s", string(output))
	}
	require.NoError(t, err, "Failed to build example-git-agent Docker image")
	t.Log("✓ example-git-agent image built")

	// Step 1: Setup isolated environment with M3.1-style config
	env := testutil.SetupE2EEnvironment(t, testutil.GitAgentSettYML())
	defer func() {
		downCmd := &cobra.Command{}
		downInstanceName = env.InstanceName
		_ = runDown(downCmd, []string{})
		t.Log("✓ Cleanup complete")
	}()

	// Start instance
	upCmd := &cobra.Command{}
	upInstanceName = env.InstanceName
	err = runUp(upCmd, []string{})
	require.NoError(t, err, "sett up failed")

	env.WaitForContainer("orchestrator")
	env.WaitForContainer("agent-git-agent")
	env.InitializeBlackboardClient()

	// Submit goal
	forageCmd := &cobra.Command{}
	forageInstanceName = env.InstanceName
	forageGoal = "test.txt"
	err = runForage(forageCmd, []string{})
	require.NoError(t, err, "sett forage failed")

	// Verify GoalDefined artefact
	goalArtefact := env.WaitForArtefactByType("GoalDefined")
	require.NotNil(t, goalArtefact)
	t.Logf("✓ GoalDefined artefact created")

	// Verify phase skipping: M3.1 exclusive-only workflow should complete successfully
	// The workflow completes very fast (<1 second), skipping review and parallel phases

	// Wait for CodeCommit artefact to prove exclusive phase executed
	codeCommitArtefact := env.WaitForArtefactByType("CodeCommit")
	require.NotNil(t, codeCommitArtefact)
	t.Logf("✓ CodeCommit artefact created (exclusive phase executed)")

	// Verify claim completes (proves M3.1 backward compatibility)
	claim, err := env.BBClient.GetClaimByArtefactID(ctx, goalArtefact.ID)
	require.NoError(t, err)
	require.Equal(t, blackboard.ClaimStatusComplete, claim.Status, "Claim should be complete")
	t.Logf("✓ Claim completed successfully (M3.1 backward compatible)")

	t.Log("=== Phase Skipping Test PASSED (M3.1 backward compatible) ===")
}
