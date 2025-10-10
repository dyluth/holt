package cub

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config holds the agent cub's runtime configuration loaded from environment variables.
// All fields are required and validated at startup to ensure fail-fast behavior.
type Config struct {
	// InstanceName is the Sett instance identifier (from SETT_INSTANCE_NAME)
	InstanceName string

	// AgentName is the logical name of this agent (from SETT_AGENT_NAME)
	AgentName string

	// AgentRole is the role of this agent (from SETT_AGENT_ROLE)
	AgentRole string

	// RedisURL is the Redis connection string (from REDIS_URL)
	RedisURL string

	// Command is the command array to execute for agent tools (from SETT_AGENT_COMMAND)
	// Expected format: JSON array like ["/app/run.sh"] or ["/usr/bin/python3", "agent.py"]
	Command []string
}

// LoadConfig reads and validates configuration from environment variables.
// Returns an error if any required variable is missing or invalid.
// This function implements fail-fast validation - all errors are detected
// at startup before any resources are allocated.
func LoadConfig() (*Config, error) {
	cfg := &Config{
		InstanceName: os.Getenv("SETT_INSTANCE_NAME"),
		AgentName:    os.Getenv("SETT_AGENT_NAME"),
		AgentRole:    os.Getenv("SETT_AGENT_ROLE"),
		RedisURL:     os.Getenv("REDIS_URL"),
	}

	// Parse command array from JSON
	commandJSON := os.Getenv("SETT_AGENT_COMMAND")
	if commandJSON != "" {
		if err := json.Unmarshal([]byte(commandJSON), &cfg.Command); err != nil {
			return nil, fmt.Errorf("failed to parse SETT_AGENT_COMMAND as JSON array: %w", err)
		}
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks that all required configuration fields are present and valid.
// Returns the first validation error encountered.
func (c *Config) Validate() error {
	if c.InstanceName == "" {
		return fmt.Errorf("SETT_INSTANCE_NAME environment variable is required")
	}

	if c.AgentName == "" {
		return fmt.Errorf("SETT_AGENT_NAME environment variable is required")
	}

	if c.AgentRole == "" {
		return fmt.Errorf("SETT_AGENT_ROLE environment variable is required")
	}

	if c.RedisURL == "" {
		return fmt.Errorf("REDIS_URL environment variable is required")
	}

	if len(c.Command) == 0 {
		return fmt.Errorf("SETT_AGENT_COMMAND environment variable is required (must be a non-empty JSON array)")
	}

	return nil
}
