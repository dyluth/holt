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
// M3.4: Enhanced with controller-worker pattern support.
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

	// M3.4: Check if winner is a controller
	agent, agentExists := e.config.Agents[winner]
	if agentExists && agent.Mode == "controller" {
		// Controller-worker pattern: check max_concurrent limit
		if e.workerManager != nil && e.workerManager.IsAtWorkerLimit(agent.Role, agent.Worker.MaxConcurrent) {
			// At max_concurrent limit - pause granting
			e.logEvent("worker_limit_reached", map[string]interface{}{
				"role":           agent.Role,
				"max_concurrent": agent.Worker.MaxConcurrent,
				"claim_id":       claim.ID,
			})

			log.Printf("[Orchestrator] Role '%s' at max_concurrent worker limit (%d), claim %s remains pending",
				agent.Role, agent.Worker.MaxConcurrent, claim.ID)

			// IMPORTANT: Stateless pause mechanism (M3.4)
			// - Claim remains in pending_consensus status
			// - Will be re-evaluated in next consensus cycle
			// - No queue, no persistence required
			// - Persistent queue deferred to M3.5
			return fmt.Errorf("role '%s' at max_concurrent worker limit (%d)", agent.Role, agent.Worker.MaxConcurrent)
		}

		// Not at limit - proceed with worker launch
		log.Printf("[Orchestrator] Granting exclusive phase to controller %s (will launch worker) for claim %s", winner, claim.ID)

		// Update claim with granted agent
		claim.GrantedExclusiveAgent = winner
		claim.Status = blackboard.ClaimStatusPendingExclusive

		if err := e.client.UpdateClaim(ctx, claim); err != nil {
			return fmt.Errorf("failed to update claim with exclusive grant: %w", err)
		}

		e.logEvent("exclusive_phase_granted_controller", map[string]interface{}{
			"claim_id":          claim.ID,
			"controller_agent":  winner,
			"exclusive_bidders": exclusiveBidders,
		})

		// M3.4: Launch worker instead of publishing grant notification
		if e.workerManager != nil {
			if err := e.workerManager.LaunchWorker(ctx, claim, winner, agent, e.client); err != nil {
				log.Printf("[Orchestrator] Failed to launch worker for controller %s: %v", winner, err)

				// Terminate claim with error
				claim.Status = blackboard.ClaimStatusTerminated
				claim.TerminationReason = fmt.Sprintf("Failed to launch worker: %v", err)
				return e.client.UpdateClaim(ctx, claim)
			}
		} else {
			log.Printf("[Orchestrator] WARN: Controller %s granted but workerManager is nil, cannot launch worker", winner)
		}

		// Create new phase state for exclusive phase
		newPhaseState := NewPhaseState(claim.ID, "exclusive", []string{winner}, bids)
		e.phaseStates[claim.ID] = newPhaseState

		// Don't publish claim event - worker doesn't subscribe
		return nil
	}

	// Traditional agent flow (M3.3 and earlier)
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
