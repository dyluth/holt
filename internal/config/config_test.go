package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_ValidConfig(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "sett.yml")

	// Write valid config
	validConfig := `version: "1.0"
agents:
  example-agent:
    role: "Example Agent"
    command: ["./run.sh"]
`
	err := os.WriteFile(configPath, []byte(validConfig), 0644)
	require.NoError(t, err)

	// Load and validate
	config, err := Load(configPath)
	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "1.0", config.Version)
	assert.Len(t, config.Agents, 1)
	assert.Equal(t, "Example Agent", config.Agents["example-agent"].Role)
	assert.Equal(t, []string{"./run.sh"}, config.Agents["example-agent"].Command)
}

func TestLoad_FileNotFound(t *testing.T) {
	config, err := Load("/nonexistent/sett.yml")
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "failed to read config")
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "sett.yml")

	// Write invalid YAML
	invalidYAML := `version: "1.0"
agents:
  - this is invalid
    yaml syntax
`
	err := os.WriteFile(configPath, []byte(invalidYAML), 0644)
	require.NoError(t, err)

	config, err := Load(configPath)
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "failed to parse YAML")
}

func TestValidate_UnsupportedVersion(t *testing.T) {
	config := &SettConfig{
		Version: "2.0",
		Agents: map[string]Agent{
			"test": {
				Role:    "Test",
				Command: []string{"test"},
			},
		},
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported version: 2.0")
}

func TestValidate_NoAgents(t *testing.T) {
	config := &SettConfig{
		Version: "1.0",
		Agents:  map[string]Agent{},
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no agents defined")
}

func TestAgentValidate_MissingRole(t *testing.T) {
	agent := Agent{
		Command: []string{"./run.sh"},
	}

	err := agent.Validate("test-agent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "role is required")
}

func TestAgentValidate_MissingCommand(t *testing.T) {
	agent := Agent{
		Role:    "Test Agent",
		Command: []string{},
	}

	err := agent.Validate("test-agent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "command is required")
}

func TestAgentValidate_InvalidBuildContext(t *testing.T) {
	agent := Agent{
		Role:    "Test Agent",
		Command: []string{"./run.sh"},
		Build: &BuildConfig{
			Context: "/nonexistent/path",
		},
	}

	err := agent.Validate("test-agent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "build context does not exist")
}

func TestAgentValidate_ValidBuildContext(t *testing.T) {
	tmpDir := t.TempDir()

	agent := Agent{
		Role:    "Test Agent",
		Command: []string{"./run.sh"},
		Build: &BuildConfig{
			Context: tmpDir,
		},
	}

	err := agent.Validate("test-agent")
	assert.NoError(t, err)
}

func TestAgentValidate_InvalidWorkspaceMode(t *testing.T) {
	agent := Agent{
		Role:    "Test Agent",
		Command: []string{"./run.sh"},
		Workspace: &WorkspaceConfig{
			Mode: "invalid",
		},
	}

	err := agent.Validate("test-agent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid workspace mode")
}

func TestAgentValidate_ValidWorkspaceModes(t *testing.T) {
	modes := []string{"ro", "rw"}
	for _, mode := range modes {
		agent := Agent{
			Role:    "Test Agent",
			Command: []string{"./run.sh"},
			Workspace: &WorkspaceConfig{
				Mode: mode,
			},
		}

		err := agent.Validate("test-agent")
		assert.NoError(t, err, "mode %s should be valid", mode)
	}
}

func TestAgentValidate_InvalidStrategy(t *testing.T) {
	agent := Agent{
		Role:     "Test Agent",
		Command:  []string{"./run.sh"},
		Strategy: "invalid_strategy",
	}

	err := agent.Validate("test-agent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid strategy")
}

func TestAgentValidate_ValidStrategies(t *testing.T) {
	strategies := []string{"reuse", "fresh_per_call"}
	for _, strategy := range strategies {
		agent := Agent{
			Role:     "Test Agent",
			Command:  []string{"./run.sh"},
			Strategy: strategy,
		}

		err := agent.Validate("test-agent")
		assert.NoError(t, err, "strategy %s should be valid", strategy)
	}
}

func TestLoad_ComplexConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "sett.yml")
	buildContext := filepath.Join(tmpDir, "agent-build")
	err := os.Mkdir(buildContext, 0755)
	require.NoError(t, err)

	// Write complex config with all features
	// Note: Using absolute path for build context in test
	complexConfig := `version: "1.0"
agents:
  designer:
    role: "Design Agent"
    command: ["python", "design.py"]
    build:
      context: ` + buildContext + `
    workspace:
      mode: "ro"
    replicas: 3
    strategy: "reuse"
    environment:
      - "API_KEY=secret"
      - "DEBUG=true"
    resources:
      limits:
        cpus: "2.0"
        memory: "4GB"
      reservations:
        cpus: "1.0"
        memory: "2GB"
    prompts:
      claim: "Evaluate this design task"
      execution: "Execute this design"
  coder:
    role: "Code Agent"
    command: ["./code.sh"]
services:
  redis:
    image: "redis:7-alpine"
  orchestrator:
    image: "custom-orchestrator:latest"
`
	err = os.WriteFile(configPath, []byte(complexConfig), 0644)
	require.NoError(t, err)

	// Load and validate
	config, err := Load(configPath)
	require.NoError(t, err)
	assert.NotNil(t, config)

	// Verify agents
	assert.Len(t, config.Agents, 2)

	designer := config.Agents["designer"]
	assert.Equal(t, "Design Agent", designer.Role)
	assert.Equal(t, []string{"python", "design.py"}, designer.Command)
	assert.NotNil(t, designer.Build)
	assert.Equal(t, buildContext, designer.Build.Context)
	assert.NotNil(t, designer.Workspace)
	assert.Equal(t, "ro", designer.Workspace.Mode)
	assert.NotNil(t, designer.Replicas)
	assert.Equal(t, 3, *designer.Replicas)
	assert.Equal(t, "reuse", designer.Strategy)
	assert.Len(t, designer.Environment, 2)
	assert.NotNil(t, designer.Resources)
	assert.NotNil(t, designer.Resources.Limits)
	assert.Equal(t, "2.0", designer.Resources.Limits.CPUs)
	assert.Equal(t, "4GB", designer.Resources.Limits.Memory)
	assert.NotNil(t, designer.Prompts)
	assert.Equal(t, "Evaluate this design task", designer.Prompts.Claim)

	coder := config.Agents["coder"]
	assert.Equal(t, "Code Agent", coder.Role)
	assert.Equal(t, []string{"./code.sh"}, coder.Command)

	// Verify services
	assert.NotNil(t, config.Services)
	assert.NotNil(t, config.Services.Redis)
	assert.Equal(t, "redis:7-alpine", config.Services.Redis.Image)
	assert.NotNil(t, config.Services.Orchestrator)
	assert.Equal(t, "custom-orchestrator:latest", config.Services.Orchestrator.Image)
}
