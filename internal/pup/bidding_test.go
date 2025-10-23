package pup

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dyluth/holt/pkg/blackboard"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestDynamicBidding(t *testing.T) {
	td := t.TempDir()

	// Create a mock bid.sh script for the validator agent
	bidScriptPath := filepath.Join(td, "bid.sh")
	bidScriptContent := `#!/bin/sh
input=$(cat)
# Use grep and cut to avoid jq dependency in tests
artefact_type=$(echo "$input" | grep '"type"' | cut -d'"' -f4)
if [ "$artefact_type" = "RecipeYAML" ]; then
  echo "review"
else
  echo "ignore"
fi
`
	err := os.WriteFile(bidScriptPath, []byte(bidScriptContent), 0755)
	require.NoError(t, err)

	// Configure a pup engine to use the dynamic bid script
	pupEngine := &Engine{
		config: &Config{
			BidScript: []string{bidScriptPath},
		},
	}

	ctx := context.Background()

	t.Run("should ignore GoalDefined artefact", func(t *testing.T) {
		// Create a GoalDefined artefact
		goalArtefact := &blackboard.Artefact{
			ID:   uuid.New().String(),
			Type: "GoalDefined",
		}

		// Determine the bid
		bidType, err := pupEngine.determineBidType(ctx, goalArtefact)

		// Assert that the bid is "ignore"
		require.NoError(t, err)
		require.Equal(t, blackboard.BidTypeIgnore, bidType, "Validator should ignore GoalDefined artefacts")
	})

	t.Run("should review RecipeYAML artefact", func(t *testing.T) {
		// Create a RecipeYAML artefact
		recipeArtefact := &blackboard.Artefact{
			ID:   uuid.New().String(),
			Type: "RecipeYAML",
		}

		// Determine the bid
		bidType, err := pupEngine.determineBidType(ctx, recipeArtefact)

		// Assert that the bid is "review"
		require.NoError(t, err)
		require.Equal(t, blackboard.BidTypeReview, bidType, "Validator should bid review on RecipeYAML artefacts")
	})

	t.Run("should fallback to static strategy if script is not defined", func(t *testing.T) {
		// Create an engine with no bid script, only static strategy
		staticEngine := &Engine{
			config: &Config{
				BiddingStrategy: blackboard.BidTypeExclusive,
			},
		}

		goalArtefact := &blackboard.Artefact{Type: "GoalDefined"}
		bidType, err := staticEngine.determineBidType(ctx, goalArtefact)

		require.NoError(t, err)
		require.Equal(t, blackboard.BidTypeExclusive, bidType, "Should use static bidding_strategy as fallback")
	})

	t.Run("should fallback to static strategy when script fails", func(t *testing.T) {
		// Create a bid script that exits with error
		failingScriptPath := filepath.Join(td, "failing_bid.sh")
		failingScript := `#!/bin/sh
exit 1
`
		err := os.WriteFile(failingScriptPath, []byte(failingScript), 0755)
		require.NoError(t, err)

		// Engine with failing script and fallback strategy
		engineWithFallback := &Engine{
			config: &Config{
				AgentName:       "test-agent",
				BidScript:       []string{failingScriptPath},
				BiddingStrategy: blackboard.BidTypeClaim, // Fallback
			},
		}

		artefact := &blackboard.Artefact{Type: "SomeType"}
		bidType, err := engineWithFallback.determineBidType(ctx, artefact)

		require.NoError(t, err)
		require.Equal(t, blackboard.BidTypeClaim, bidType, "Should fall back to static strategy on script failure")
	})

	t.Run("should return ignore when script fails and no fallback", func(t *testing.T) {
		// Create a bid script that exits with error
		failingScriptPath := filepath.Join(td, "failing_bid2.sh")
		failingScript := `#!/bin/sh
exit 1
`
		err := os.WriteFile(failingScriptPath, []byte(failingScript), 0755)
		require.NoError(t, err)

		// Engine with failing script and NO fallback strategy
		engineNoFallback := &Engine{
			config: &Config{
				AgentName: "test-agent",
				BidScript: []string{failingScriptPath},
				// No BiddingStrategy set
			},
		}

		artefact := &blackboard.Artefact{Type: "SomeType"}
		bidType, err := engineNoFallback.determineBidType(ctx, artefact)

		require.NoError(t, err)
		require.Equal(t, blackboard.BidTypeIgnore, bidType, "Should return 'ignore' when no fallback available")
	})

	t.Run("should fallback when script returns invalid bid type", func(t *testing.T) {
		// Create a bid script that returns invalid output
		invalidScriptPath := filepath.Join(td, "invalid_bid.sh")
		invalidScript := `#!/bin/sh
echo "invalid_bid_type"
`
		err := os.WriteFile(invalidScriptPath, []byte(invalidScript), 0755)
		require.NoError(t, err)

		// Engine with invalid script and fallback
		engineWithFallback := &Engine{
			config: &Config{
				AgentName:       "test-agent",
				BidScript:       []string{invalidScriptPath},
				BiddingStrategy: blackboard.BidTypeReview, // Fallback
			},
		}

		artefact := &blackboard.Artefact{Type: "SomeType"}
		bidType, err := engineWithFallback.determineBidType(ctx, artefact)

		require.NoError(t, err)
		require.Equal(t, blackboard.BidTypeReview, bidType, "Should fall back when script returns invalid bid type")
	})

	t.Run("should handle script that outputs extra whitespace", func(t *testing.T) {
		// Create a bid script that outputs with extra whitespace
		whitespaceScriptPath := filepath.Join(td, "whitespace_bid.sh")
		whitespaceScript := `#!/bin/sh
echo "  claim  "
`
		err := os.WriteFile(whitespaceScriptPath, []byte(whitespaceScript), 0755)
		require.NoError(t, err)

		engineWhitespace := &Engine{
			config: &Config{
				BidScript: []string{whitespaceScriptPath},
			},
		}

		artefact := &blackboard.Artefact{Type: "SomeType"}
		bidType, err := engineWhitespace.determineBidType(ctx, artefact)

		require.NoError(t, err)
		require.Equal(t, blackboard.BidTypeClaim, bidType, "Should trim whitespace from script output")
	})
}
