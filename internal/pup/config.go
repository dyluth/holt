package pup

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/dyluth/holt/pkg/blackboard"
)

// Config holds the agent pup's runtime configuration loaded from environment variables.
// All fields are required and validated at startup to ensure fail-fast behavior.
type Config struct {
	// InstanceName is the Holt instance identifier (from HOLT_INSTANCE_NAME)
	InstanceName string

	// AgentName is the logical name of this agent (from HOLT_AGENT_NAME)
	AgentName string

	// AgentRole is the role of this agent (from HOLT_AGENT_ROLE)
	AgentRole string

	// RedisURL is the Redis connection string (from REDIS_URL)
	RedisURL string

	// Command is the command array to execute for agent tools (from HOLT_AGENT_COMMAND)
	// Expected format: JSON array like ["/app/run.sh"] or ["/usr/bin/python3", "agent.py"]
	Command []string

	// BiddingStrategy is the bid type this agent submits for claims (from HOLT_BIDDING_STRATEGY)
	// M3.1: Must be one of: review, claim, exclusive, ignore
	BiddingStrategy blackboard.BidType

	// BidScript is the command array to execute for dynamic bidding (from HOLT_AGENT_BID_SCRIPT)
	BidScript []string
}

// LoadConfig reads and validates configuration from environment variables.
// Returns an error if any required variable is missing or invalid.
// This function implements fail-fast validation - all errors are detected
// at startup before any resources are allocated.
func LoadConfig() (*Config, error) {
	cfg := &Config{
		InstanceName: os.Getenv("HOLT_INSTANCE_NAME"),
		AgentName:    os.Getenv("HOLT_AGENT_NAME"),
		AgentRole:    os.Getenv("HOLT_AGENT_ROLE"),
		RedisURL:     os.Getenv("REDIS_URL"),
	}

	// Parse command array from JSON
	commandJSON := os.Getenv("HOLT_AGENT_COMMAND")
	if commandJSON != "" {
		if err := json.Unmarshal([]byte(commandJSON), &cfg.Command); err != nil {
			return nil, fmt.Errorf("failed to parse HOLT_AGENT_COMMAND as JSON array: %w", err)
		}
	}

	// Parse bid script array from JSON
	bidScriptJSON := os.Getenv("HOLT_AGENT_BID_SCRIPT")
	if bidScriptJSON != "" {
		if err := json.Unmarshal([]byte(bidScriptJSON), &cfg.BidScript); err != nil {
			return nil, fmt.Errorf("failed to parse HOLT_AGENT_BID_SCRIPT as JSON array: %w", err)
		}
	}

	// Parse bidding strategy (M3.1)
	biddingStrategyStr := os.Getenv("HOLT_BIDDING_STRATEGY")
	if biddingStrategyStr != "" {
		cfg.BiddingStrategy = blackboard.BidType(biddingStrategyStr)
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
		return fmt.Errorf("HOLT_INSTANCE_NAME environment variable is required")
	}

	if c.AgentName == "" {
		return fmt.Errorf("HOLT_AGENT_NAME environment variable is required")
	}

	if c.AgentRole == "" {
		return fmt.Errorf("HOLT_AGENT_ROLE environment variable is required")
	}

	if c.RedisURL == "" {
		return fmt.Errorf("REDIS_URL environment variable is required")
	}

	if len(c.Command) == 0 {
		return fmt.Errorf("HOLT_AGENT_COMMAND environment variable is required (must be a non-empty JSON array)")
	}

	// M3.6: Bidding strategy validation - either bid_script or bidding_strategy required
	hasBidScript := len(c.BidScript) > 0
	hasStaticStrategy := c.BiddingStrategy != ""

	if !hasBidScript && !hasStaticStrategy {
		return fmt.Errorf("either HOLT_BIDDING_STRATEGY or HOLT_AGENT_BID_SCRIPT must be provided")
	}

	// Validate bidding strategy is a valid enum if provided
	if hasStaticStrategy {
		if err := c.BiddingStrategy.Validate(); err != nil {
			return fmt.Errorf("invalid HOLT_BIDDING_STRATEGY: %w", err)
		}
	} else {
		log.Printf("[WARN] No static bidding_strategy configured for agent %s, relying entirely on bid_script", c.AgentName)
	}

	return nil
}
