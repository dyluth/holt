package instance

import (
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	dockerpkg "github.com/dyluth/holt/internal/docker"
	"github.com/stretchr/testify/require"
)

// pullImageIfNeeded pulls a Docker image if it doesn't exist locally
func pullImageIfNeeded(t *testing.T, cli *client.Client, ctx context.Context, imageName string) {
	t.Helper()

	// Check if image exists
	_, _, err := cli.ImageInspectWithRaw(ctx, imageName)
	if err == nil {
		// Image already exists
		return
	}

	// Pull the image
	t.Logf("Pulling image %s...", imageName)
	reader, err := cli.ImagePull(ctx, imageName, types.ImagePullOptions{})
	if err != nil {
		t.Fatalf("Failed to pull image %s: %v", imageName, err)
	}
	defer reader.Close()

	// Wait for pull to complete
	_, err = io.Copy(io.Discard, reader)
	if err != nil {
		t.Fatalf("Failed to complete image pull %s: %v", imageName, err)
	}
	t.Logf("Successfully pulled %s", imageName)
}

func TestFindInstanceByWorkspace(t *testing.T) {
	// Skip if Docker not available
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Skip("Docker not available")
	}
	defer cli.Close()

	ctx := context.Background()

	t.Run("returns instance name when one match found", func(t *testing.T) {
		// Pull image if needed
		pullImageIfNeeded(t, cli, ctx, "busybox:latest")

		// Use /tmp and canonicalize it (on macOS, /tmp is a symlink to /private/tmp)
		workspacePath, err := filepath.EvalSymlinks("/tmp")
		require.NoError(t, err)
		workspacePath, err = filepath.Abs(workspacePath)
		require.NoError(t, err)

		// Create dummy container with workspace label using canonicalized path
		labels := map[string]string{
			dockerpkg.LabelProject:       "true",
			dockerpkg.LabelInstanceName:  "test-instance",
			dockerpkg.LabelWorkspacePath: workspacePath,
			dockerpkg.LabelComponent:     "redis",
		}

		// Use busybox for minimal footprint
		resp, err := cli.ContainerCreate(ctx, &container.Config{
			Image:  "busybox:latest",
			Cmd:    []string{"sleep", "1"},
			Labels: labels,
		}, nil, nil, nil, "")
		require.NoError(t, err)
		defer cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})

		// Find instance - should work with either /tmp or its canonical path
		instanceName, err := FindInstanceByWorkspace(ctx, cli, "/tmp")
		require.NoError(t, err)
		require.Equal(t, "test-instance", instanceName)
	})

	t.Run("returns error when no instances found", func(t *testing.T) {
		// Use /usr which exists but won't have our test containers
		// (they're all created with /tmp or /private/tmp)
		_, err := FindInstanceByWorkspace(ctx, cli, "/usr")
		require.Error(t, err)
		require.Contains(t, err.Error(), "no instances found")
	})

	t.Run("returns error when multiple instances found", func(t *testing.T) {
		// Pull image if needed
		pullImageIfNeeded(t, cli, ctx, "busybox:latest")

		// Use /usr as a shared workspace path
		sharedWorkspace := "/usr"

		// Create two containers for different instances on same workspace
		labels1 := map[string]string{
			dockerpkg.LabelProject:       "true",
			dockerpkg.LabelInstanceName:  "instance-1",
			dockerpkg.LabelWorkspacePath: sharedWorkspace,
			dockerpkg.LabelComponent:     "redis",
		}
		labels2 := map[string]string{
			dockerpkg.LabelProject:       "true",
			dockerpkg.LabelInstanceName:  "instance-2",
			dockerpkg.LabelWorkspacePath: sharedWorkspace,
			dockerpkg.LabelComponent:     "redis",
		}

		resp1, err := cli.ContainerCreate(ctx, &container.Config{
			Image:  "busybox:latest",
			Cmd:    []string{"sleep", "1"},
			Labels: labels1,
		}, nil, nil, nil, "")
		require.NoError(t, err)
		defer cli.ContainerRemove(ctx, resp1.ID, container.RemoveOptions{Force: true})

		resp2, err := cli.ContainerCreate(ctx, &container.Config{
			Image:  "busybox:latest",
			Cmd:    []string{"sleep", "1"},
			Labels: labels2,
		}, nil, nil, nil, "")
		require.NoError(t, err)
		defer cli.ContainerRemove(ctx, resp2.ID, container.RemoveOptions{Force: true})

		// Find instance
		_, err = FindInstanceByWorkspace(ctx, cli, sharedWorkspace)
		require.Error(t, err)
		require.Contains(t, err.Error(), "multiple instances found")
	})
}

func TestGetInstanceRedisPort(t *testing.T) {
	// Skip if Docker not available
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Skip("Docker not available")
	}
	defer cli.Close()

	ctx := context.Background()

	t.Run("returns port from Redis container label", func(t *testing.T) {
		// Pull image if needed
		pullImageIfNeeded(t, cli, ctx, "busybox:latest")

		// Create dummy Redis container with port label
		labels := map[string]string{
			dockerpkg.LabelProject:      "true",
			dockerpkg.LabelInstanceName: "test-instance",
			dockerpkg.LabelComponent:    "redis",
			dockerpkg.LabelRedisPort:    "6380",
		}

		resp, err := cli.ContainerCreate(ctx, &container.Config{
			Image:  "busybox:latest",
			Cmd:    []string{"sleep", "1"},
			Labels: labels,
		}, nil, nil, nil, "")
		require.NoError(t, err)
		defer cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})

		// Get port
		port, err := GetInstanceRedisPort(ctx, cli, "test-instance")
		require.NoError(t, err)
		require.Equal(t, 6380, port)
	})

	t.Run("returns error when Redis container not found", func(t *testing.T) {
		_, err := GetInstanceRedisPort(ctx, cli, "nonexistent-instance")
		require.Error(t, err)
		require.Contains(t, err.Error(), "Redis container not found")
	})

	t.Run("returns error when port label missing", func(t *testing.T) {
		// Pull image if needed
		pullImageIfNeeded(t, cli, ctx, "busybox:latest")

		// Create Redis container without port label
		labels := map[string]string{
			dockerpkg.LabelProject:      "true",
			dockerpkg.LabelInstanceName: "test-instance-no-port",
			dockerpkg.LabelComponent:    "redis",
		}

		resp, err := cli.ContainerCreate(ctx, &container.Config{
			Image:  "busybox:latest",
			Cmd:    []string{"sleep", "1"},
			Labels: labels,
		}, nil, nil, nil, "")
		require.NoError(t, err)
		defer cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})

		// Get port
		_, err = GetInstanceRedisPort(ctx, cli, "test-instance-no-port")
		require.Error(t, err)
		require.Contains(t, err.Error(), "port label missing")
	})
}

func TestVerifyInstanceRunning(t *testing.T) {
	// Skip if Docker not available
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Skip("Docker not available")
	}
	defer cli.Close()

	ctx := context.Background()

	t.Run("returns nil when instance containers are running", func(t *testing.T) {
		// Pull image if needed
		pullImageIfNeeded(t, cli, ctx, "busybox:latest")

		// Create and start Redis container
		redisLabels := map[string]string{
			dockerpkg.LabelProject:      "true",
			dockerpkg.LabelInstanceName: "running-instance",
			dockerpkg.LabelComponent:    "redis",
		}

		redisResp, err := cli.ContainerCreate(ctx, &container.Config{
			Image:  "busybox:latest",
			Cmd:    []string{"sleep", "10"},
			Labels: redisLabels,
		}, nil, nil, nil, "")
		require.NoError(t, err)
		defer cli.ContainerRemove(ctx, redisResp.ID, container.RemoveOptions{Force: true})

		// Start Redis container
		err = cli.ContainerStart(ctx, redisResp.ID, container.StartOptions{})
		require.NoError(t, err)

		// Create and start orchestrator container
		orchLabels := map[string]string{
			dockerpkg.LabelProject:      "true",
			dockerpkg.LabelInstanceName: "running-instance",
			dockerpkg.LabelComponent:    "orchestrator",
		}

		orchResp, err := cli.ContainerCreate(ctx, &container.Config{
			Image:  "busybox:latest",
			Cmd:    []string{"sleep", "10"},
			Labels: orchLabels,
		}, nil, nil, nil, "")
		require.NoError(t, err)
		defer cli.ContainerRemove(ctx, orchResp.ID, container.RemoveOptions{Force: true})

		// Start orchestrator container
		err = cli.ContainerStart(ctx, orchResp.ID, container.StartOptions{})
		require.NoError(t, err)

		// Verify running
		err = VerifyInstanceRunning(ctx, cli, "running-instance")
		require.NoError(t, err)
	})

	t.Run("returns error when instance not found", func(t *testing.T) {
		err := VerifyInstanceRunning(ctx, cli, "nonexistent-instance")
		require.Error(t, err)
		require.Contains(t, err.Error(), "not found")
	})

	t.Run("returns error when container not running", func(t *testing.T) {
		// Pull image if needed
		pullImageIfNeeded(t, cli, ctx, "busybox:latest")

		// Create but don't start Redis container
		redisLabels := map[string]string{
			dockerpkg.LabelProject:      "true",
			dockerpkg.LabelInstanceName: "stopped-instance",
			dockerpkg.LabelComponent:    "redis",
		}

		redisResp, err := cli.ContainerCreate(ctx, &container.Config{
			Image:  "busybox:latest",
			Cmd:    []string{"sleep", "1"},
			Labels: redisLabels,
		}, nil, nil, nil, "")
		require.NoError(t, err)
		defer cli.ContainerRemove(ctx, redisResp.ID, container.RemoveOptions{Force: true})

		// Create and start orchestrator container (so we have both, but Redis is stopped)
		orchLabels := map[string]string{
			dockerpkg.LabelProject:      "true",
			dockerpkg.LabelInstanceName: "stopped-instance",
			dockerpkg.LabelComponent:    "orchestrator",
		}

		orchResp, err := cli.ContainerCreate(ctx, &container.Config{
			Image:  "busybox:latest",
			Cmd:    []string{"sleep", "10"},
			Labels: orchLabels,
		}, nil, nil, nil, "")
		require.NoError(t, err)
		defer cli.ContainerRemove(ctx, orchResp.ID, container.RemoveOptions{Force: true})

		// Start only orchestrator
		err = cli.ContainerStart(ctx, orchResp.ID, container.StartOptions{})
		require.NoError(t, err)

		// Verify (should fail because Redis is not running)
		err = VerifyInstanceRunning(ctx, cli, "stopped-instance")
		require.Error(t, err)
		require.Contains(t, err.Error(), "not running")
	})
}
