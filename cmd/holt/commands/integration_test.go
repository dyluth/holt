//go:build integration
// +build integration

package commands

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	dockerpkg "github.com/dyluth/holt/internal/docker"
	"github.com/dyluth/holt/internal/instance"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests require a running Docker daemon
// Run with: go test -tags=integration -v ./cmd/holt/commands

func setupGitRepo(t *testing.T) string {
	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	err := cmd.Run()
	require.NoError(t, err, "failed to init git repo")

	// Configure git user (required for commits)
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	return tmpDir
}

func writeSampleConfig(t *testing.T, gitRoot string) {
	configContent := `version: "1.0"
agents:
  example-agent:
    role: "Example Agent"
    image: "example-agent:latest"
    command: ["echo", "hello"]
    bidding_strategy: "exclusive"
`
	err := os.WriteFile(filepath.Join(gitRoot, "holt.yml"), []byte(configContent), 0644)
	require.NoError(t, err)
}

func getDockerClient(t *testing.T) *client.Client {
	cli, err := dockerpkg.NewClient(context.Background())
	require.NoError(t, err, "Docker daemon must be running for integration tests")
	return cli
}

func listContainersForInstance(t *testing.T, cli *client.Client, instanceName string) []types.Container {
	ctx := context.Background()
	f := filters.NewArgs()
	f.Add("label", dockerpkg.LabelInstanceName+"="+instanceName)

	containers, err := cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: f,
	})
	require.NoError(t, err)
	return containers
}

func listNetworksForInstance(t *testing.T, cli *client.Client, instanceName string) []types.NetworkResource {
	ctx := context.Background()
	f := filters.NewArgs()
	f.Add("label", dockerpkg.LabelInstanceName+"="+instanceName)

	networks, err := cli.NetworkList(ctx, types.NetworkListOptions{
		Filters: f,
	})
	require.NoError(t, err)
	return networks
}

func containerNames(containers []types.Container) []string {
	names := make([]string, len(containers))
	for i, c := range containers {
		// Docker container names include leading /
		names[i] = strings.TrimPrefix(c.Names[0], "/")
	}
	return names
}

func cleanupInstance(t *testing.T, cli *client.Client, instanceName string) {
	ctx := context.Background()

	// Find all containers
	containers := listContainersForInstance(t, cli, instanceName)

	// Stop and remove containers
	timeout := 5
	for _, c := range containers {
		_ = cli.ContainerStop(ctx, c.ID, container.StopOptions{Timeout: &timeout})
		_ = cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true})
	}

	// Remove networks
	networks := listNetworksForInstance(t, cli, instanceName)
	for _, n := range networks {
		_ = cli.NetworkRemove(ctx, n.ID)
	}
}

func stopInstanceContainers(t *testing.T, cli *client.Client, instanceName string) {
	ctx := context.Background()
	containers := listContainersForInstance(t, cli, instanceName)

	timeout := 5
	for _, c := range containers {
		err := cli.ContainerStop(ctx, c.ID, container.StopOptions{Timeout: &timeout})
		require.NoError(t, err)
	}
}

func findInstance(infos []instance.InstanceInfo, name string) *instance.InstanceInfo {
	for i := range infos {
		if infos[i].Name == name {
			return &infos[i]
		}
	}
	return nil
}

// TestUpSuccess verifies the complete success path for holt up
func TestUpSuccess(t *testing.T) {
	// Setup: Create temp Git repo with holt.yml
	gitRoot := setupGitRepo(t)
	writeSampleConfig(t, gitRoot)

	// Change to git root
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(originalDir)

	err = os.Chdir(gitRoot)
	require.NoError(t, err)

	// Get Docker client
	cli := getDockerClient(t)
	defer cli.Close()

	instanceName := "test-up-success"
	defer cleanupInstance(t, cli, instanceName)

	// Execute: holt up --name test-up-success directly
	// Set the global variables that runUp will read
	upInstanceName = instanceName
	upForce = false

	// Call runUp directly
	err = runUp(nil, nil)
	require.NoError(t, err)

	// Verify: Containers exist and running
	containers := listContainersForInstance(t, cli, instanceName)

	assert.Len(t, containers, 2, "should have 2 containers")
	names := containerNames(containers)
	assert.Contains(t, names, dockerpkg.RedisContainerName(instanceName))
	assert.Contains(t, names, dockerpkg.OrchestratorContainerName(instanceName))

	for _, c := range containers {
		assert.Equal(t, "running", c.State, "container %s should be running", c.Names[0])
		assert.Equal(t, instanceName, c.Labels[dockerpkg.LabelInstanceName])
		assert.Equal(t, "true", c.Labels[dockerpkg.LabelProject])
	}

	// Verify: Network exists
	networks := listNetworksForInstance(t, cli, instanceName)
	assert.Len(t, networks, 1)
	assert.Equal(t, dockerpkg.NetworkName(instanceName), networks[0].Name)
}

// TestUpNameCollision verifies name collision detection
func TestUpNameCollision(t *testing.T) {
	gitRoot := setupGitRepo(t)
	writeSampleConfig(t, gitRoot)

	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(originalDir)

	err = os.Chdir(gitRoot)
	require.NoError(t, err)

	cli := getDockerClient(t)
	defer cli.Close()

	instanceName := "test-name-collision"
	defer cleanupInstance(t, cli, instanceName)

	// First instance succeeds
	upInstanceName = instanceName
	upForce = false
	err = runUp(nil, nil)
	require.NoError(t, err)

	// Second instance with same name fails
	err = runUp(nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

// TestUpWorkspaceCollision verifies workspace collision detection
func TestUpWorkspaceCollision(t *testing.T) {
	gitRoot := setupGitRepo(t)
	writeSampleConfig(t, gitRoot)

	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(originalDir)

	err = os.Chdir(gitRoot)
	require.NoError(t, err)

	cli := getDockerClient(t)
	defer cli.Close()

	instance1 := "test-workspace-1"
	instance2 := "test-workspace-2"
	defer cleanupInstance(t, cli, instance1)
	defer cleanupInstance(t, cli, instance2)

	// First instance succeeds
	upInstanceName = instance1
	upForce = false
	err = runUp(nil, nil)
	require.NoError(t, err)

	// Second instance on same workspace fails (without --force)
	upInstanceName = instance2
	upForce = false
	err = runUp(nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "workspace in use")

	// With --force, succeeds
	upForce = true
	err = runUp(nil, nil)
	assert.NoError(t, err)

	// Verify both instances exist
	containers1 := listContainersForInstance(t, cli, instance1)
	assert.Len(t, containers1, 2)

	containers2 := listContainersForInstance(t, cli, instance2)
	assert.Len(t, containers2, 2)
}

// TestDownCompleteCleanup verifies complete resource cleanup
func TestDownCompleteCleanup(t *testing.T) {
	gitRoot := setupGitRepo(t)
	writeSampleConfig(t, gitRoot)

	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(originalDir)

	err = os.Chdir(gitRoot)
	require.NoError(t, err)

	cli := getDockerClient(t)
	defer cli.Close()

	instanceName := "test-down-cleanup"

	// Create instance
	upInstanceName = instanceName
	upForce = false
	err = runUp(nil, nil)
	require.NoError(t, err)

	// Verify it exists
	containers := listContainersForInstance(t, cli, instanceName)
	require.Len(t, containers, 2)

	// Execute down command
	downInstanceName = instanceName
	err = runDown(nil, nil)
	require.NoError(t, err)

	// Verify: Nothing remains
	containers = listContainersForInstance(t, cli, instanceName)
	assert.Empty(t, containers, "all containers should be removed")

	networks := listNetworksForInstance(t, cli, instanceName)
	assert.Empty(t, networks, "all networks should be removed")
}

// TestListAccuracy verifies list command accuracy
func TestListAccuracy(t *testing.T) {
	gitRoot := setupGitRepo(t)
	writeSampleConfig(t, gitRoot)

	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(originalDir)

	err = os.Chdir(gitRoot)
	require.NoError(t, err)

	cli := getDockerClient(t)
	defer cli.Close()

	instance1 := "test-list-1"
	instance2 := "test-list-2"
	defer cleanupInstance(t, cli, instance1)
	defer cleanupInstance(t, cli, instance2)

	// Create two instances
	upInstanceName = instance1
	upForce = false
	err = runUp(nil, nil)
	require.NoError(t, err)

	upInstanceName = instance2
	upForce = true
	err = runUp(nil, nil)
	require.NoError(t, err)

	// Stop one instance's containers (but don't remove)
	stopInstanceContainers(t, cli, instance1)

	// Give containers a moment to stop
	time.Sleep(1 * time.Second)

	// Run list command by calling the function directly
	ctx := context.Background()
	dockerCli := getDockerClient(t)
	defer dockerCli.Close()

	// Find all Holt containers
	containerFilters := filters.NewArgs()
	containerFilters.Add("label", dockerpkg.LabelProject+"=true")

	allContainers, err := dockerCli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: containerFilters,
	})
	require.NoError(t, err)

	// Group by instance
	instances := make(map[string][]types.Container)
	for _, c := range allContainers {
		name := c.Labels[dockerpkg.LabelInstanceName]
		instances[name] = append(instances[name], c)
	}

	// Check statuses
	status1 := instance.DetermineStatus(instances[instance1])
	status2 := instance.DetermineStatus(instances[instance2])

	assert.Equal(t, instance.StatusStopped, status1, "instance1 should be stopped")
	assert.Equal(t, instance.StatusRunning, status2, "instance2 should be running")
}
