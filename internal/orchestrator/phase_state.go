package orchestrator

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/dyluth/holt/internal/config"
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

// pauseGrantForQueue adds claim to persistent grant queue when max_concurrent reached (M3.5).
// Uses Redis ZSET for FIFO ordering based on pause timestamp.
func (e *Engine) pauseGrantForQueue(ctx context.Context, claim *blackboard.Claim, agentName string, role string) error {
	pausedAt := time.Now().Unix()

	// Update claim with queue metadata
	claim.GrantQueue = &blackboard.GrantQueue{
		PausedAt:  pausedAt,
		AgentName: agentName,
		Position:  0, // Not populated in M3.5 - ZSET score provides ordering
	}

	// Persist claim with queue metadata
	if err := e.client.UpdateClaim(ctx, claim); err != nil {
		return fmt.Errorf("failed to update claim with queue metadata: %w", err)
	}

	// Add to Redis ZSET (score = pausedAt for FIFO)
	queueKey := fmt.Sprintf("holt:%s:grant_queue:%s", e.instanceName, role)
	if err := e.client.ZAdd(ctx, queueKey, float64(pausedAt), claim.ID); err != nil {
		return fmt.Errorf("failed to add claim to grant queue: %w", err)
	}

	e.logEvent("grant_paused_for_queue", map[string]interface{}{
		"claim_id":  claim.ID,
		"role":      role,
		"agent":     agentName,
		"paused_at": pausedAt,
	})

	log.Printf("[Orchestrator] Claim %s paused in grant queue for role '%s' (max_concurrent reached)", claim.ID, role)
	return nil
}

// resumeFromQueue pops next claim from grant queue when worker slot opens (M3.5).
// Returns the resumed claim, or nil if queue is empty.
func (e *Engine) resumeFromQueue(ctx context.Context, role string) (*blackboard.Claim, error) {
	queueKey := fmt.Sprintf("holt:%s:grant_queue:%s", e.instanceName, role)

	// Get oldest claim (lowest score = earliest pause time)
	results, err := e.client.ZRangeWithScores(ctx, queueKey, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to read grant queue: %w", err)
	}

	if len(results) == 0 {
		return nil, nil // Queue empty
	}

	claimID := results[0].Member.(string)
	claim, err := e.client.GetClaim(ctx, claimID)
	if err != nil {
		if blackboard.IsNotFound(err) {
			// Claim no longer exists - remove from queue and continue
			e.client.ZRem(ctx, queueKey, claimID)
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch queued claim: %w", err)
	}

	// Remove from queue
	if err := e.client.ZRem(ctx, queueKey, claimID); err != nil {
		return nil, fmt.Errorf("failed to remove claim from queue: %w", err)
	}

	// Clear queue metadata from claim
	claim.GrantQueue = nil
	if err := e.client.UpdateClaim(ctx, claim); err != nil {
		return nil, fmt.Errorf("failed to clear queue metadata: %w", err)
	}

	e.logEvent("grant_resumed_from_queue", map[string]interface{}{
		"claim_id": claimID,
		"role":     role,
	})

	log.Printf("[Orchestrator] Resuming claim %s from grant queue for role '%s'", claimID, role)
	return claim, nil
}

// handleWorkerSlotAvailable handles queue resumption when a worker completes (M3.5).
// This is the callback invoked by WorkerManager after worker cleanup.
func (e *Engine) handleWorkerSlotAvailable(ctx context.Context, role string) {
	log.Printf("[Orchestrator] Worker slot available for role '%s', checking grant queue", role)

	// Try to resume next claim from queue
	claim, err := e.resumeFromQueue(ctx, role)
	if err != nil {
		log.Printf("[Orchestrator] Error resuming from grant queue for role '%s': %v", role, err)
		return
	}

	if claim == nil {
		log.Printf("[Orchestrator] No claims in grant queue for role '%s'", role)
		return
	}

	// Grant to agent stored in GrantQueue.AgentName (before we cleared it)
	// We need to fetch the agent info from config
	var agentName string
	var agent config.Agent
	var found bool

	// Find the agent by role
	for name, a := range e.config.Agents {
		if a.Role == role {
			agentName = name
			agent = a
			found = true
			break
		}
	}

	if !found {
		log.Printf("[Orchestrator] No agent found for role '%s', cannot resume claim %s", role, claim.ID)
		return
	}

	// Grant exclusive phase (launch worker)
	log.Printf("[Orchestrator] Resuming grant for claim %s to agent %s (role: %s)", claim.ID, agentName, role)

	// Update claim status and granted agent
	claim.GrantedExclusiveAgent = agentName
	claim.Status = blackboard.ClaimStatusPendingExclusive

	if err := e.client.UpdateClaim(ctx, claim); err != nil {
		log.Printf("[Orchestrator] Failed to update resumed claim: %v", err)
		return
	}

	// Launch worker
	if e.workerManager != nil {
		if err := e.workerManager.LaunchWorker(ctx, claim, agentName, agent, e.client); err != nil {
			log.Printf("[Orchestrator] Failed to launch worker for resumed claim: %v", err)

			// Terminate claim
			claim.Status = blackboard.ClaimStatusTerminated
			claim.TerminationReason = fmt.Sprintf("Failed to launch worker after queue resumption: %v", err)
			e.client.UpdateClaim(ctx, claim)
		}
	}
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
