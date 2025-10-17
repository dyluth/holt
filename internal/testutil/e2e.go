// +build integration

package testutil

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/dyluth/sett/internal/instance"
	"github.com/dyluth/sett/pkg/blackboard"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// ArtefactResult is a simplified artefact for test assertions
type ArtefactResult struct {
	ID      string
	Type    string
	Payload string
}

// E2EEnvironment represents an isolated E2E test environment
type E2EEnvironment struct {
	T            *testing.T
	TmpDir       string              // Container path for file operations
	TmpDirHost   string              // Host path for Docker bind mounts (DinD only)
	OriginalDir  string
	InstanceName string
	DockerClient *client.Client
	BBClient     *blackboard.Client
	RedisPort    int
	Ctx          context.Context
}

// detectHostPathForApp tries to detect the host filesystem path that maps to /app
// in the current container (for Docker-in-Docker scenarios)
func detectHostPathForApp() string {
	// Try to get our own container ID
	hostname, err := os.Hostname()
	if err != nil {
		return ""
	}

	// Try to inspect our own container using Docker
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return ""
	}
	defer cli.Close()

	inspect, err := cli.ContainerInspect(context.Background(), hostname)
	if err != nil {
		return ""
	}

	// Look for /app mount
	for _, mount := range inspect.Mounts {
		if mount.Destination == "/app" {
			return mount.Source
		}
	}

	return ""
}

// SetupE2EEnvironment creates a fully isolated E2E test environment
// with temp directory, Git repo, sett.yml, and unique instance name
func SetupE2EEnvironment(t *testing.T, settYML string) *E2EEnvironment {
	ctx := context.Background()

	// Create isolated temporary directory in a location accessible to Docker host
	// When running in Docker-in-Docker (e.g., CI or Claude Code), we need to use
	// a directory that's bind-mounted from the host, not in an overlay filesystem.

	// Check if we're running in Docker (Docker-in-Docker scenario)
	_, inDocker := os.LookupEnv("DOCKER_HOST")
	if !inDocker {
		// Also check for .dockerenv file
		if _, err := os.Stat("/.dockerenv"); err == nil {
			inDocker = true
		}
	}

	var tmpDir string
	var err error

	var tmpDirHost string // Host path for Docker bind mounts

	if inDocker {
		// In DinD, use /app if available (likely mounted from host)
		testWorkspacesDir := filepath.Join("/app", ".test-workspaces")
		if err := os.MkdirAll(testWorkspacesDir, 0755); err == nil {
			// Create temp directory using container path (/app/.test-workspaces)
			tmpDir, err = os.MkdirTemp(testWorkspacesDir, fmt.Sprintf("test-e2e-%s-*", time.Now().Format("20060102-150405")))
			if err == nil && tmpDir != "" {
				// Detect host path for Docker bind mounts
				hostPath := detectHostPathForApp()
				if hostPath != "" {
					// Translate container path to host path: /app/... -> /Users/cam/github/sett/...
					tmpDirHost = filepath.Join(hostPath, tmpDir[len("/app"):])
				} else {
					// If detection fails, use container path and hope for the best
					tmpDirHost = tmpDir
				}
			}
		}
	}

	if tmpDir == "" || err != nil {
		// Fall back to system temp directory
		tmpDir, err = os.MkdirTemp("", fmt.Sprintf("test-e2e-%s-*", time.Now().Format("20060102-150405")))
		tmpDirHost = tmpDir // In non-DinD, paths are the same
	}
	require.NoError(t, err, "Failed to create temp directory")

	// Resolve symlinks to get canonical path (critical for macOS where /var -> /private/var)
	canonicalTmpDir, err := filepath.EvalSymlinks(tmpDir)
	require.NoError(t, err, "Failed to resolve tmpDir symlinks")
	tmpDir = canonicalTmpDir

	// Also resolve host path if different
	if tmpDirHost != tmpDir {
		canonicalTmpDirHost, err := filepath.EvalSymlinks(tmpDirHost)
		if err == nil {
			tmpDirHost = canonicalTmpDirHost
		}
	} else {
		tmpDirHost = tmpDir
	}

	// Register cleanup
	t.Cleanup(func() {
		os.RemoveAll(tmpDir) // Clean up using container path
	})

	// Initialize Git repository
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run(), "Failed to initialize Git repository")

	// Configure Git
	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@sett.local").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Sett Test").Run()

	// Create initial commit (required for clean workspace check)
	testFile := filepath.Join(tmpDir, "README.md")
	require.NoError(t, os.WriteFile(testFile, []byte("# Test Project\n"), 0644))
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial commit").Run()

	// Write sett.yml
	settYMLPath := filepath.Join(tmpDir, "sett.yml")
	require.NoError(t, os.WriteFile(settYMLPath, []byte(settYML), 0644), "Failed to write sett.yml")

	// Commit sett.yml so workspace is clean
	exec.Command("git", "-C", tmpDir, "add", "sett.yml").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "Add sett.yml").Run()

	// Fix permissions for Docker container access (critical for CI environments)
	// Containers may run as different users, so we need world-readable/writable files
	// a+rwX means: add read+write for all users, and execute for directories
	chmodCmd := exec.Command("chmod", "-R", "a+rwX", tmpDir)
	if output, err := chmodCmd.CombinedOutput(); err != nil {
		t.Logf("Warning: chmod failed: %v\nOutput: %s", err, string(output))
	}

	// Change to test directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir), "Failed to change to test directory")

	// Generate unique instance name with microseconds for uniqueness
	instanceName := fmt.Sprintf("test-e2e-%s", time.Now().Format("20060102-150405-000000"))

	// Get Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	require.NoError(t, err, "Failed to create Docker client")

	env := &E2EEnvironment{
		T:            t,
		TmpDir:       tmpDir,
		TmpDirHost:   tmpDirHost,
		OriginalDir:  originalDir,
		InstanceName: instanceName,
		DockerClient: cli,
		Ctx:          ctx,
	}

	// Register cleanup
	t.Cleanup(func() {
		if env.BBClient != nil {
			env.BBClient.Close()
		}
		if env.DockerClient != nil {
			env.DockerClient.Close()
		}
		os.Chdir(originalDir)
	})

	return env
}

// InitializeBlackboardClient connects to the blackboard for this environment
// and waits for Redis to be ready
func (env *E2EEnvironment) InitializeBlackboardClient() {
	var err error
	env.RedisPort, err = instance.GetInstanceRedisPort(env.Ctx, env.DockerClient, env.InstanceName)
	require.NoError(env.T, err, "Failed to get Redis port")

	// In Docker-in-Docker scenarios, use host.docker.internal instead of localhost
	// because port mappings don't work between sibling containers
	redisHost := "localhost"
	if _, err := os.Stat("/.dockerenv"); err == nil {
		// We're in Docker, use host.docker.internal to reach the host's published ports
		redisHost = "host.docker.internal"
	}

	redisOpts := &redis.Options{
		Addr: fmt.Sprintf("%s:%d", redisHost, env.RedisPort),
	}

	env.BBClient, err = blackboard.NewClient(redisOpts, env.InstanceName)
	require.NoError(env.T, err, "Failed to create blackboard client")

	// Wait for Redis to be ready (up to 10 seconds)
	env.T.Logf("Waiting for Redis to be ready on %s:%d...", redisHost, env.RedisPort)
	for i := 0; i < 10; i++ {
		if err := env.BBClient.Ping(env.Ctx); err == nil {
			env.T.Logf("✓ Redis is ready")
			return
		}
		time.Sleep(1 * time.Second)
	}
	require.Fail(env.T, "Redis did not become ready within 10 seconds")
}

// WaitForContainer waits for a container to be running (up to 30 seconds)
// containerNameSuffix: "orchestrator", "redis", or "agent-{agent-name}"
func (env *E2EEnvironment) WaitForContainer(containerNameSuffix string) {
	// Container naming patterns:
	// - orchestrator/redis: sett-{component}-{instance-name}
	// - agents: sett-agent-{instance-name}-{agent-name}
	var fullName string
	if containerNameSuffix == "orchestrator" || containerNameSuffix == "redis" {
		fullName = fmt.Sprintf("sett-%s-%s", containerNameSuffix, env.InstanceName)
	} else {
		// Agent pattern: containerNameSuffix is "agent-{agent-name}"
		// Result: sett-agent-{instance-name}-{agent-name}
		agentName := containerNameSuffix[6:] // Remove "agent-" prefix
		fullName = fmt.Sprintf("sett-agent-%s-%s", env.InstanceName, agentName)
	}

	var lastState string
	var lastStatus string
	for i := 0; i < 30; i++ {
		containers, err := env.DockerClient.ContainerList(env.Ctx, container.ListOptions{All: true})
		if err == nil {
			for _, c := range containers {
				for _, name := range c.Names {
					if name == "/"+fullName {
						lastState = c.State
						lastStatus = c.Status
						if c.State == "running" {
							env.T.Logf("✓ Container %s is running", fullName)
							return
						}
					}
				}
			}
		}
		time.Sleep(1 * time.Second)
	}

	// Container never became running - show diagnostic info with logs
	if lastState != "" {
		// Try to get container logs for debugging
		logs, logErr := env.DockerClient.ContainerLogs(env.Ctx, fullName, container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Tail:       "50",
		})
		var logOutput string
		if logErr == nil {
			defer logs.Close()
			logBytes, _ := io.ReadAll(logs)
			logOutput = string(logBytes)
		}

		failMsg := fmt.Sprintf("Container %s did not start within 30 seconds (last state: %s, status: %s)", fullName, lastState, lastStatus)
		if logOutput != "" {
			failMsg += fmt.Sprintf("\n\nContainer logs:\n%s", logOutput)
		}
		require.Fail(env.T, failMsg)
	} else {
		require.Fail(env.T, fmt.Sprintf("Container %s not found", fullName))
	}
}

// WaitForArtefactByType polls blackboard for an artefact of specific type (up to 60 seconds)
func (env *E2EEnvironment) WaitForArtefactByType(artefactType string) *blackboard.Artefact {
	require.NotNil(env.T, env.BBClient, "Blackboard client not initialized - call InitializeBlackboardClient first")

	env.T.Logf("Waiting for artefact of type '%s'...", artefactType)

	var allArtefacts []string // Track all artefacts for debugging

	for i := 0; i < 60; i++ {
		// Scan for artefacts using Redis SCAN
		pattern := fmt.Sprintf("sett:%s:artefact:*", env.InstanceName)
		iter := env.BBClient.RedisClient().Scan(env.Ctx, 0, pattern, 0).Iterator()

		allArtefacts = allArtefacts[:0] // Reset for this iteration

		for iter.Next(env.Ctx) {
			key := iter.Val()

			// Get artefact data
			data, err := env.BBClient.RedisClient().HGetAll(env.Ctx, key).Result()
			if err != nil {
				continue
			}

			// Track this artefact
			if data["type"] != "" {
				allArtefacts = append(allArtefacts, fmt.Sprintf("%s (id=%s)", data["type"], data["id"][:8]))
			}

			// Check if type matches
			if data["type"] == artefactType {
				// Parse artefact
				artefact := &blackboard.Artefact{
					ID:               data["id"],
					LogicalID:        data["logical_id"],
					StructuralType:   blackboard.StructuralType(data["structural_type"]),
					Type:             data["type"],
					Payload:          data["payload"],
					ProducedByRole:   data["produced_by_role"],
					SourceArtefacts:  []string{}, // Simplified for now
				}

				if versionStr, ok := data["version"]; ok {
					if version, err := strconv.Atoi(versionStr); err == nil {
						artefact.Version = version
					}
				}

				env.T.Logf("✓ Found artefact: type=%s, id=%s, payload=%s", artefact.Type, artefact.ID, artefact.Payload)
				return artefact
			}
		}

		time.Sleep(1 * time.Second)
	}

	// Timeout - show what artefacts WERE found
	failMsg := fmt.Sprintf("Artefact of type '%s' not found within 60 seconds", artefactType)
	if len(allArtefacts) > 0 {
		failMsg += fmt.Sprintf("\n\nArtefacts found: %s", strings.Join(allArtefacts, ", "))
	} else {
		failMsg += "\n\nNo artefacts found on blackboard"
	}

	// If we found a ToolExecutionFailure, try to extract and display its payload
	pattern := fmt.Sprintf("sett:%s:artefact:*", env.InstanceName)
	iter := env.BBClient.RedisClient().Scan(env.Ctx, 0, pattern, 0).Iterator()
	for iter.Next(env.Ctx) {
		key := iter.Val()
		data, err := env.BBClient.RedisClient().HGetAll(env.Ctx, key).Result()
		if err != nil {
			continue
		}

		if data["type"] == "ToolExecutionFailure" {
			failMsg += fmt.Sprintf("\n\nToolExecutionFailure payload:\n%s", data["payload"])
			break
		}
	}

	// Try to get container logs for debugging
	for _, containerSuffix := range []string{"orchestrator", "agent-git-agent"} {
		var fullName string
		if containerSuffix == "orchestrator" {
			fullName = fmt.Sprintf("sett-orchestrator-%s", env.InstanceName)
		} else {
			fullName = fmt.Sprintf("sett-agent-%s-git-agent", env.InstanceName)
		}

		logs, logErr := env.DockerClient.ContainerLogs(env.Ctx, fullName, container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Tail:       "30",
		})
		if logErr == nil {
			defer logs.Close()
			logBytes, _ := io.ReadAll(logs)
			failMsg += fmt.Sprintf("\n\n%s logs:\n%s", containerSuffix, string(logBytes))
		}
	}

	require.Fail(env.T, failMsg)
	return nil
}

// VerifyGitCommitExists checks that a commit hash exists in the workspace
func (env *E2EEnvironment) VerifyGitCommitExists(commitHash string) {
	cmd := exec.Command("git", "cat-file", "-e", commitHash)
	cmd.Dir = env.TmpDir
	err := cmd.Run()
	require.NoError(env.T, err, "Git commit %s does not exist", commitHash)
	env.T.Logf("✓ Git commit %s exists", commitHash)
}

// VerifyFileExists checks that a file exists in the workspace
func (env *E2EEnvironment) VerifyFileExists(filename string) {
	filePath := filepath.Join(env.TmpDir, filename)
	_, err := os.Stat(filePath)
	require.NoError(env.T, err, "File %s does not exist", filename)
	env.T.Logf("✓ File %s exists", filename)
}

// VerifyFileContent checks file content matches expected
func (env *E2EEnvironment) VerifyFileContent(filename string, expectedContent string) {
	filePath := filepath.Join(env.TmpDir, filename)
	content, err := os.ReadFile(filePath)
	require.NoError(env.T, err, "Failed to read file %s", filename)
	require.Contains(env.T, string(content), expectedContent, "File content mismatch")
	env.T.Logf("✓ File %s contains expected content", filename)
}

// VerifyWorkspaceClean checks that Git workspace has no uncommitted changes
func (env *E2EEnvironment) VerifyWorkspaceClean() {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = env.TmpDir
	output, err := cmd.Output()
	require.NoError(env.T, err, "Failed to run git status")
	require.Empty(env.T, string(output), "Workspace has uncommitted changes")
	env.T.Logf("✓ Workspace is clean")
}

// CreateDirtyWorkspace creates an uncommitted file to make workspace dirty
func (env *E2EEnvironment) CreateDirtyWorkspace() {
	dirtyFile := filepath.Join(env.TmpDir, "uncommitted.txt")
	require.NoError(env.T, os.WriteFile(dirtyFile, []byte("dirty"), 0644))
	env.T.Logf("✓ Created dirty file: uncommitted.txt")
}

// DefaultSettYML returns a minimal sett.yml with no agents
func DefaultSettYML() string {
	return `version: "1.0"
agents: []
services:
  redis:
    image: redis:7-alpine
`
}

// GitAgentSettYML returns a sett.yml with example-git-agent configured
func GitAgentSettYML() string {
	return `version: "1.0"
agents:
  git-agent:
    role: "Git Agent"
    image: "example-git-agent:latest"
    command: ["/app/run.sh"]
    bidding_strategy: "exclusive"
    workspace:
      mode: rw
services:
  redis:
    image: redis:7-alpine
`
}

// EchoAgentSettYML returns a sett.yml with example-agent (echo) configured
func EchoAgentSettYML() string {
	return `version: "1.0"
agents:
  echo-agent:
    role: "Echo Agent"
    image: "example-agent:latest"
    command: ["/app/run.sh"]
    bidding_strategy: "exclusive"
    workspace:
      mode: ro
services:
  redis:
    image: redis:7-alpine
`
}

// CreateTestAgent creates a custom test agent with provided run.sh script
func (env *E2EEnvironment) CreateTestAgent(agentName, runScript string) {
	agentDir := filepath.Join(env.TmpDir, ".test-agents", agentName)
	require.NoError(env.T, os.MkdirAll(agentDir, 0755))

	// Write run.sh
	runScriptPath := filepath.Join(agentDir, "run.sh")
	require.NoError(env.T, os.WriteFile(runScriptPath, []byte(runScript), 0755))

	env.T.Logf("✓ Created test agent: %s", agentName)
}

// GetProjectRoot returns the project root directory for building Docker images
func GetProjectRoot() string {
	// When running tests, we need to go up from internal/testutil to project root
	// This works because tests compile to a binary in the cmd/sett/commands directory
	root, err := os.Getwd()
	if err != nil {
		return "."
	}

	// Walk up until we find go.mod
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			return root
		}
		parent := filepath.Dir(root)
		if parent == root {
			// Reached filesystem root, default to current dir
			return "."
		}
		root = parent
	}
}
