package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/dyluth/holt/internal/config"
	"github.com/dyluth/holt/pkg/blackboard"
	"github.com/google/uuid"
)

// Engine is the core orchestrator that watches for artefacts and creates claims.
// It implements the event-driven coordination logic for Phase 1.
type Engine struct {
	client                  *blackboard.Client
	instanceName            string
	config                  *config.HoltConfig // M3.3: Need config for max_review_iterations
	healthServer            *HealthServer
	agentRegistry           map[string]string      // agent_name -> agent_role
	phaseStates             map[string]*PhaseState // claimID -> PhaseState (M3.2: in-memory tracking)
	pendingAssignmentClaims map[string]string      // claimID -> targetArtefactID (M3.3: feedback claim tracking)
	workerManager           *WorkerManager         // M3.4: Worker lifecycle management
}

// NewEngine creates a new orchestrator engine.
// Config is required in M2.2+ to build the agent registry for consensus.
// M3.4: workerManager can be nil if Docker socket is not available (workers disabled)
func NewEngine(client *blackboard.Client, instanceName string, cfg *config.HoltConfig, workerManager *WorkerManager) *Engine {
	// Build agent registry from config
	agentRegistry := make(map[string]string)
	if cfg != nil {
		for agentName, agent := range cfg.Agents {
			agentRegistry[agentName] = agent.Role
		}
	}

	return &Engine{
		client:                  client,
		instanceName:            instanceName,
		config:                  cfg, // M3.3: Store config for feedback loop logic
		healthServer:            NewHealthServer(client),
		agentRegistry:           agentRegistry,
		phaseStates:             make(map[string]*PhaseState), // M3.2: Initialize phase state tracking
		pendingAssignmentClaims: make(map[string]string),      // M3.3: Initialize feedback claim tracking
		workerManager:           workerManager,                // M3.4: Worker lifecycle management
	}
}

// Run starts the orchestrator engine and blocks until context is cancelled.
// Returns error if subscription or processing fails.
func (e *Engine) Run(ctx context.Context) error {
	// Start health check server
	if err := e.healthServer.Start(); err != nil {
		return fmt.Errorf("failed to start health server: %w", err)
	}
	defer e.healthServer.Shutdown(context.Background())

	log.Printf("[Orchestrator] Starting for instance '%s'", e.instanceName)

	// Subscribe to artefact events
	subscription, err := e.client.SubscribeArtefactEvents(ctx)
	if err != nil {
		return fmt.Errorf("failed to subscribe to artefact events: %w", err)
	}
	defer subscription.Close()

	log.Printf("[Orchestrator] Subscribed to artefact_events")

	// Process events until context is cancelled
	for {
		select {
		case <-ctx.Done():
			log.Printf("[Orchestrator] Shutting down...")
			return nil

		case artefact, ok := <-subscription.Events():
			if !ok {
				log.Printf("[Orchestrator] Subscription closed")
				return nil
			}

			e.logEvent("artefact_received", map[string]interface{}{
				"artefact_id":     artefact.ID,
				"type":            artefact.Type,
				"structural_type": artefact.StructuralType,
			})

			if err := e.processArtefact(ctx, artefact); err != nil {
				log.Printf("[Orchestrator] Error processing artefact %s: %v", artefact.ID, err)
				// Continue processing - don't crash on single artefact failure
			}

			// M3.2: Also process artefact for phase completion tracking
			e.processArtefactForPhases(ctx, artefact)

		case err, ok := <-subscription.Errors():
			if !ok {
				log.Printf("[Orchestrator] Error channel closed")
				return nil
			}
			log.Printf("[Orchestrator] Subscription error: %v", err)
			// Continue processing - errors are non-fatal
		}
	}
}

// processArtefact handles a single artefact event.
// Creates a claim if appropriate, or skips if Terminal or Failure type.
func (e *Engine) processArtefact(ctx context.Context, artefact *blackboard.Artefact) error {
	// Check if this is a Terminal artefact
	if artefact.StructuralType == blackboard.StructuralTypeTerminal {
		e.logEvent("terminal_skipped", map[string]interface{}{
			"artefact_id": artefact.ID,
			"type":        artefact.Type,
		})
		return nil
	}

	// Check if this is a Failure artefact (terminates workflow)
	if artefact.StructuralType == blackboard.StructuralTypeFailure {
		e.logEvent("failure_skipped", map[string]interface{}{
			"artefact_id": artefact.ID,
			"type":        artefact.Type,
		})
		return nil
	}

	// Check if a claim already exists (idempotency)
	existingClaim, err := e.client.GetClaimByArtefactID(ctx, artefact.ID)
	if err != nil && !blackboard.IsNotFound(err) {
		return fmt.Errorf("failed to check for existing claim: %w", err)
	}

	if existingClaim != nil {
		e.logEvent("duplicate_artefact", map[string]interface{}{
			"artefact_id":       artefact.ID,
			"existing_claim_id": existingClaim.ID,
		})
		return nil
	}

	// Create new claim
	startTime := time.Now()
	claimID := uuid.New().String()

	claim := &blackboard.Claim{
		ID:         claimID,
		ArtefactID: artefact.ID,
		Status:     blackboard.ClaimStatusPendingReview,
		// Phase 1: No granted agents (bidding in Phase 2)
		GrantedReviewAgents:   []string{},
		GrantedParallelAgents: []string{},
		GrantedExclusiveAgent: "",
	}

	if err := e.client.CreateClaim(ctx, claim); err != nil {
		return fmt.Errorf("failed to create claim: %w", err)
	}

	latencyMs := time.Since(startTime).Milliseconds()

	e.logEvent("claim_created", map[string]interface{}{
		"artefact_id": artefact.ID,
		"claim_id":    claimID,
		"status":      string(claim.Status),
		"latency_ms":  latencyMs,
	})

	// M3.1: Wait for consensus and grant claim
	if len(e.agentRegistry) > 0 {
		if err := e.waitForConsensusAndGrant(ctx, claim); err != nil {
			log.Printf("[Orchestrator] Error in consensus/granting for claim %s: %v", claimID, err)
			// Don't return error - continue processing other artefacts
		}
	}

	return nil
}

// processArtefactForPhases checks if this artefact completes a phase for any active claims.
// M3.2: Tracks artefacts produced by granted agents and triggers phase completion checks.
// M3.3: Also handles pending_assignment claims (feedback claims).
func (e *Engine) processArtefactForPhases(ctx context.Context, artefact *blackboard.Artefact) {
	// Skip non-phase-relevant artefacts
	if artefact.StructuralType == blackboard.StructuralTypeTerminal ||
		artefact.StructuralType == blackboard.StructuralTypeFailure {
		return
	}

	// M3.3: Check for pending_assignment claims (feedback claims)
	// These don't use phaseStates - they complete immediately when agent produces artefact
	e.checkPendingAssignmentClaims(ctx, artefact)

	// Find claims waiting for artefacts from this producer (phased claims)
	for claimID, phaseState := range e.phaseStates {
		claim, err := e.client.GetClaim(ctx, claimID)
		if err != nil {
			log.Printf("[Orchestrator] Error fetching claim %s: %v", claimID, err)
			continue
		}

		// Check if this artefact is derived from the claim's target artefact
		if !isSourceOfClaim(artefact, claim.ArtefactID) {
			continue
		}

		// Check if this artefact is from a granted agent in the current phase
		if !e.isProducedByGrantedAgent(claim, artefact.ProducedByRole, phaseState.Phase) {
			continue
		}

		// Track this artefact as received
		phaseState.ReceivedArtefacts[artefact.ProducedByRole] = artefact.ID

		log.Printf("[Orchestrator] Phase %s artefact received for claim %s: producer=%s, artefact=%s",
			phaseState.Phase, claim.ID, artefact.ProducedByRole, artefact.ID)

		e.logEvent("phase_artefact_received", map[string]interface{}{
			"claim_id":    claim.ID,
			"phase":       phaseState.Phase,
			"agent_role":  artefact.ProducedByRole,
			"artefact_id": artefact.ID,
		})

		// Check phase completion
		e.checkPhaseCompletion(ctx, claim, phaseState, artefact)
	}
}

// isSourceOfClaim checks if an artefact is derived from the claim's target artefact.
func isSourceOfClaim(artefact *blackboard.Artefact, claimArtefactID string) bool {
	for _, sourceID := range artefact.SourceArtefacts {
		if sourceID == claimArtefactID {
			return true
		}
	}
	return false
}

// isProducedByGrantedAgent checks if the artefact's producer role matches any granted agent.
// Uses the engine's agent registry to map agent names to roles.
func (e *Engine) isProducedByGrantedAgent(claim *blackboard.Claim, producerRole string, phase string) bool {
	var grantedAgentNames []string

	switch phase {
	case "review":
		grantedAgentNames = claim.GrantedReviewAgents
	case "parallel":
		grantedAgentNames = claim.GrantedParallelAgents
	case "exclusive":
		// For exclusive, check if the granted agent's role matches
		if claim.GrantedExclusiveAgent == "" {
			return false
		}
		grantedAgentNames = []string{claim.GrantedExclusiveAgent}
	default:
		return false
	}

	// Map agent names to roles and check if any match the producer role
	for _, agentName := range grantedAgentNames {
		// Look up the agent's role in the registry
		agentRole, exists := e.agentRegistry[agentName]
		if exists && agentRole == producerRole {
			return true
		}
	}

	return false
}

// checkPhaseCompletion checks if a phase is complete and triggers appropriate logic.
func (e *Engine) checkPhaseCompletion(ctx context.Context, claim *blackboard.Claim, phaseState *PhaseState, artefact *blackboard.Artefact) {
	switch phaseState.Phase {
	case "review":
		if err := e.CheckReviewPhaseCompletion(ctx, claim, phaseState); err != nil {
			log.Printf("[Orchestrator] Error checking review phase completion: %v", err)
		}

	case "parallel":
		if err := e.CheckParallelPhaseCompletion(ctx, claim, phaseState); err != nil {
			log.Printf("[Orchestrator] Error checking parallel phase completion: %v", err)
		}

	case "exclusive":
		// Exclusive phase completes immediately when artefact is received
		log.Printf("[Orchestrator] Exclusive phase complete for claim %s", claim.ID)
		if err := e.TransitionToNextPhase(ctx, claim, phaseState); err != nil {
			log.Printf("[Orchestrator] Error transitioning from exclusive phase: %v", err)
		}
	}
}

// waitForConsensusAndGrant orchestrates the full consensus and granting process.
// Uses the new M3.1 consensus and granting logic with bid tracking and alphabetical tie-breaking.
func (e *Engine) waitForConsensusAndGrant(ctx context.Context, claim *blackboard.Claim) error {
	// Wait for full consensus (all agents bid)
	bids, err := e.WaitForConsensus(ctx, claim.ID)
	if err != nil {
		return fmt.Errorf("failed to achieve consensus: %w", err)
	}

	// Grant claim using deterministic selection
	if err := e.GrantClaim(ctx, claim, bids); err != nil {
		return fmt.Errorf("failed to grant claim: %w", err)
	}

	return nil
}

// getAgentsStillToSubmitBids returns a list of agent names that haven't submitted bids yet.
func (e *Engine) getAgentsStillToSubmitBids(receivedBids map[string]blackboard.BidType) []string {
	var waiting []string
	for agentName := range e.agentRegistry {
		if _, hasBid := receivedBids[agentName]; !hasBid {
			waiting = append(waiting, agentName)
		}
	}
	return waiting
}

// logEvent logs a structured event in JSON format.
func (e *Engine) logEvent(eventType string, data map[string]interface{}) {
	data["timestamp"] = time.Now().UTC().Format(time.RFC3339)
	data["level"] = "info"
	data["component"] = "orchestrator"
	data["event_type"] = eventType
	data["instance"] = e.instanceName

	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Printf("[Orchestrator] Failed to marshal log event: %v", err)
		return
	}

	log.Println(string(jsonData))
}

// checkPendingAssignmentClaims checks if an artefact completes any pending_assignment claims.
// M3.3: Feedback claims complete immediately when the granted agent produces an artefact.
func (e *Engine) checkPendingAssignmentClaims(ctx context.Context, artefact *blackboard.Artefact) {
	for claimID, targetArtefactID := range e.pendingAssignmentClaims {
		// Check if this artefact is derived from the claim's target artefact
		if !isSourceOfClaim(artefact, targetArtefactID) {
			continue
		}

		// Fetch the claim
		claim, err := e.client.GetClaim(ctx, claimID)
		if err != nil {
			log.Printf("[Orchestrator] Error fetching pending_assignment claim %s: %v", claimID, err)
			continue
		}

		// Verify claim is still pending_assignment (defensive check)
		if claim.Status != blackboard.ClaimStatusPendingAssignment {
			log.Printf("[Orchestrator] Warning: claim %s is not pending_assignment (status=%s), removing from tracking",
				claimID, claim.Status)
			delete(e.pendingAssignmentClaims, claimID)
			continue
		}

		// Check if artefact is produced by the granted agent
		grantedAgentRole, exists := e.agentRegistry[claim.GrantedExclusiveAgent]
		if !exists {
			log.Printf("[Orchestrator] Warning: granted agent %s not found in registry for claim %s",
				claim.GrantedExclusiveAgent, claimID)
			continue
		}

		if artefact.ProducedByRole != grantedAgentRole {
			// Artefact not from granted agent, continue waiting
			continue
		}

		// Artefact received from granted agent - mark claim as complete
		claim.Status = blackboard.ClaimStatusComplete
		if err := e.client.UpdateClaim(ctx, claim); err != nil {
			log.Printf("[Orchestrator] Error updating claim %s to complete: %v", claimID, err)
			continue
		}

		// Remove from tracking
		delete(e.pendingAssignmentClaims, claimID)

		e.logEvent("feedback_claim_complete", map[string]interface{}{
			"claim_id":    claimID,
			"artefact_id": artefact.ID,
			"agent":       claim.GrantedExclusiveAgent,
		})

		log.Printf("[Orchestrator] Feedback claim %s completed by agent %s (artefact: %s)",
			claimID, claim.GrantedExclusiveAgent, artefact.ID)
	}
}
