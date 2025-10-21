package orchestrator

import (
	"context"
	"fmt"
	"log"
	"sort"

	"github.com/dyluth/holt/pkg/blackboard"
)

// GrantClaim determines the initial phase and grants the claim accordingly.
// M3.2: Processes review, parallel, and exclusive bids with phased execution.
//
// Returns error if Redis operations fail. Logs dormant claims if no bids in any phase.
func (e *Engine) GrantClaim(ctx context.Context, claim *blackboard.Claim, bids map[string]blackboard.BidType) error {
	// Determine initial phase based on bids
	initialStatus, initialPhase := DetermineInitialPhase(bids)

	// Check for dormant claim (no bids in any phase)
	if initialPhase == "" {
		log.Printf("[Orchestrator] No bids in any phase for claim %s, claim becomes dormant", claim.ID)
		e.logEvent("claim_dormant", map[string]interface{}{
			"claim_id": claim.ID,
			"reason":   "no_bids_in_any_phase",
			"bids":     bids,
		})
		return nil
	}

	// Update claim status
	claim.Status = initialStatus
	if err := e.client.UpdateClaim(ctx, claim); err != nil {
		return fmt.Errorf("failed to update claim status: %w", err)
	}

	e.logEvent("initial_phase_determined", map[string]interface{}{
		"claim_id":       claim.ID,
		"initial_phase":  initialPhase,
		"initial_status": initialStatus,
		"bids":           bids,
	})

	log.Printf("[Orchestrator] Claim %s starting in %s phase (status: %s)", claim.ID, initialPhase, initialStatus)

	// Grant based on initial phase
	var err error
	var grantedAgents []string

	switch initialPhase {
	case "review":
		err = e.GrantReviewPhase(ctx, claim, bids)
		grantedAgents = claim.GrantedReviewAgents

	case "parallel":
		err = e.GrantParallelPhase(ctx, claim, bids)
		grantedAgents = claim.GrantedParallelAgents

	case "exclusive":
		err = e.GrantExclusivePhase(ctx, claim, bids)
		grantedAgents = []string{claim.GrantedExclusiveAgent}

	default:
		return fmt.Errorf("unknown initial phase: %s", initialPhase)
	}

	if err != nil {
		return fmt.Errorf("failed to grant %s phase: %w", initialPhase, err)
	}

	// Initialize phase state tracking
	phaseState := NewPhaseState(claim.ID, initialPhase, grantedAgents, bids)
	e.phaseStates[claim.ID] = phaseState

	return nil
}

// SelectExclusiveWinner implements deterministic tie-breaking using alphabetical sorting.
// Given a list of agent names, returns the alphabetically-first agent.
//
// This ensures:
//   - Reproducible workflows across runs
//   - No race conditions from temporal ordering
//   - Simple, debuggable tie-breaking logic
//
// Panics if bidders list is empty (caller must check).
func SelectExclusiveWinner(bidders []string) string {
	if len(bidders) == 0 {
		panic("SelectExclusiveWinner called with empty bidders list")
	}

	if len(bidders) == 1 {
		return bidders[0]
	}

	// Sort alphabetically
	sorted := make([]string, len(bidders))
	copy(sorted, bidders)
	sort.Strings(sorted)

	// Return first (alphabetically earliest)
	return sorted[0]
}

// publishClaimGrantedEvent publishes a claim_granted event to the workflow_events channel.
// Detects grant type from claim fields (exclusive, review, or parallel).
func (e *Engine) publishClaimGrantedEvent(ctx context.Context, claim *blackboard.Claim, agentName string) error {
	// Detect grant type from claim fields
	var grantType string
	if claim.GrantedExclusiveAgent != "" {
		grantType = "exclusive"
	} else if len(claim.GrantedReviewAgents) > 0 {
		grantType = "review"
	} else if len(claim.GrantedParallelAgents) > 0 {
		grantType = "parallel"
	} else {
		// Should not happen, but handle gracefully
		return fmt.Errorf("claim has no granted agents")
	}

	eventData := map[string]interface{}{
		"claim_id":   claim.ID,
		"agent_name": agentName,
		"grant_type": grantType,
	}

	if err := e.client.PublishWorkflowEvent(ctx, "claim_granted", eventData); err != nil {
		return fmt.Errorf("failed to publish workflow event: %w", err)
	}

	log.Printf("[Orchestrator] Published claim_granted event: claim_id=%s, agent=%s, type=%s",
		claim.ID, agentName, grantType)

	return nil
}
