package orchestrator

import (
	"context"
	"fmt"
	"log"

	"github.com/dyluth/sett/pkg/blackboard"
)

// TransitionToNextPhase atomically transitions a claim to the next phase.
// Refetches claim from Redis to prevent double-transition race conditions.
// Determines next phase based on available bids and grants accordingly.
func (e *Engine) TransitionToNextPhase(ctx context.Context, claim *blackboard.Claim, phaseState *PhaseState) error {
	// Atomic check: Fetch current claim status from Redis
	currentClaim, err := e.client.GetClaim(ctx, claim.ID)
	if err != nil {
		e.logError("failed to fetch claim for transition", err)
		return fmt.Errorf("failed to fetch claim for transition: %w", err)
	}

	// Verify status hasn't changed (prevents double-transition)
	if currentClaim.Status != claim.Status {
		e.logEvent("phase_transition_skipped", map[string]interface{}{
			"claim_id":        claim.ID,
			"expected_status": claim.Status,
			"actual_status":   currentClaim.Status,
		})
		log.Printf("[Orchestrator] Phase transition skipped for claim %s: status changed from %s to %s",
			claim.ID, claim.Status, currentClaim.Status)
		return nil
	}

	// Determine next phase
	var nextStatus blackboard.ClaimStatus
	var nextPhase string

	switch currentClaim.Status {
	case blackboard.ClaimStatusPendingReview:
		// Check if parallel phase has bids
		if HasBidsForPhase(phaseState.AllBids, "parallel") {
			nextStatus = blackboard.ClaimStatusPendingParallel
			nextPhase = "parallel"
		} else if HasBidsForPhase(phaseState.AllBids, "exclusive") {
			nextStatus = blackboard.ClaimStatusPendingExclusive
			nextPhase = "exclusive"
		} else {
			// No more work - claim becomes dormant
			e.logEvent("claim_dormant", map[string]interface{}{
				"claim_id": claim.ID,
				"reason":   "no_grants_remaining_after_review",
			})
			log.Printf("[Orchestrator] Claim %s has no remaining grants after review phase, becoming dormant", claim.ID)
			delete(e.phaseStates, claim.ID)
			return nil
		}

	case blackboard.ClaimStatusPendingParallel:
		// Check if exclusive phase has bids
		if HasBidsForPhase(phaseState.AllBids, "exclusive") {
			nextStatus = blackboard.ClaimStatusPendingExclusive
			nextPhase = "exclusive"
		} else {
			// No exclusive work - claim becomes dormant
			e.logEvent("claim_dormant", map[string]interface{}{
				"claim_id": claim.ID,
				"reason":   "no_grants_remaining_after_parallel",
			})
			log.Printf("[Orchestrator] Claim %s has no remaining grants after parallel phase, becoming dormant", claim.ID)
			delete(e.phaseStates, claim.ID)
			return nil
		}

	case blackboard.ClaimStatusPendingExclusive:
		// Exclusive completes → claim complete
		nextStatus = blackboard.ClaimStatusComplete
		currentClaim.Status = nextStatus
		if err := e.client.UpdateClaim(ctx, currentClaim); err != nil {
			e.logError("failed to update claim status to complete", err)
			return fmt.Errorf("failed to update claim status: %w", err)
		}
		delete(e.phaseStates, claim.ID)

		e.logEvent("claim_complete", map[string]interface{}{
			"claim_id": claim.ID,
		})
		log.Printf("[Orchestrator] Claim %s marked as complete", claim.ID)
		return nil

	default:
		return fmt.Errorf("unexpected claim status for transition: %s", currentClaim.Status)
	}

	// Update claim status
	currentClaim.Status = nextStatus
	if err := e.client.UpdateClaim(ctx, currentClaim); err != nil {
		e.logError("failed to update claim status", err)
		return fmt.Errorf("failed to update claim status: %w", err)
	}

	e.logEvent("phase_transition", map[string]interface{}{
		"claim_id":    claim.ID,
		"from_status": claim.Status,
		"to_status":   nextStatus,
		"next_phase":  nextPhase,
	})

	log.Printf("[Orchestrator] Claim %s transitioned from %s to %s",
		claim.ID, claim.Status, nextStatus)

	// Grant next phase
	return e.GrantNextPhase(ctx, currentClaim, phaseState, nextPhase)
}

// GrantNextPhase grants the next phase to appropriate agents.
func (e *Engine) GrantNextPhase(ctx context.Context, claim *blackboard.Claim, phaseState *PhaseState, nextPhase string) error {
	switch nextPhase {
	case "parallel":
		return e.GrantParallelPhase(ctx, claim, phaseState.AllBids)

	case "exclusive":
		return e.GrantExclusivePhase(ctx, claim, phaseState.AllBids)

	default:
		return fmt.Errorf("unknown next phase: %s", nextPhase)
	}
}

// GrantExclusivePhase grants the claim to a single exclusive agent.
// Uses existing M3.1 logic for exclusive granting.
func (e *Engine) GrantExclusivePhase(ctx context.Context, claim *blackboard.Claim, bids map[string]blackboard.BidType) error {
	// Collect all agents with exclusive bids
	var exclusiveBidders []string
	for agentName, bidType := range bids {
		if bidType == blackboard.BidTypeExclusive {
			exclusiveBidders = append(exclusiveBidders, agentName)
		}
	}

	if len(exclusiveBidders) == 0 {
		return fmt.Errorf("GrantExclusivePhase called with no exclusive bidders")
	}

	// Select winner using deterministic alphabetical ordering (M3.1 logic)
	winner := SelectExclusiveWinner(exclusiveBidders)

	log.Printf("[Orchestrator] Granting exclusive phase to %s for claim %s", winner, claim.ID)

	// Update claim with granted agent
	claim.GrantedExclusiveAgent = winner
	claim.Status = blackboard.ClaimStatusPendingExclusive

	if err := e.client.UpdateClaim(ctx, claim); err != nil {
		return fmt.Errorf("failed to update claim with exclusive grant: %w", err)
	}

	e.logEvent("exclusive_phase_granted", map[string]interface{}{
		"claim_id":          claim.ID,
		"exclusive_agent":   winner,
		"exclusive_bidders": exclusiveBidders,
	})

	// Publish grant notification
	if err := e.publishGrantNotificationWithType(ctx, winner, claim.ID, "exclusive"); err != nil {
		log.Printf("[Orchestrator] Failed to publish exclusive grant notification to %s: %v", winner, err)
	}

	// Create new phase state for exclusive phase
	newPhaseState := NewPhaseState(claim.ID, "exclusive", []string{winner}, bids)
	e.phaseStates[claim.ID] = newPhaseState

	return nil
}
