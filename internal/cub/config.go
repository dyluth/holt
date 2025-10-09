package cub

import (
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

	// RedisURL is the Redis connection string (from REDIS_URL)
	RedisURL string
}

// LoadConfig reads and validates configuration from environment variables.
// Returns an error if any required variable is missing or invalid.
// This function implements fail-fast validation - all errors are detected
// at startup before any resources are allocated.
func LoadConfig() (*Config, error) {
	cfg := &Config{
		InstanceName: os.Getenv("SETT_INSTANCE_NAME"),
		AgentName:    os.Getenv("SETT_AGENT_NAME"),
		RedisURL:     os.Getenv("REDIS_URL"),
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

	if c.RedisURL == "" {
		return fmt.Errorf("REDIS_URL environment variable is required")
	}

	return nil
}
