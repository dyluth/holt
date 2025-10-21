package orchestrator

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/dyluth/holt/pkg/blackboard"
)

// PhaseState tracks the execution state of a single claim's current phase.
// This is an in-memory structure maintained by the orchestrator to monitor
// phase completion.
//
// M3.5: Phase state is now persisted to Redis (in the Claim structure) to enable
// orchestrator restart resilience. The in-memory PhaseState is synchronized with
// Redis on every phase transition.
type PhaseState struct {
	ClaimID           string                        // The claim being tracked
	Phase             string                        // Current phase: "review", "parallel", or "exclusive"
	GrantedAgents     []string                      // Agents granted in this phase
	ReceivedArtefacts map[string]string             // agentRole → artefactID
	AllBids           map[string]blackboard.BidType // All original bids (for phase transition logic)
	StartTime         time.Time                     // When this phase started
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

// persistPhaseState writes phase state to the claim in Redis (M3.5).
// This enables orchestrator restart resilience by persisting all phase tracking state.
// Maps in-memory PhaseState to blackboard.PhaseState for persistence.
func (e *Engine) persistPhaseState(ctx context.Context, claim *blackboard.Claim, phaseState *PhaseState) error {
	// Convert in-memory PhaseState to blackboard.PhaseState for persistence
	claim.PhaseState = &blackboard.PhaseState{
		Current:       phaseState.Phase,
		GrantedAgents: phaseState.GrantedAgents,
		Received:      phaseState.ReceivedArtefacts,
		AllBids:       phaseState.AllBids,
		StartTime:     phaseState.StartTime.Unix(),
	}

	// Persist to Redis
	if err := e.client.UpdateClaim(ctx, claim); err != nil {
		return fmt.Errorf("failed to persist phase state: %w", err)
	}

	return nil
}

// DetermineInitialPhase determines which phase a claim should start in based on bids.
// Returns the claim status and the list of agents to grant.
func DetermineInitialPhase(bids map[string]blackboard.BidType) (blackboard.ClaimStatus, string) {
	hasReviewBids := HasBidsForPhase(bids, "review")
	hasParallelBids := HasBidsForPhase(bids, "parallel")
	hasExclusiveBids := HasBidsForPhase(bids, "exclusive")

	// Debug logging to diagnose phase determination
	log.Printf("[DEBUG] DetermineInitialPhase: hasReview=%v, hasParallel=%v, hasExclusive=%v, bids=%v",
		hasReviewBids, hasParallelBids, hasExclusiveBids, bids)

	// Phase skipping logic
	if !hasReviewBids {
		if !hasParallelBids {
			if hasExclusiveBids {
				// Skip directly to exclusive
				log.Printf("[DEBUG] Skipping to exclusive phase")
				return blackboard.ClaimStatusPendingExclusive, "exclusive"
			}
			// No bids in any phase - claim becomes dormant
			log.Printf("[DEBUG] No bids in any phase - dormant")
			return blackboard.ClaimStatusPendingReview, "" // Will be logged as dormant
		}
		// Skip to parallel
		log.Printf("[DEBUG] Skipping review, starting with parallel")
		return blackboard.ClaimStatusPendingParallel, "parallel"
	}

	// Start with review
	log.Printf("[DEBUG] Starting with review phase")
	return blackboard.ClaimStatusPendingReview, "review"
}
