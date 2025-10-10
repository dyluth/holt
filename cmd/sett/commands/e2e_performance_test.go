// +build integration

package commands

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/dyluth/sett/internal/instance"
	"github.com/dyluth/sett/internal/testutil"
	"github.com/dyluth/sett/pkg/blackboard"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// TestPerformance_Startup measures sett up duration
func TestPerformance_Startup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	t.Log("=== Performance Test: Startup ===")

	// Setup environment
	env := testutil.SetupE2EEnvironment(t, testutil.DefaultSettYML())
	defer func() {
		downCmd := &cobra.Command{}
		downInstanceName = env.InstanceName
		_ = runDown(downCmd, []string{})
	}()

	// Measure sett up duration
	t.Log("Measuring sett up duration...")
	startTime := time.Now()

	upCmd := &cobra.Command{}
	upInstanceName = env.InstanceName
	upForce = false

	err := runUp(upCmd, []string{})
	require.NoError(t, err, "sett up failed")

	duration := time.Since(startTime)
	t.Logf("✓ sett up completed in: %v", duration)

	// Verify containers are running
	err = instance.VerifyInstanceRunning(context.Background(), env.DockerClient, env.InstanceName)
	require.NoError(t, err)

	// Assert threshold
	threshold := 10 * time.Second
	if duration > threshold {
		t.Errorf("❌ PERFORMANCE REGRESSION: sett up took %v, threshold is %v", duration, threshold)
	} else {
		t.Logf("✓ Performance requirement met: %v < %v", duration, threshold)
	}

	t.Log("=== Performance Test: Startup Complete ===")
}

// TestPerformance_ClaimToExecution measures latency from claim creation to agent execution start
func TestPerformance_ClaimToExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	ctx := context.Background()

	t.Log("=== Performance Test: Claim-to-Execution Latency ===")

	// Build example-agent
	buildCmd := exec.Command("docker", "build",
		"-t", "example-agent:latest",
		"-f", "agents/example-agent/Dockerfile",
		".")
	buildCmd.Dir = testutil.GetProjectRoot()
	buildCmd.Run()

	// Setup environment with echo agent
	env := testutil.SetupE2EEnvironment(t, testutil.EchoAgentSettYML())
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

	env.WaitForContainer("orchestrator")
	env.WaitForContainer("agent-echo-agent")
	env.InitializeBlackboardClient()

	t.Log("✓ Instance ready, submitting goal...")

	// Measure time from goal submission to artefact creation
	startTime := time.Now()

	forageCmd := &cobra.Command{}
	forageInstanceName = env.InstanceName
	forageWatch = false
	forageGoal = "performance-test"

	err = runForage(forageCmd, []string{})
	require.NoError(t, err)

	t.Log("✓ Goal submitted, waiting for agent execution...")

	// Wait for EchoSuccess artefact (indicates agent executed)
	echoArtefact := env.WaitForArtefactByType("EchoSuccess")
	require.NotNil(t, echoArtefact)

	duration := time.Since(startTime)
	t.Logf("✓ Agent execution completed in: %v", duration)

	// Assert threshold (claim creation + bidding + granting + execution)
	threshold := 2 * time.Second
	if duration > threshold {
		t.Errorf("❌ PERFORMANCE REGRESSION: Claim-to-execution took %v, threshold is %v", duration, threshold)
	} else {
		t.Logf("✓ Performance requirement met: %v < %v", duration, threshold)
	}

	t.Log("=== Performance Test: Claim-to-Execution Complete ===")
}

// TestPerformance_ContextAssembly measures context assembly time for deep artefact graph
func TestPerformance_ContextAssembly(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	ctx := context.Background()

	t.Log("=== Performance Test: Context Assembly ===")

	// Setup environment
	env := testutil.SetupE2EEnvironment(t, testutil.DefaultSettYML())
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

	env.WaitForContainer("orchestrator")
	env.InitializeBlackboardClient()

	t.Log("✓ Instance ready, creating 10-level artefact chain...")

	// Create a 10-level deep artefact chain manually
	var previousArtefactID string
	artefactIDs := make([]string, 10)

	for i := 0; i < 10; i++ {
		artefact := &blackboard.Artefact{
			ID:              uuid.NewString(),
			LogicalID:       uuid.NewString(),
			Version:         1,
			StructuralType:  "Standard",
			Type:            fmt.Sprintf("Level%d", i),
			Payload:         fmt.Sprintf("content-level-%d", i),
			ProducedByRole:  "test",
			SourceArtefacts: []string{},
		}

		if i > 0 {
			artefact.SourceArtefacts = []string{previousArtefactID}
		}

		// Store artefact on blackboard
		err := env.BBClient.CreateArtefact(ctx, artefact)
		require.NoError(t, err)

		artefactIDs[i] = artefact.ID
		previousArtefactID = artefact.ID
	}

	t.Log("✓ Created 10-level artefact chain")

	// Measure context assembly time
	// Note: This is a proxy test - we measure reading and traversing the chain
	// In a real scenario, the cub would do this during execution
	t.Log("Measuring context traversal time...")
	startTime := time.Now()

	// Simulate context assembly: BFS traversal
	visited := make(map[string]bool)
	queue := []string{artefactIDs[9]} // Start from deepest artefact

	for len(queue) > 0 {
		currentID := queue[0]
		queue = queue[1:]

		if visited[currentID] {
			continue
		}
		visited[currentID] = true

		// Fetch artefact
		artefact, err := env.BBClient.GetArtefact(ctx, currentID)
		require.NoError(t, err)

		// Add source artefacts to queue
		for _, sourceID := range artefact.SourceArtefacts {
			if !visited[sourceID] {
				queue = append(queue, sourceID)
			}
		}
	}

	duration := time.Since(startTime)
	t.Logf("✓ Context assembly (10-level graph) completed in: %v", duration)
	t.Logf("  Visited %d artefacts", len(visited))

	// Assert threshold
	threshold := 1 * time.Second
	if duration > threshold {
		t.Errorf("❌ PERFORMANCE REGRESSION: Context assembly took %v, threshold is %v", duration, threshold)
	} else {
		t.Logf("✓ Performance requirement met: %v < %v", duration, threshold)
	}

	t.Log("=== Performance Test: Context Assembly Complete ===")
}

// TestPerformance_GitCommit measures git commit operation time
func TestPerformance_GitCommit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	t.Log("=== Performance Test: Git Commit ===")

	// Build git agent
	buildCmd := exec.Command("docker", "build",
		"-t", "example-git-agent:latest",
		"-f", "agents/example-git-agent/Dockerfile",
		".")
	buildCmd.Dir = testutil.GetProjectRoot()
	output, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Logf("Build output:\n%s", string(output))
	}
	require.NoError(t, err)

	// Setup environment
	env := testutil.SetupE2EEnvironment(t, testutil.GitAgentSettYML())
	defer func() {
		downCmd := &cobra.Command{}
		downInstanceName = env.InstanceName
		_ = runDown(downCmd, []string{})
	}()

	// Start instance
	upCmd := &cobra.Command{}
	upInstanceName = env.InstanceName
	upForce = false
	err = runUp(upCmd, []string{})
	require.NoError(t, err)

	env.WaitForContainer("orchestrator")
	env.WaitForContainer("agent-git-agent")
	env.InitializeBlackboardClient()

	t.Log("✓ Instance ready with git agent")

	// Submit goal and measure time to CodeCommit
	t.Log("Measuring git commit operation time...")
	startTime := time.Now()

	forageCmd := &cobra.Command{}
	forageInstanceName = env.InstanceName
	forageWatch = false
	forageGoal = "perf-test.txt"

	err = runForage(forageCmd, []string{})
	require.NoError(t, err)

	// Wait for CodeCommit artefact
	codeCommitArtefact := env.WaitForArtefactByType("CodeCommit")
	require.NotNil(t, codeCommitArtefact)

	duration := time.Since(startTime)
	t.Logf("✓ Git commit operation completed in: %v", duration)

	// Verify commit exists
	env.VerifyGitCommitExists(codeCommitArtefact.Payload)
	env.VerifyFileExists("perf-test.txt")

	// Assert threshold (includes agent execution + file creation + git add + git commit)
	threshold := 5 * time.Second
	if duration > threshold {
		t.Errorf("❌ PERFORMANCE REGRESSION: Git commit operation took %v, threshold is %v", duration, threshold)
	} else {
		t.Logf("✓ Performance requirement met: %v < %v", duration, threshold)
	}

	t.Log("=== Performance Test: Git Commit Complete ===")
}

// TestPerformance_FullWorkflowE2E measures complete end-to-end workflow time
func TestPerformance_FullWorkflowE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	ctx := context.Background()

	t.Log("=== Performance Test: Full Workflow E2E ===")

	// Build git agent
	buildCmd := exec.Command("docker", "build",
		"-t", "example-git-agent:latest",
		"-f", "agents/example-git-agent/Dockerfile",
		".")
	buildCmd.Dir = testutil.GetProjectRoot()
	buildCmd.Run()

	// Setup environment
	env := testutil.SetupE2EEnvironment(t, testutil.GitAgentSettYML())
	defer func() {
		downCmd := &cobra.Command{}
		downInstanceName = env.InstanceName
		_ = runDown(downCmd, []string{})
	}()

	// Measure total time from sett up to CodeCommit
	t.Log("Measuring full workflow time (sett up + forage + execution)...")
	totalStartTime := time.Now()

	// sett up
	upCmd := &cobra.Command{}
	upInstanceName = env.InstanceName
	upForce = false
	err := runUp(upCmd, []string{})
	require.NoError(t, err)

	upDuration := time.Since(totalStartTime)
	t.Logf("  sett up: %v", upDuration)

	err = instance.VerifyInstanceRunning(ctx, env.DockerClient, env.InstanceName)
	require.NoError(t, err)

	env.WaitForContainer("orchestrator")
	env.WaitForContainer("agent-git-agent")
	env.InitializeBlackboardClient()

	// forage
	forageStartTime := time.Now()
	forageCmd := &cobra.Command{}
	forageInstanceName = env.InstanceName
	forageWatch = false
	forageGoal = "e2e-perf.txt"

	err = runForage(forageCmd, []string{})
	require.NoError(t, err)

	// Wait for completion
	codeCommitArtefact := env.WaitForArtefactByType("CodeCommit")
	require.NotNil(t, codeCommitArtefact)

	forageDuration := time.Since(forageStartTime)
	totalDuration := time.Since(totalStartTime)

	t.Logf("  forage + execution: %v", forageDuration)
	t.Logf("✓ Full workflow completed in: %v", totalDuration)

	// Verify result
	env.VerifyGitCommitExists(codeCommitArtefact.Payload)
	env.VerifyFileExists("e2e-perf.txt")

	// Log performance breakdown
	t.Log("Performance Breakdown:")
	t.Logf("  Startup: %v (%.1f%%)", upDuration, float64(upDuration)/float64(totalDuration)*100)
	t.Logf("  Execution: %v (%.1f%%)", forageDuration, float64(forageDuration)/float64(totalDuration)*100)
	t.Logf("  Total: %v", totalDuration)

	t.Log("=== Performance Test: Full Workflow E2E Complete ===")
}
