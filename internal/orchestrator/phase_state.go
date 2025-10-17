package orchestrator

import (
	"time"

	"github.com/dyluth/sett/pkg/blackboard"
)

// PhaseState tracks the execution state of a single claim's current phase.
// This is an in-memory structure maintained by the orchestrator to monitor
// phase completion. It is NOT persisted to Redis.
//
// M3.2 Limitation: If the orchestrator restarts, all phase state is lost and
// claims in active phases will become stuck. This is documented as a known
// limitation and will be addressed in a future milestone.
type PhaseState struct {
	ClaimID           string                      // The claim being tracked
	Phase             string                      // Current phase: "review", "parallel", or "exclusive"
	GrantedAgents     []string                    // Agents granted in this phase
	ReceivedArtefacts map[string]string           // agentRole → artefactID
	AllBids           map[string]blackboard.BidType // All original bids (for phase transition logic)
	StartTime         time.Time                   // When this phase started
}

// NewPhaseState creates a new phase state tracker for a claim.
func NewPhaseState(claimID string, phase string, grantedAgents []string, allBids map[string]blackboard.BidType) *PhaseState {
	return &PhaseState{
		ClaimID:           claimID,
		Phase:             phase,
		GrantedAgents:     grantedAgents,
		ReceivedArtefacts: make(map[string]string),
		AllBids:           allBids,
		StartTime:         time.Now(),
	}
}

// IsComplete returns true if all granted agents have produced artefacts.
func (ps *PhaseState) IsComplete() bool {
	return len(ps.ReceivedArtefacts) >= len(ps.GrantedAgents)
}

// HasBidsForPhase checks if there are any bids of a specific type in AllBids.
func HasBidsForPhase(bids map[string]blackboard.BidType, phase string) bool {
	var bidType blackboard.BidType
	switch phase {
	case "review":
		bidType = blackboard.BidTypeReview
	case "parallel":
		bidType = blackboard.BidTypeParallel
	case "exclusive":
		bidType = blackboard.BidTypeExclusive
	default:
		return false
	}

	for _, bt := range bids {
		if bt == bidType {
			return true
		}
	}
	return false
}

// isGrantedAgent checks if an agent role is in the granted agents list.
func isGrantedAgent(claim *blackboard.Claim, agentRole string, phase string) bool {
	var grantedList []string

	switch phase {
	case "review":
		grantedList = claim.GrantedReviewAgents
	case "parallel":
		grantedList = claim.GrantedParallelAgents
	case "exclusive":
		if claim.GrantedExclusiveAgent == "" {
			return false
		}
		// For exclusive, check if the role matches the single granted agent's role
		// Note: In M3.2 we need to map agent name to role, which we'll do when processing artefacts
		return claim.GrantedExclusiveAgent != ""
	default:
		return false
	}

	for _, granted := range grantedList {
		if granted == agentRole {
			return true
		}
	}
	return false
}

// DetermineInitialPhase determines which phase a claim should start in based on bids.
// Returns the claim status and the list of agents to grant.
func DetermineInitialPhase(bids map[string]blackboard.BidType) (blackboard.ClaimStatus, string) {
	hasReviewBids := HasBidsForPhase(bids, "review")
	hasParallelBids := HasBidsForPhase(bids, "parallel")
	hasExclusiveBids := HasBidsForPhase(bids, "exclusive")

	// Phase skipping logic
	if !hasReviewBids {
		if !hasParallelBids {
			if hasExclusiveBids {
				// Skip directly to exclusive
				return blackboard.ClaimStatusPendingExclusive, "exclusive"
			}
			// No bids in any phase - claim becomes dormant
			return blackboard.ClaimStatusPendingReview, "" // Will be logged as dormant
		}
		// Skip to parallel
		return blackboard.ClaimStatusPendingParallel, "parallel"
	}

	// Start with review
	return blackboard.ClaimStatusPendingReview, "review"
}
