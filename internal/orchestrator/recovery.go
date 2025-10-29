package orchestrator

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/dyluth/holt/pkg/blackboard"
)

// RecoverState recovers orchestrator state from Redis after restart (M3.5).
// This method is called during startup to:
// 1. Clean up orphaned workers from previous orchestrator run
// 2. Scan Redis for active claims
// 3. Reconstruct in-memory phase state
// 4. Re-trigger grants for claims missing artefacts
// 5. Recover grant queues
func (e *Engine) RecoverState(ctx context.Context) error {
	log.Printf("[Orchestrator] Starting state recovery...")
	startTime := time.Now()

	// Step 1: Clean up orphaned workers
	if e.workerManager != nil {
		if err := e.workerManager.CleanupOrphanedWorkers(ctx); err != nil {
			log.Printf("[Orchestrator] Warning: Failed to cleanup orphaned workers: %v", err)
			// Non-fatal - continue recovery
		}
	}

	// Step 2: Scan Redis for active claims
	activeClaims, err := e.client.GetClaimsByStatus(ctx, []string{
		string(blackboard.ClaimStatusPendingReview),
		string(blackboard.ClaimStatusPendingParallel),
		string(blackboard.ClaimStatusPendingExclusive),
		string(blackboard.ClaimStatusPendingAssignment),
	})
	if err != nil {
		return fmt.Errorf("failed to scan for active claims: %w", err)
	}

	log.Printf("[Orchestrator] Found %d active claims to recover", len(activeClaims))

	// Step 3: Reconstruct phase state for each active claim
	recoveredCount := 0
	terminatedCount := 0

	for _, claim := range activeClaims {
		if err := e.reconstructPhaseState(ctx, claim); err != nil {
			log.Printf("[Orchestrator] Warning: Failed to recover claim %s: %v", claim.ID, err)
			// Terminate claim with clear reason
			e.terminateClaimWithReason(ctx, claim.ID, fmt.Sprintf("Recovery failed: %v", err))
			terminatedCount++
		} else {
			recoveredCount++
		}
	}

	// Step 4: Recover grant queues
	if e.workerManager != nil {
		if err := e.recoverGrantQueues(ctx); err != nil {
			log.Printf("[Orchestrator] Warning: Failed to recover grant queues: %v", err)
			// Non-fatal - continue
		}
	}

	duration := time.Since(startTime)
	e.logEvent("recovery_complete", map[string]interface{}{
		"claims_recovered":  recoveredCount,
		"claims_terminated": terminatedCount,
		"duration_ms":       duration.Milliseconds(),
	})

	log.Printf("[Orchestrator] State recovery complete: %d claims recovered, %d terminated (duration: %v)",
		recoveredCount, terminatedCount, duration.Round(time.Millisecond))

	return nil
}

// reconstructPhaseState reconstructs in-memory phase state from persisted claim data (M3.5).
func (e *Engine) reconstructPhaseState(ctx context.Context, claim *blackboard.Claim) error {
	// Handle pending_assignment claims (feedback loops) - M3.3
	if claim.Status == blackboard.ClaimStatusPendingAssignment {
		// Track in pendingAssignmentClaims map
		e.pendingAssignmentClaims[claim.ID] = claim.ArtefactID

		e.logEvent("claim_recovered", map[string]interface{}{
			"claim_id": claim.ID,
			"status":   "pending_assignment",
		})

		log.Printf("[Orchestrator] Recovered feedback claim %s (pending_assignment)", claim.ID)
		return nil
	}

	// For phased claims, we need persisted phase state
	if claim.PhaseState == nil {
		return fmt.Errorf("claim in phased status (%s) has no persisted phase state", claim.Status)
	}

	phase := claim.PhaseState.Current
	grantedAgents := claim.PhaseState.GrantedAgents
	receivedArtefacts := claim.PhaseState.Received
	allBids := claim.PhaseState.AllBids

	// Validate granted agents still exist in config
	for _, agentName := range grantedAgents {
		if _, exists := e.agentRegistry[agentName]; !exists {
			return fmt.Errorf("granted agent '%s' no longer in config", agentName)
		}
	}

	// Reconstruct PhaseState object
	phaseState := &PhaseState{
		ClaimID:           claim.ID,
		Phase:             phase,
		GrantedAgents:     grantedAgents,
		ReceivedArtefacts: receivedArtefacts,
		AllBids:           allBids,
		StartTime:         time.UnixMilli(claim.PhaseState.StartTimeMs), // M3.9: Millisecond precision
	}

	// Store in-memory
	e.phaseStates[claim.ID] = phaseState

	// Check if we need to re-trigger grants
	if claim.ArtefactExpected && !hasReceivedAllArtefacts(phaseState) {
		// Some granted agents haven't produced output - re-trigger
		if err := e.retriggerGrant(ctx, claim, grantedAgents); err != nil {
			return fmt.Errorf("failed to re-trigger grant: %w", err)
		}
	}

	e.logEvent("claim_recovered", map[string]interface{}{
		"claim_id":        claim.ID,
		"phase":           phase,
		"granted_agents":  grantedAgents,
		"received_count":  len(receivedArtefacts),
		"retriggered":     claim.ArtefactExpected && !hasReceivedAllArtefacts(phaseState),
	})

	log.Printf("[Orchestrator] Recovered claim %s (phase: %s, granted: %v, received: %d/%d)",
		claim.ID, phase, grantedAgents, len(receivedArtefacts), len(grantedAgents))

	return nil
}

// hasReceivedAllArtefacts checks if all granted agents have produced artefacts.
func hasReceivedAllArtefacts(phaseState *PhaseState) bool {
	// Map granted agents to roles to check against received artefacts
	// This is a simplification - we check if received count matches granted count
	return len(phaseState.ReceivedArtefacts) >= len(phaseState.GrantedAgents)
}

// retriggerGrant re-triggers grant for claims that were granted but never produced output (M3.5).
func (e *Engine) retriggerGrant(ctx context.Context, claim *blackboard.Claim, grantedAgents []string) error {
	log.Printf("[Orchestrator] Re-triggering grant for claim %s (granted to: %v)", claim.ID, grantedAgents)

	// Determine grant type based on claim status/phase
	switch claim.Status {
	case blackboard.ClaimStatusPendingExclusive:
		// Exclusive grant
		agentName := grantedAgents[0] // Only one agent in exclusive phase
		agent, exists := e.config.Agents[agentName]
		if !exists {
			return fmt.Errorf("granted agent '%s' not found in config", agentName)
		}

		// Check if controller-worker
		if agent.Mode == "controller" {
			// Re-launch worker
			if e.workerManager != nil {
				return e.workerManager.LaunchWorker(ctx, claim, agentName, agent, e.client)
			}
			return fmt.Errorf("worker manager not available for controller agent")
		}

		// Traditional agent - re-publish grant notification
		return e.publishGrantNotificationWithType(ctx, agentName, claim.ID, "exclusive")

	case blackboard.ClaimStatusPendingAssignment:
		// Feedback loop claim - already assigned, just publish
		agentName := claim.GrantedExclusiveAgent
		return e.publishGrantNotificationWithType(ctx, agentName, claim.ID, "exclusive")

	case blackboard.ClaimStatusPendingReview:
		// Review - re-publish for all granted agents
		for _, agentName := range grantedAgents {
			if err := e.publishGrantNotificationWithType(ctx, agentName, claim.ID, "review"); err != nil {
				log.Printf("[Orchestrator] Failed to re-publish review grant to %s: %v", agentName, err)
			}
		}
		return nil

	case blackboard.ClaimStatusPendingParallel:
		// Parallel - re-publish for all granted agents
		for _, agentName := range grantedAgents {
			if err := e.publishGrantNotificationWithType(ctx, agentName, claim.ID, "claim"); err != nil {
				log.Printf("[Orchestrator] Failed to re-publish parallel grant to %s: %v", agentName, err)
			}
		}
		return nil

	default:
		return fmt.Errorf("unexpected claim status for re-trigger: %s", claim.Status)
	}
}

// recoverGrantQueues recovers grant queues from Redis (M3.5).
// Scans all role-specific grant queues and logs their state.
func (e *Engine) recoverGrantQueues(ctx context.Context) error {
	log.Printf("[Orchestrator] Recovering grant queues...")

	// Build set of unique roles from config
	// M3.7: Agent key IS the role
	roles := make(map[string]bool)
	for role, agent := range e.config.Agents {
		if agent.Mode == "controller" {
			roles[role] = true
		}
	}

	totalQueued := 0

	// Check each role's grant queue
	for role := range roles {
		queueKey := fmt.Sprintf("holt:%s:grant_queue:%s", e.instanceName, role)

		// Get queue size
		results, err := e.client.ZRangeWithScores(ctx, queueKey, 0, -1)
		if err != nil {
			log.Printf("[Orchestrator] Warning: Failed to read grant queue for role '%s': %v", role, err)
			continue
		}

		if len(results) > 0 {
			log.Printf("[Orchestrator] Grant queue for role '%s': %d claims", role, len(results))
			totalQueued += len(results)
		}
	}

	if totalQueued > 0 {
		e.logEvent("grant_queues_recovered", map[string]interface{}{
			"total_queued": totalQueued,
		})
		log.Printf("[Orchestrator] Recovered %d claims in grant queues", totalQueued)
	}

	return nil
}

// terminateClaimWithReason marks a claim as terminated with a specific reason (M3.5).
func (e *Engine) terminateClaimWithReason(ctx context.Context, claimID string, reason string) {
	claim, err := e.client.GetClaim(ctx, claimID)
	if err != nil {
		log.Printf("[Orchestrator] Failed to fetch claim %s for termination: %v", claimID, err)
		return
	}

	claim.Status = blackboard.ClaimStatusTerminated
	claim.TerminationReason = reason

	if err := e.client.UpdateClaim(ctx, claim); err != nil {
		log.Printf("[Orchestrator] Failed to terminate claim %s: %v", claimID, err)
		return
	}

	// Remove from tracking
	delete(e.phaseStates, claimID)
	delete(e.pendingAssignmentClaims, claimID)

	e.logEvent("claim_terminated", map[string]interface{}{
		"claim_id": claimID,
		"reason":   reason,
	})

	log.Printf("[Orchestrator] Claim %s terminated: %s", claimID, reason)
}
