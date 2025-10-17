package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// SettConfig represents the top-level sett.yml configuration
type SettConfig struct {
	Version  string              `yaml:"version"`
	Agents   map[string]Agent    `yaml:"agents"`
	Services *ServicesConfig     `yaml:"services,omitempty"`
}

// Agent represents a single agent configuration
type Agent struct {
	Role            string            `yaml:"role"`
	Image           string            `yaml:"image"`         // Required: Docker image name for this agent
	Build           *BuildConfig      `yaml:"build,omitempty"`
	Command         []string          `yaml:"command"`
	Workspace       *WorkspaceConfig  `yaml:"workspace,omitempty"`
	Replicas        *int              `yaml:"replicas,omitempty"`
	Strategy        string            `yaml:"strategy,omitempty"`
	BiddingStrategy string            `yaml:"bidding_strategy"` // Required: review, claim, exclusive, or ignore
	Environment     []string          `yaml:"environment,omitempty"`
	Resources       *ResourcesConfig  `yaml:"resources,omitempty"`
	Prompts         *PromptsConfig    `yaml:"prompts,omitempty"`
}

// BuildConfig specifies how to build an agent's container image
type BuildConfig struct {
	Context string `yaml:"context"`
}

// WorkspaceConfig specifies workspace mount configuration
type WorkspaceConfig struct {
	Mode string `yaml:"mode"` // "ro" or "rw"
}

// ResourcesConfig specifies resource limits and reservations
type ResourcesConfig struct {
	Limits       *ResourceLimits `yaml:"limits,omitempty"`
	Reservations *ResourceLimits `yaml:"reservations,omitempty"`
}

// ResourceLimits specifies CPU and memory limits
type ResourceLimits struct {
	CPUs   string `yaml:"cpus,omitempty"`
	Memory string `yaml:"memory,omitempty"`
}

// PromptsConfig specifies custom prompts for agent operations
type PromptsConfig struct {
	Claim     string `yaml:"claim,omitempty"`
	Execution string `yaml:"execution,omitempty"`
}

// ServicesConfig specifies service-level overrides
type ServicesConfig struct {
	Orchestrator *ServiceOverride `yaml:"orchestrator,omitempty"`
	Redis        *ServiceOverride `yaml:"redis,omitempty"`
}

// ServiceOverride allows overriding default service images
type ServiceOverride struct {
	Image     string           `yaml:"image,omitempty"`
	Resources *ResourcesConfig `yaml:"resources,omitempty"`
}

// Validate performs strict validation on the configuration
func (c *SettConfig) Validate() error {
	// Required: version
	if c.Version != "1.0" {
		return fmt.Errorf("unsupported version: %s (expected: 1.0)", c.Version)
	}

	// Required: at least one agent
	if len(c.Agents) == 0 {
		return fmt.Errorf("no agents defined")
	}

	// Validate each agent
	for name, agent := range c.Agents {
		if err := agent.Validate(name); err != nil {
			return err
		}
	}

	// M3.2: Enforce unique agent roles
	rolesSeen := make(map[string]string) // role → agentName
	for agentName, agent := range c.Agents {
		if existingAgent, exists := rolesSeen[agent.Role]; exists {
			return fmt.Errorf("duplicate agent role '%s' found (agents '%s' and '%s'): all agents must have unique roles in Phase 3",
				agent.Role, existingAgent, agentName)
		}
		rolesSeen[agent.Role] = agentName
	}

	return nil
}

// Validate performs validation on a single agent configuration
func (a *Agent) Validate(name string) error {
	// Required: role
	if a.Role == "" {
		return fmt.Errorf("agent '%s': role is required", name)
	}

	// Required: image
	if a.Image == "" {
		return fmt.Errorf("agent '%s': image is required", name)
	}

	// Required: command
	if len(a.Command) == 0 {
		return fmt.Errorf("agent '%s': command is required", name)
	}

	// Required: bidding_strategy (M3.1)
	if a.BiddingStrategy == "" {
		return fmt.Errorf("agent '%s': bidding_strategy is required", name)
	}

	// Validate bidding_strategy enum
	if a.BiddingStrategy != "review" && a.BiddingStrategy != "claim" && a.BiddingStrategy != "exclusive" && a.BiddingStrategy != "ignore" {
		return fmt.Errorf("agent '%s': invalid bidding_strategy: %s (must be 'review', 'claim', 'exclusive', or 'ignore')", name, a.BiddingStrategy)
	}

	// If build.context specified, verify path exists
	if a.Build != nil && a.Build.Context != "" {
		if _, err := os.Stat(a.Build.Context); os.IsNotExist(err) {
			return fmt.Errorf("agent '%s': build context does not exist: %s", name, a.Build.Context)
		}
	}

	// Validate workspace mode if specified
	if a.Workspace != nil {
		if a.Workspace.Mode != "" && a.Workspace.Mode != "ro" && a.Workspace.Mode != "rw" {
			return fmt.Errorf("agent '%s': invalid workspace mode: %s (must be 'ro' or 'rw')", name, a.Workspace.Mode)
		}
	}

	// Validate strategy if specified
	if a.Strategy != "" && a.Strategy != "reuse" && a.Strategy != "fresh_per_call" {
		return fmt.Errorf("agent '%s': invalid strategy: %s (must be 'reuse' or 'fresh_per_call')", name, a.Strategy)
	}

	return nil
}

// Load reads and validates sett.yml from the specified path
func Load(path string) (*SettConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config SettConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}
