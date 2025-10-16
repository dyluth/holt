package orchestrator

import (
	"testing"
)

// TestSelectExclusiveWinner_Single tests single bidder case
func TestSelectExclusiveWinner_Single(t *testing.T) {
	bidders := []string{"agent-a"}
	winner := SelectExclusiveWinner(bidders)

	if winner != "agent-a" {
		t.Errorf("Expected winner 'agent-a', got '%s'", winner)
	}
}

// TestSelectExclusiveWinner_AlphabeticalOrdering tests deterministic alphabetical tie-breaking
func TestSelectExclusiveWinner_AlphabeticalOrdering(t *testing.T) {
	tests := []struct {
		name     string
		bidders  []string
		expected string
	}{
		{
			name:     "two bidders - alpha first",
			bidders:  []string{"beta-agent", "alpha-agent"},
			expected: "alpha-agent",
		},
		{
			name:     "two bidders - already sorted",
			bidders:  []string{"alpha-agent", "beta-agent"},
			expected: "alpha-agent",
		},
		{
			name:     "three bidders - gamma first alphabetically",
			bidders:  []string{"omega-agent", "gamma-agent", "zeta-agent"},
			expected: "gamma-agent",
		},
		{
			name:     "multiple bidders - deterministic",
			bidders:  []string{"zulu", "alpha", "charlie", "bravo"},
			expected: "alpha",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			winner := SelectExclusiveWinner(tt.bidders)
			if winner != tt.expected {
				t.Errorf("Expected winner '%s', got '%s'", tt.expected, winner)
			}

			// Test determinism - run again and verify same result
			winner2 := SelectExclusiveWinner(tt.bidders)
			if winner2 != winner {
				t.Errorf("Non-deterministic result: first='%s', second='%s'", winner, winner2)
			}
		})
	}
}

// TestSelectExclusiveWinner_Panic tests panic on empty list
func TestSelectExclusiveWinner_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic on empty bidders list")
		}
	}()

	SelectExclusiveWinner([]string{})
}
