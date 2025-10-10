// +build integration

package testutil

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

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
	TmpDir       string
	OriginalDir  string
	InstanceName string
	DockerClient *client.Client
	BBClient     *blackboard.Client
	RedisPort    int
	Ctx          context.Context
}

// SetupE2EEnvironment creates a fully isolated E2E test environment
// with temp directory, Git repo, sett.yml, and unique instance name
func SetupE2EEnvironment(t *testing.T, settYML string) *E2EEnvironment {
	ctx := context.Background()

	// Create isolated temporary directory (auto-cleaned up)
	tmpDir := t.TempDir()

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
func (env *E2EEnvironment) InitializeBlackboardClient() {
	var err error
	env.RedisPort, err = instance.GetInstanceRedisPort(env.Ctx, env.DockerClient, env.InstanceName)
	require.NoError(env.T, err, "Failed to get Redis port")

	redisOpts := &redis.Options{
		Addr: fmt.Sprintf("localhost:%d", env.RedisPort),
	}

	env.BBClient, err = blackboard.NewClient(redisOpts, env.InstanceName)
	require.NoError(env.T, err, "Failed to create blackboard client")
}

// WaitForContainer waits for a container to be running (up to 30 seconds)
func (env *E2EEnvironment) WaitForContainer(containerNameSuffix string) {
	fullName := fmt.Sprintf("sett-%s-%s", env.InstanceName, containerNameSuffix)

	for i := 0; i < 30; i++ {
		containers, err := env.DockerClient.ContainerList(env.Ctx, client.ListContainersOptions{All: true})
		if err == nil {
			for _, container := range containers {
				for _, name := range container.Names {
					if name == "/"+fullName && container.State == "running" {
						env.T.Logf("✓ Container %s is running", fullName)
						return
					}
				}
			}
		}
		time.Sleep(1 * time.Second)
	}

	require.Fail(env.T, fmt.Sprintf("Container %s did not start within 30 seconds", fullName))
}

// WaitForArtefactByType polls blackboard for an artefact of specific type (up to 60 seconds)
func (env *E2EEnvironment) WaitForArtefactByType(artefactType string) *blackboard.Artefact {
	require.NotNil(env.T, env.BBClient, "Blackboard client not initialized - call InitializeBlackboardClient first")

	env.T.Logf("Waiting for artefact of type '%s'...", artefactType)

	for i := 0; i < 60; i++ {
		// Scan for artefacts using Redis SCAN
		pattern := fmt.Sprintf("sett:%s:artefact:*", env.InstanceName)
		iter := env.BBClient.Client.Scan(env.Ctx, 0, pattern, 0).Iterator()

		for iter.Next(env.Ctx) {
			key := iter.Val()

			// Get artefact data
			data, err := env.BBClient.Client.HGetAll(env.Ctx, key).Result()
			if err != nil {
				continue
			}

			// Check if type matches
			if data["type"] == artefactType {
				// Parse artefact
				artefact := &blackboard.Artefact{
					ID:               data["id"],
					LogicalID:        data["logical_id"],
					StructuralType:   data["structural_type"],
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

	require.Fail(env.T, fmt.Sprintf("Artefact of type '%s' not found within 60 seconds", artefactType))
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
