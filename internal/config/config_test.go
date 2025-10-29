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
	configPath := filepath.Join(tmpDir, "holt.yml")

	// Write valid config
	validConfig := `version: "1.0"
agents:
  example-agent:
    role: "Example Agent"
    image: "example-agent:latest"
    command: ["./run.sh"]
    bidding_strategy: "exclusive"
`
	err := os.WriteFile(configPath, []byte(validConfig), 0644)
	require.NoError(t, err)

	// Load and validate
	config, err := Load(configPath)
	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "1.0", config.Version)
	assert.Len(t, config.Agents, 1)
	// M3.7: Role field removed - agent key IS the role
	assert.Equal(t, []string{"./run.sh"}, config.Agents["example-agent"].Command)
}

func TestLoad_FileNotFound(t *testing.T) {
	config, err := Load("/nonexistent/holt.yml")
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "failed to read config")
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "holt.yml")

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
	config := &HoltConfig{
		Version: "2.0",
		Agents: map[string]Agent{
			"test": {
				Image:           "test:latest",
				Command:         []string{"test"},
				BiddingStrategy: "exclusive",
			},
		},
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported version: 2.0")
}

func TestValidate_NoAgents(t *testing.T) {
	config := &HoltConfig{
		Version: "1.0",
		Agents:  map[string]Agent{},
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no agents defined")
}

func TestAgentValidate_MissingRole(t *testing.T) {
	agent := Agent{
		Image:   "test-agent:latest",
		Command: []string{"./run.sh"},
	}

	err := agent.Validate("test-agent")
	assert.Error(t, err)
	// M3.7: No role field validation needed - role check removed
}

func TestAgentValidate_MissingImage(t *testing.T) {
	agent := Agent{
		Command: []string{"./run.sh"},
	}

	err := agent.Validate("test-agent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "image is required")
}

func TestAgentValidate_MissingCommand(t *testing.T) {
	agent := Agent{
		Image:   "test-agent:latest",
		Command: []string{},
	}

	err := agent.Validate("test-agent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "command is required")
}

func TestAgentValidate_InvalidBuildContext(t *testing.T) {
	// Note: Image can be empty when build context is provided (image will be built)
	// But the validation order checks image first, so we can't test build context
	// validation with empty image. This test is now redundant with the more specific test below.
	// Keeping for backward compatibility but marking as obsolete.
	t.Skip("Obsolete: Image validation happens before build context validation")
}

// Test that build context validation is skipped when pre-built image is specified
func TestAgentValidate_BuildContextSkippedWithPrebuiltImage(t *testing.T) {
	agent := Agent{
		Image:           "prebuilt-agent:latest", // Pre-built image specified
		Command:         []string{"./run.sh"},
		BiddingStrategy: "exclusive",
		Build: &BuildConfig{
			Context: "/nonexistent/path", // Path doesn't exist, but should not be validated
		},
	}

	// Should NOT error because pre-built image is specified
	err := agent.Validate("test-agent")
	assert.NoError(t, err, "build context validation should be skipped when pre-built image is specified")
}

// Test that image is required (regardless of build context)
// This documents the actual validation order: image is checked first
func TestAgentValidate_ImageRequired(t *testing.T) {
	agent := Agent{
		Image:           "", // No image
		Command:         []string{"./run.sh"},
		BiddingStrategy: "exclusive",
		Build: &BuildConfig{
			Context: "/some/path", // Build context present but doesn't matter
		},
	}

	err := agent.Validate("test-agent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "image is required",
		"Image validation should happen before build context validation")
}

func TestAgentValidate_ValidBuildContext(t *testing.T) {
	tmpDir := t.TempDir()

	agent := Agent{
		Image:           "test-agent:latest",
		Command:         []string{"./run.sh"},
		BiddingStrategy: "exclusive",
		Build: &BuildConfig{
			Context: tmpDir,
		},
	}

	err := agent.Validate("test-agent")
	assert.NoError(t, err)
}

func TestAgentValidate_InvalidWorkspaceMode(t *testing.T) {
	agent := Agent{
		Image:           "test-agent:latest",
		Command:         []string{"./run.sh"},
		BiddingStrategy: "exclusive",
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
			Image:           "test-agent:latest",
			Command:         []string{"./run.sh"},
			BiddingStrategy: "exclusive",
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
		Image:           "test-agent:latest",
		Command:         []string{"./run.sh"},
		BiddingStrategy: "exclusive",
		Strategy:        "invalid_strategy",
	}

	err := agent.Validate("test-agent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid strategy")
}

func TestAgentValidate_ValidStrategies(t *testing.T) {
	strategies := []string{"reuse", "fresh_per_call"}
	for _, strategy := range strategies {
		agent := Agent{
			Image:           "test-agent:latest",
			Command:         []string{"./run.sh"},
			BiddingStrategy: "exclusive",
			Strategy:        strategy,
		}

		err := agent.Validate("test-agent")
		assert.NoError(t, err, "strategy %s should be valid", strategy)
	}
}

func TestLoad_ComplexConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "holt.yml")
	buildContext := filepath.Join(tmpDir, "agent-build")
	err := os.Mkdir(buildContext, 0755)
	require.NoError(t, err)

	// Write complex config with all features
	// Note: Using absolute path for build context in test
	complexConfig := `version: "1.0"
agents:
  designer:
    role: "Design Agent"
    image: "designer-agent:latest"
    command: ["python", "design.py"]
    bidding_strategy: "exclusive"
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
    image: "coder-agent:latest"
    command: ["./code.sh"]
    bidding_strategy: "exclusive"
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
	// M3.7: Role field removed - agent key IS the role
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
	// M3.7: Role field removed - agent key IS the role
	assert.Equal(t, []string{"./code.sh"}, coder.Command)

	// Verify services
	assert.NotNil(t, config.Services)
	assert.NotNil(t, config.Services.Redis)
	assert.Equal(t, "redis:7-alpine", config.Services.Redis.Image)
	assert.NotNil(t, config.Services.Orchestrator)
	assert.Equal(t, "custom-orchestrator:latest", config.Services.Orchestrator.Image)
}

// M3.2: Test unique role validation
// M3.7: Removed - map keys guarantee uniqueness - func TestValidate_DuplicateRoles(t *testing.T) {
// M3.7: Removed - map keys guarantee uniqueness - 	config := &HoltConfig{
// M3.7: Removed - map keys guarantee uniqueness - 		Version: "1.0",
// M3.7: Removed - map keys guarantee uniqueness - 		Agents: map[string]Agent{
// M3.7: Removed - map keys guarantee uniqueness - 			"agent-1": {
// M3.7: Removed - map keys guarantee uniqueness - 				Image:           "agent1:latest",
// M3.7: Removed - map keys guarantee uniqueness - 				Command:         []string{"./run.sh"},
// M3.7: Removed - map keys guarantee uniqueness - 				BiddingStrategy: "exclusive",
// M3.7: Removed - map keys guarantee uniqueness - 			},
// M3.7: Removed - map keys guarantee uniqueness - 			"agent-2": {
// M3.7: Removed - map keys guarantee uniqueness - 				Image:           "agent2:latest",
// M3.7: Removed - map keys guarantee uniqueness - 				Command:         []string{"./run.sh"},
// M3.7: Removed - map keys guarantee uniqueness - 				BiddingStrategy: "exclusive",
// M3.7: Removed - map keys guarantee uniqueness - 			},
// M3.7: Removed - map keys guarantee uniqueness - 		},
// M3.7: Removed - map keys guarantee uniqueness - 	}
// M3.7: Removed - map keys guarantee uniqueness - 
// M3.7: Removed - map keys guarantee uniqueness - 	err := config.Validate()
// M3.7: Removed - map keys guarantee uniqueness - 	assert.Error(t, err)
// M3.7: Removed - map keys guarantee uniqueness - 	assert.Contains(t, err.Error(), "duplicate agent role 'Coder' found")
// M3.7: Removed - map keys guarantee uniqueness - 	assert.Contains(t, err.Error(), "agent-1")
// M3.7: Removed - map keys guarantee uniqueness - 	assert.Contains(t, err.Error(), "agent-2")
// M3.7: Removed - map keys guarantee uniqueness - 	assert.Contains(t, err.Error(), "all agents must have unique roles in Phase 3")
// M3.7: Removed - map keys guarantee uniqueness - }
// M3.7: Removed - map keys guarantee uniqueness - 
// M3.7: Removed - map keys guarantee uniqueness - func TestValidate_UniqueRoles(t *testing.T) {
// M3.7: Removed - map keys guarantee uniqueness - 	config := &HoltConfig{
// M3.7: Removed - map keys guarantee uniqueness - 		Version: "1.0",
// M3.7: Removed - map keys guarantee uniqueness - 		Agents: map[string]Agent{
// M3.7: Removed - map keys guarantee uniqueness - 			"reviewer": {
// M3.7: Removed - map keys guarantee uniqueness - 				Image:           "reviewer:latest",
// M3.7: Removed - map keys guarantee uniqueness - 				Command:         []string{"./review.sh"},
// M3.7: Removed - map keys guarantee uniqueness - 				BiddingStrategy: "review",
// M3.7: Removed - map keys guarantee uniqueness - 			},
// M3.7: Removed - map keys guarantee uniqueness - 			"tester": {
// M3.7: Removed - map keys guarantee uniqueness - 				Image:           "tester:latest",
// M3.7: Removed - map keys guarantee uniqueness - 				Command:         []string{"./test.sh"},
// M3.7: Removed - map keys guarantee uniqueness - 				BiddingStrategy: "claim",
// M3.7: Removed - map keys guarantee uniqueness - 			},
// M3.7: Removed - map keys guarantee uniqueness - 			"coder": {
// M3.7: Removed - map keys guarantee uniqueness - 				Image:           "coder:latest",
// M3.7: Removed - map keys guarantee uniqueness - 				Command:         []string{"./code.sh"},
// M3.7: Removed - map keys guarantee uniqueness - 				BiddingStrategy: "exclusive",
// M3.7: Removed - map keys guarantee uniqueness - 			},
// M3.7: Removed - map keys guarantee uniqueness - 		},
// M3.7: Removed - map keys guarantee uniqueness - 	}
// M3.7: Removed - map keys guarantee uniqueness - 
// M3.7: Removed - map keys guarantee uniqueness - 	err := config.Validate()
// M3.7: Removed - map keys guarantee uniqueness - 	assert.NoError(t, err)
// M3.7: Removed - map keys guarantee uniqueness - }

// M3.7: Removed - map keys guarantee uniqueness - func TestValidate_MultipleDuplicateRoles(t *testing.T) {
// M3.7: Removed - map keys guarantee uniqueness - 	config := &HoltConfig{
// M3.7: Removed - map keys guarantee uniqueness - 		Version: "1.0",
// M3.7: Removed - map keys guarantee uniqueness - 		Agents: map[string]Agent{
// M3.7: Removed - map keys guarantee uniqueness - 			"agent-1": {
// M3.7: Removed - map keys guarantee uniqueness - 				Image:           "agent1:latest",
// M3.7: Removed - map keys guarantee uniqueness - 				Command:         []string{"./run.sh"},
// M3.7: Removed - map keys guarantee uniqueness - 				BiddingStrategy: "exclusive",
// M3.7: Removed - map keys guarantee uniqueness - 			},
// M3.7: Removed - map keys guarantee uniqueness - 			"agent-2": {
// M3.7: Removed - map keys guarantee uniqueness - 				Image:           "agent2:latest",
// M3.7: Removed - map keys guarantee uniqueness - 				Command:         []string{"./run.sh"},
// M3.7: Removed - map keys guarantee uniqueness - 				BiddingStrategy: "exclusive",
// M3.7: Removed - map keys guarantee uniqueness - 			},
// M3.7: Removed - map keys guarantee uniqueness - 			"agent-3": {
// M3.7: Removed - map keys guarantee uniqueness - 				Image:           "agent3:latest",
// M3.7: Removed - map keys guarantee uniqueness - 				Command:         []string{"./run.sh"},
// M3.7: Removed - map keys guarantee uniqueness - 				BiddingStrategy: "review",
// M3.7: Removed - map keys guarantee uniqueness - 			},
// M3.7: Removed - map keys guarantee uniqueness - 		},
// M3.7: Removed - map keys guarantee uniqueness - 	}
// M3.7: Removed - map keys guarantee uniqueness - 
// M3.7: Removed - map keys guarantee uniqueness - 	// Should catch the first duplicate it encounters
// M3.7: Removed - map keys guarantee uniqueness - 	err := config.Validate()
// M3.7: Removed - map keys guarantee uniqueness - 	assert.Error(t, err)
// M3.7: Removed - map keys guarantee uniqueness - 	assert.Contains(t, err.Error(), "duplicate agent role")
// M3.7: Removed - map keys guarantee uniqueness - }
// M3.7: Removed - map keys guarantee uniqueness - 
// M3.7: Removed - map keys guarantee uniqueness - // M3.3: Orchestrator config validation tests
// M3.7: Removed - map keys guarantee uniqueness - 
// M3.7: Removed - map keys guarantee uniqueness - func TestValidate_OrchestratorConfig_DefaultValue(t *testing.T) {
// M3.7: Removed - map keys guarantee uniqueness - 	config := &HoltConfig{
// M3.7: Removed - map keys guarantee uniqueness - 		Version: "1.0",
// M3.7: Removed - map keys guarantee uniqueness - 		// Orchestrator section omitted - should default to 3
// M3.7: Removed - map keys guarantee uniqueness - 		Agents: map[string]Agent{
// M3.7: Removed - map keys guarantee uniqueness - 			"test": {
// M3.7: Removed - map keys guarantee uniqueness - 				Image:           "test:latest",
// M3.7: Removed - map keys guarantee uniqueness - 				Command:         []string{"test"},
// M3.7: Removed - map keys guarantee uniqueness - 				BiddingStrategy: "exclusive",
// M3.7: Removed - map keys guarantee uniqueness - 			},
// M3.7: Removed - map keys guarantee uniqueness - 		},
// M3.7: Removed - map keys guarantee uniqueness - 	}
// M3.7: Removed - map keys guarantee uniqueness -
// M3.7: Removed - map keys guarantee uniqueness - 	err := config.Validate()
// M3.7: Removed - map keys guarantee uniqueness - 	assert.NoError(t, err)
// M3.7: Removed - map keys guarantee uniqueness - 	assert.NotNil(t, config.Orchestrator, "Orchestrator config should be initialized with defaults")
// M3.7: Removed - map keys guarantee uniqueness - 	assert.NotNil(t, config.Orchestrator.MaxReviewIterations, "MaxReviewIterations should not be nil")
// M3.7: Removed - map keys guarantee uniqueness - 	assert.Equal(t, 3, *config.Orchestrator.MaxReviewIterations, "Default max_review_iterations should be 3")
// M3.7: Removed - map keys guarantee uniqueness - }

func TestValidate_OrchestratorConfig_DefaultValue(t *testing.T) {
	config := &HoltConfig{
		Version: "1.0",
		// Orchestrator section omitted - should default to 3
		Agents: map[string]Agent{
			"test": {
				Image:           "test:latest",
				Command:         []string{"test"},
				BiddingStrategy: "exclusive",
			},
		},
	}

	err := config.Validate()
	assert.NoError(t, err)
	assert.NotNil(t, config.Orchestrator, "Orchestrator config should be initialized with defaults")
	assert.NotNil(t, config.Orchestrator.MaxReviewIterations, "MaxReviewIterations should not be nil")
	assert.Equal(t, 3, *config.Orchestrator.MaxReviewIterations, "Default max_review_iterations should be 3")
}

func TestValidate_OrchestratorConfig_DefaultWhenSectionExists(t *testing.T) {
	config := &HoltConfig{
		Version:      "1.0",
		Orchestrator: &OrchestratorConfig{
			// max_review_iterations not specified - should default to 3
		},
		Agents: map[string]Agent{
			"test": {
				Image:           "test:latest",
				Command:         []string{"test"},
				BiddingStrategy: "exclusive",
			},
		},
	}

	err := config.Validate()
	assert.NoError(t, err)
	assert.NotNil(t, config.Orchestrator.MaxReviewIterations, "MaxReviewIterations should not be nil after validation")
	assert.Equal(t, 3, *config.Orchestrator.MaxReviewIterations, "Default max_review_iterations should be 3 even when orchestrator section exists")
}

func TestValidate_OrchestratorConfig_ValidValues(t *testing.T) {
	tests := []struct {
		name          string
		maxIterations int
	}{
		{"zero (unlimited)", 0},
		{"one iteration", 1},
		{"three iterations", 3},
		{"ten iterations", 10},
		{"large number", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			iterations := tt.maxIterations
			config := &HoltConfig{
				Version: "1.0",
				Orchestrator: &OrchestratorConfig{
					MaxReviewIterations: &iterations,
				},
				Agents: map[string]Agent{
					"test": {
						Image:           "test:latest",
						Command:         []string{"test"},
						BiddingStrategy: "exclusive",
					},
				},
			}

			err := config.Validate()
			assert.NoError(t, err)
			assert.Equal(t, tt.maxIterations, *config.Orchestrator.MaxReviewIterations)
		})
	}
}

func TestValidate_OrchestratorConfig_NegativeValue(t *testing.T) {
	negativeValue := -1
	config := &HoltConfig{
		Version: "1.0",
		Orchestrator: &OrchestratorConfig{
			MaxReviewIterations: &negativeValue,
		},
		Agents: map[string]Agent{
			"test": {
				Image:           "test:latest",
				Command:         []string{"test"},
				BiddingStrategy: "exclusive",
			},
		},
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "orchestrator.max_review_iterations must be >= 0")
	assert.Contains(t, err.Error(), "-1")
}

func TestLoad_WithOrchestratorConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "holt.yml")

	// Write config with orchestrator section
	configWithOrchestrator := `version: "1.0"
orchestrator:
  max_review_iterations: 5
agents:
  example-agent:
    role: "Example Agent"
    image: "example-agent:latest"
    command: ["./run.sh"]
    bidding_strategy: "exclusive"
`
	err := os.WriteFile(configPath, []byte(configWithOrchestrator), 0644)
	require.NoError(t, err)

	// Load and validate
	config, err := Load(configPath)
	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.NotNil(t, config.Orchestrator)
	assert.NotNil(t, config.Orchestrator.MaxReviewIterations)
	assert.Equal(t, 5, *config.Orchestrator.MaxReviewIterations)
}

// M3.4: Controller-worker configuration validation tests

func TestAgentValidate_ControllerWithValidWorker(t *testing.T) {
	agent := Agent{
		Image:           "coder-controller:latest",
		Command:         []string{"./controller.sh"},
		BiddingStrategy: "exclusive",
		Mode:            "controller",
		Worker: &WorkerConfig{
			Image:         "coder-worker:latest",
			MaxConcurrent: 3,
			Command:       []string{"./worker.sh"},
			Workspace: &WorkspaceConfig{
				Mode: "rw",
			},
		},
	}

	err := agent.Validate("coder-controller")
	assert.NoError(t, err)
}

func TestAgentValidate_ControllerMissingWorkerConfig(t *testing.T) {
	agent := Agent{
		Image:           "coder:latest",
		Command:         []string{"./run.sh"},
		BiddingStrategy: "exclusive",
		Mode:            "controller",
		Worker:          nil, // Missing worker config
	}

	err := agent.Validate("coder")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "has mode='controller' but no worker configuration")
}

func TestAgentValidate_ControllerWorkerMissingImage(t *testing.T) {
	agent := Agent{
		Image:           "coder:latest",
		Command:         []string{"./run.sh"},
		BiddingStrategy: "exclusive",
		Mode:            "controller",
		Worker: &WorkerConfig{
			Image:   "", // Missing image
			Command: []string{"./worker.sh"},
		},
	}

	err := agent.Validate("coder")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "worker configuration missing image")
}

func TestAgentValidate_ControllerWorkerMissingCommand(t *testing.T) {
	agent := Agent{
		Image:           "coder:latest",
		Command:         []string{"./run.sh"},
		BiddingStrategy: "exclusive",
		Mode:            "controller",
		Worker: &WorkerConfig{
			Image:   "worker:latest",
			Command: []string{}, // Missing command
		},
	}

	err := agent.Validate("coder")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "worker configuration missing command")
}

func TestAgentValidate_ControllerWorkerDefaultMaxConcurrent(t *testing.T) {
	agent := Agent{
		Image:           "coder:latest",
		Command:         []string{"./run.sh"},
		BiddingStrategy: "exclusive",
		Mode:            "controller",
		Worker: &WorkerConfig{
			Image:         "worker:latest",
			Command:       []string{"./worker.sh"},
			MaxConcurrent: 0, // Should default to 1
		},
	}

	err := agent.Validate("coder")
	assert.NoError(t, err)
	assert.Equal(t, 1, agent.Worker.MaxConcurrent, "MaxConcurrent should default to 1")
}

func TestAgentValidate_ControllerWorkerNegativeMaxConcurrent(t *testing.T) {
	agent := Agent{
		Image:           "coder:latest",
		Command:         []string{"./run.sh"},
		BiddingStrategy: "exclusive",
		Mode:            "controller",
		Worker: &WorkerConfig{
			Image:         "worker:latest",
			Command:       []string{"./worker.sh"},
			MaxConcurrent: -1, // Invalid
		},
	}

	err := agent.Validate("coder")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "worker.max_concurrent must be >= 1")
}

func TestAgentValidate_ControllerWorkerValidMaxConcurrent(t *testing.T) {
	tests := []struct {
		name          string
		maxConcurrent int
	}{
		{"one worker", 1},
		{"three workers", 3},
		{"ten workers", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := Agent{
				Image:           "coder:latest",
				Command:         []string{"./run.sh"},
				BiddingStrategy: "exclusive",
				Mode:            "controller",
				Worker: &WorkerConfig{
					Image:         "worker:latest",
					Command:       []string{"./worker.sh"},
					MaxConcurrent: tt.maxConcurrent,
				},
			}

			err := agent.Validate("coder")
			assert.NoError(t, err)
			assert.Equal(t, tt.maxConcurrent, agent.Worker.MaxConcurrent)
		})
	}
}

func TestAgentValidate_ControllerWorkerInvalidWorkspaceMode(t *testing.T) {
	agent := Agent{
		Image:           "coder:latest",
		Command:         []string{"./run.sh"},
		BiddingStrategy: "exclusive",
		Mode:            "controller",
		Worker: &WorkerConfig{
			Image:   "worker:latest",
			Command: []string{"./worker.sh"},
			Workspace: &WorkspaceConfig{
				Mode: "invalid", // Invalid mode
			},
		},
	}

	err := agent.Validate("coder")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "worker: invalid workspace mode: invalid")
	assert.Contains(t, err.Error(), "must be 'ro' or 'rw'")
}

func TestAgentValidate_ControllerWorkerValidWorkspaceModes(t *testing.T) {
	tests := []struct {
		name string
		mode string
	}{
		{"read-only", "ro"},
		{"read-write", "rw"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := Agent{
				Image:           "coder:latest",
				Command:         []string{"./run.sh"},
				BiddingStrategy: "exclusive",
				Mode:            "controller",
				Worker: &WorkerConfig{
					Image:   "worker:latest",
					Command: []string{"./worker.sh"},
					Workspace: &WorkspaceConfig{
						Mode: tt.mode,
					},
				},
			}

			err := agent.Validate("coder")
			assert.NoError(t, err)
		})
	}
}

func TestAgentValidate_UnknownMode(t *testing.T) {
	agent := Agent{
		Image:           "coder:latest",
		Command:         []string{"./run.sh"},
		BiddingStrategy: "exclusive",
		Mode:            "unknown-mode", // Invalid mode
	}

	err := agent.Validate("coder")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "has unknown mode 'unknown-mode'")
	assert.Contains(t, err.Error(), "valid: 'controller' or omit")
}

func TestAgentValidate_TraditionalAgentNoMode(t *testing.T) {
	agent := Agent{
		Image:           "coder:latest",
		Command:         []string{"./run.sh"},
		BiddingStrategy: "exclusive",
		Mode:            "", // Traditional agent (no mode)
	}

	err := agent.Validate("coder")
	assert.NoError(t, err)
}

func TestLoad_WithControllerWorkerConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "holt.yml")

	// Write config with controller-worker pattern
	controllerConfig := `version: "1.0"
agents:
  coder-controller:
    role: "Coder"
    mode: "controller"
    image: "coder:latest"
    command: ["./controller.sh"]
    bidding_strategy: "exclusive"
    worker:
      image: "coder-worker:latest"
      max_concurrent: 3
      command: ["./worker.sh"]
      workspace:
        mode: rw
`
	err := os.WriteFile(configPath, []byte(controllerConfig), 0644)
	require.NoError(t, err)

	// Load and validate
	config, err := Load(configPath)
	require.NoError(t, err)
	assert.NotNil(t, config)

	// Verify controller configuration
	controller := config.Agents["coder-controller"]
	// M3.7: Role field removed - agent key IS the role
	assert.Equal(t, "controller", controller.Mode)
	assert.Equal(t, "coder:latest", controller.Image)

	// Verify worker configuration
	assert.NotNil(t, controller.Worker)
	assert.Equal(t, "coder-worker:latest", controller.Worker.Image)
	assert.Equal(t, 3, controller.Worker.MaxConcurrent)
	assert.Equal(t, []string{"./worker.sh"}, controller.Worker.Command)
	assert.NotNil(t, controller.Worker.Workspace)
	assert.Equal(t, "rw", controller.Worker.Workspace.Mode)
}

func TestLoad_MixedControllerAndTraditionalAgents(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "holt.yml")

	// Write config with both controller and traditional agents
	mixedConfig := `version: "1.0"
agents:
  coder-controller:
    role: "Coder"
    mode: "controller"
    image: "coder:latest"
    command: ["./controller.sh"]
    bidding_strategy: "exclusive"
    worker:
      image: "coder-worker:latest"
      max_concurrent: 2
      command: ["./worker.sh"]
      workspace:
        mode: rw
  reviewer:
    role: "Reviewer"
    image: "reviewer:latest"
    command: ["./review.sh"]
    bidding_strategy: "review"
`
	err := os.WriteFile(configPath, []byte(mixedConfig), 0644)
	require.NoError(t, err)

	// Load and validate
	config, err := Load(configPath)
	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Len(t, config.Agents, 2)

	// Verify controller
	controller := config.Agents["coder-controller"]
	assert.Equal(t, "controller", controller.Mode)
	assert.NotNil(t, controller.Worker)

	// Verify traditional agent
	reviewer := config.Agents["reviewer"]
	assert.Equal(t, "", reviewer.Mode)
	assert.Nil(t, reviewer.Worker)
}
