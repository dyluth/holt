package config

import (
	"strings"
	"testing"
)

// TestAgent_Validate_BiddingStrategy tests M3.1 bidding strategy validation
func TestAgent_Validate_BiddingStrategy(t *testing.T) {
	tests := []struct {
		name          string
		agent         Agent
		expectError   bool
		errorContains string
	}{
		{
			name: "valid exclusive strategy",
			agent: Agent{
				Image:           "test:latest",
				Command:         []string{"/app/run.sh"},
				BiddingStrategy: "exclusive",
			},
			expectError: false,
		},
		{
			name: "valid review strategy",
			agent: Agent{
				Image:           "test:latest",
				Command:         []string{"/app/run.sh"},
				BiddingStrategy: "review",
			},
			expectError: false,
		},
		{
			name: "valid claim strategy",
			agent: Agent{
				Image:           "test:latest",
				Command:         []string{"/app/run.sh"},
				BiddingStrategy: "claim",
			},
			expectError: false,
		},
		{
			name: "valid ignore strategy",
			agent: Agent{
				Image:           "test:latest",
				Command:         []string{"/app/run.sh"},
				BiddingStrategy: "ignore",
			},
			expectError: false,
		},
		// M3.6: Test bid_script scenarios
		{
			name: "valid agent with only bid_script",
			agent: Agent{
				Image:     "test:latest",
				Command:   []string{"/app/run.sh"},
				BidScript: []string{"/app/bid.sh"},
				// No BiddingStrategy
			},
			expectError: false,
		},
		{
			name: "valid agent with both bid_script and bidding_strategy",
			agent: Agent{
				Image:           "test:latest",
				Command:         []string{"/app/run.sh"},
				BidScript:       []string{"/app/bid.sh"},
				BiddingStrategy: "claim", // Fallback
			},
			expectError: false,
		},
		{
			name: "missing both bidding_strategy and bid_script",
			agent: Agent{
				Image:   "test:latest",
				Command: []string{"/app/run.sh"},
				// BiddingStrategy omitted, no BidScript
			},
			expectError:   true,
			errorContains: "either bidding_strategy or bid_script must be provided",
		},
		{
			name: "invalid bidding_strategy with bid_script",
			agent: Agent{
				Image:           "test:latest",
				Command:         []string{"/app/run.sh"},
				BidScript:       []string{"/app/bid.sh"},
				BiddingStrategy: "invalid",
			},
			expectError:   true,
			errorContains: "invalid bidding_strategy",
		},
		{
			name: "empty bidding_strategy (old test - now valid with bid_script)",
			agent: Agent{
				Image:     "test:latest",
				Command:   []string{"/app/run.sh"},
				BidScript: []string{"/app/bid.sh"},
				// Empty BiddingStrategy is now OK if bid_script present
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.agent.Validate("test-agent")

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if tt.expectError && err != nil && tt.errorContains != "" {
				if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorContains, err)
				}
			}
		})
	}
}
