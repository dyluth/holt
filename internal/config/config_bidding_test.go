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
				Role:            "Test",
				Image:           "test:latest",
				Command:         []string{"/app/run.sh"},
				BiddingStrategy: "exclusive",
			},
			expectError: false,
		},
		{
			name: "valid review strategy",
			agent: Agent{
				Role:            "Test",
				Image:           "test:latest",
				Command:         []string{"/app/run.sh"},
				BiddingStrategy: "review",
			},
			expectError: false,
		},
		{
			name: "valid claim strategy",
			agent: Agent{
				Role:            "Test",
				Image:           "test:latest",
				Command:         []string{"/app/run.sh"},
				BiddingStrategy: "claim",
			},
			expectError: false,
		},
		{
			name: "valid ignore strategy",
			agent: Agent{
				Role:            "Test",
				Image:           "test:latest",
				Command:         []string{"/app/run.sh"},
				BiddingStrategy: "ignore",
			},
			expectError: false,
		},
		{
			name: "missing bidding_strategy",
			agent: Agent{
				Role:    "Test",
				Image:   "test:latest",
				Command: []string{"/app/run.sh"},
				// BiddingStrategy omitted
			},
			expectError:   true,
			errorContains: "bidding_strategy is required",
		},
		{
			name: "invalid bidding_strategy",
			agent: Agent{
				Role:            "Test",
				Image:           "test:latest",
				Command:         []string{"/app/run.sh"},
				BiddingStrategy: "invalid",
			},
			expectError:   true,
			errorContains: "invalid bidding_strategy",
		},
		{
			name: "empty bidding_strategy",
			agent: Agent{
				Role:            "Test",
				Image:           "test:latest",
				Command:         []string{"/app/run.sh"},
				BiddingStrategy: "",
			},
			expectError:   true,
			errorContains: "bidding_strategy is required",
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
