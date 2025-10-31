package config

import (
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

// OrchestratorConfig specifies orchestrator behavior holtings (M3.3)
type OrchestratorConfig struct {
	MaxReviewIterations *int `yaml:"max_review_iterations,omitempty"` // How many times an artefact can be rejected and reworked (0 = unlimited, default = 3)
}

// HoltConfig represents the top-level holt.yml configuration
type HoltConfig struct {
	Version      string              `yaml:"version"`
	Orchestrator *OrchestratorConfig `yaml:"orchestrator,omitempty"` // M3.3: Orchestrator holtings
	Agents       map[string]Agent    `yaml:"agents"`
	Services     *ServicesConfig     `yaml:"services,omitempty"`
}

// Agent represents a single agent configuration
// M3.7: Agent key in holt.yml IS the role - no separate role field
type Agent struct {
	Image           string           `yaml:"image"` // Required: Docker image name for this agent
	Build           *BuildConfig     `yaml:"build,omitempty"`
	Command         []string         `yaml:"command"`
	BidScript       []string         `yaml:"bid_script,omitempty"`
	Workspace       *WorkspaceConfig `yaml:"workspace,omitempty"`
	Replicas        *int             `yaml:"replicas,omitempty"`
	Strategy        string           `yaml:"strategy,omitempty"`
	BiddingStrategy string           `yaml:"bidding_strategy"` // Required: review, claim, exclusive, or ignore
	Environment     []string         `yaml:"environment,omitempty"`
	Resources       *ResourcesConfig `yaml:"resources,omitempty"`
	Prompts         *PromptsConfig   `yaml:"prompts,omitempty"`

	// M3.4: Controller-worker pattern
	Mode   string        `yaml:"mode,omitempty"`   // "controller" or empty (traditional)
	Worker *WorkerConfig `yaml:"worker,omitempty"` // Required if mode="controller"

	// M3.9: Configurable health checks
	HealthCheck *HealthCheckConfig `yaml:"health_check,omitempty"` // Optional: custom health check
}

// BuildConfig specifies how to build an agent's container image
type BuildConfig struct {
	Context string `yaml:"context"`
}

// WorkspaceConfig specifies workspace mount configuration
type WorkspaceConfig struct {
	Mode string `yaml:"mode"` // "ro" or "rw"
}

// WorkerConfig specifies worker configuration for controller-worker pattern (M3.4)
type WorkerConfig struct {
	Image         string           `yaml:"image"`                    // Worker image (can differ from controller)
	MaxConcurrent int              `yaml:"max_concurrent,omitempty"` // Default: 1
	Command       []string         `yaml:"command"`
	Workspace     *WorkspaceConfig `yaml:"workspace,omitempty"`
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

// HealthCheckConfig specifies custom health check configuration (M3.9)
type HealthCheckConfig struct {
	Command  []string `yaml:"command"`            // Command to execute for health check
	Interval string   `yaml:"interval,omitempty"` // Check interval (default: 30s)
	Timeout  string   `yaml:"timeout,omitempty"`  // Command timeout (default: 5s)
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
func (c *HoltConfig) Validate() error {
	// Required: version
	if c.Version != "1.0" {
		return fmt.Errorf("unsupported version: %s (expected: 1.0)", c.Version)
	}

	// Required: at least one agent
	if len(c.Agents) == 0 {
		return fmt.Errorf("no agents defined")
	}

	// M3.7: Validate agent keys (which are now roles)
	for agentRole, agent := range c.Agents {
		if err := validateRoleName(agentRole); err != nil {
			return fmt.Errorf("invalid agent role '%s': %w", agentRole, err)
		}
		if err := agent.Validate(agentRole); err != nil {
			return err
		}
	}

	// M3.7: Role uniqueness is guaranteed by map keys (no duplicate check needed)

	// M3.3: Apply default orchestrator config if missing
	if c.Orchestrator == nil {
		defaultIterations := 3
		c.Orchestrator = &OrchestratorConfig{
			MaxReviewIterations: &defaultIterations,
		}
	} else if c.Orchestrator.MaxReviewIterations == nil {
		// Orchestrator section exists but max_review_iterations not specified - apply default
		defaultIterations := 3
		c.Orchestrator.MaxReviewIterations = &defaultIterations
	}

	// M3.3: Validate orchestrator config
	if *c.Orchestrator.MaxReviewIterations < 0 {
		return fmt.Errorf("orchestrator.max_review_iterations must be >= 0 (0 = unlimited), got %d", *c.Orchestrator.MaxReviewIterations)
	}

	return nil
}

// validateRoleName ensures role names follow conventions (M3.7)
// Rules: PascalCase recommended, alphanumeric + hyphens, 1-64 chars
func validateRoleName(role string) error {
	if role == "" {
		return fmt.Errorf("role cannot be empty")
	}
	if len(role) > 64 {
		return fmt.Errorf("role name too long (max 64 chars)")
	}

	// Check alphanumeric (allowing hyphens for compound roles like "Code-Reviewer")
	for _, ch := range role {
		if !((ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') ||
			(ch >= '0' && ch <= '9') || ch == '-') {
			return fmt.Errorf("role must be alphanumeric with optional hyphens")
		}
	}

	// Warn if not PascalCase (not an error - allow flexibility)
	if role[0] < 'A' || role[0] > 'Z' {
		log.Printf("[Config] Warning: Role '%s' should start with uppercase letter (PascalCase convention)", role)
	}

	return nil
}

// Validate performs validation on a single agent configuration
func (a *Agent) Validate(name string) error {
	// M3.7: No role field validation - agent key IS the role

	// Required: image
	if a.Image == "" {
		return fmt.Errorf("agent '%s': image is required", name)
	}

	// Required: command
	if len(a.Command) == 0 {
		return fmt.Errorf("agent '%s': command is required", name)
	}

	// M3.6: Bidding strategy validation - either bid_script or bidding_strategy required
	hasBidScript := len(a.BidScript) > 0
	hasStaticStrategy := a.BiddingStrategy != ""

	if !hasBidScript && !hasStaticStrategy {
		return fmt.Errorf("agent '%s': either bidding_strategy or bid_script must be provided", name)
	}

	// Validate bidding_strategy enum if provided
	if hasStaticStrategy {
		if a.BiddingStrategy != "review" && a.BiddingStrategy != "claim" && a.BiddingStrategy != "exclusive" && a.BiddingStrategy != "ignore" {
			return fmt.Errorf("agent '%s': invalid bidding_strategy: %s (must be 'review', 'claim', 'exclusive', or 'ignore')", name, a.BiddingStrategy)
		}
	}

	// If build.context specified AND no pre-built image is listed, verify path exists
	// If a.Image is set, the build context is optional (pre-built image is used)
	if a.Image == "" && a.Build != nil && a.Build.Context != "" {
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

	// M3.4: Validate controller-worker configuration
	if a.Mode == "controller" {
		// Validate worker config exists
		if a.Worker == nil {
			return fmt.Errorf("agent '%s' has mode='controller' but no worker configuration", name)
		}

		// Validate worker image
		if a.Worker.Image == "" {
			return fmt.Errorf("agent '%s' worker configuration missing image", name)
		}

		// Validate worker command
		if len(a.Worker.Command) == 0 {
			return fmt.Errorf("agent '%s' worker configuration missing command", name)
		}

		// Set default max_concurrent if not specified
		if a.Worker.MaxConcurrent == 0 {
			a.Worker.MaxConcurrent = 1
		}

		// Validate max_concurrent is positive
		if a.Worker.MaxConcurrent < 1 {
			return fmt.Errorf("agent '%s' worker.max_concurrent must be >= 1", name)
		}

		// Validate worker workspace mode if specified
		if a.Worker.Workspace != nil && a.Worker.Workspace.Mode != "" {
			if a.Worker.Workspace.Mode != "ro" && a.Worker.Workspace.Mode != "rw" {
				return fmt.Errorf("agent '%s' worker: invalid workspace mode: %s (must be 'ro' or 'rw')", name, a.Worker.Workspace.Mode)
			}
		}
	} else if a.Mode != "" {
		// Unknown mode
		return fmt.Errorf("agent '%s' has unknown mode '%s' (valid: 'controller' or omit)", name, a.Mode)
	}

	return nil
}

// Load reads and validates holt.yml from the specified path
func Load(path string) (*HoltConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config HoltConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}
