package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/dyluth/sett/internal/config"
	"github.com/dyluth/sett/pkg/blackboard"
	"github.com/google/uuid"
)

// Engine is the core orchestrator that watches for artefacts and creates claims.
// It implements the event-driven coordination logic for Phase 1.
type Engine struct {
	client        *blackboard.Client
	instanceName  string
	healthServer  *HealthServer
	agentRegistry map[string]string // agent_name -> agent_role
}

// NewEngine creates a new orchestrator engine.
// Config is required in M2.2+ to build the agent registry for consensus.
func NewEngine(client *blackboard.Client, instanceName string, cfg *config.SettConfig) *Engine {
	// Build agent registry from config
	agentRegistry := make(map[string]string)
	if cfg != nil {
		for agentName, agent := range cfg.Agents {
			agentRegistry[agentName] = agent.Role
		}
	}

	return &Engine{
		client:        client,
		instanceName:  instanceName,
		healthServer:  NewHealthServer(client),
		agentRegistry: agentRegistry,
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
				"artefact_id": artefact.ID,
				"type":        artefact.Type,
				"structural_type": artefact.StructuralType,
			})

			if err := e.processArtefact(ctx, artefact); err != nil {
				log.Printf("[Orchestrator] Error processing artefact %s: %v", artefact.ID, err)
				// Continue processing - don't crash on single artefact failure
			}

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

	// M2.2: Wait for consensus and grant claim
	if len(e.agentRegistry) > 0 {
		if err := e.waitForConsensusAndGrant(ctx, claim); err != nil {
			log.Printf("[Orchestrator] Error in consensus/granting for claim %s: %v", claimID, err)
			// Don't return error - continue processing other artefacts
		}
	}

	return nil
}

// waitForConsensusAndGrant implements the consensus polling and claim granting logic.
// Polls for bids every 100ms until all known agents have submitted bids (full consensus).
// Once consensus is reached, grants the claim to the winning agent and publishes notification.
func (e *Engine) waitForConsensusAndGrant(ctx context.Context, claim *blackboard.Claim) error {
	log.Printf("[Orchestrator] Waiting for consensus on claim_id=%s", claim.ID)

	expectedBidCount := len(e.agentRegistry)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	consensusStart := time.Now()
	var lastLogTime time.Time

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting for consensus")

		case <-ticker.C:
			// Poll for bids
			bids, err := e.client.GetAllBids(ctx, claim.ID)
			if err != nil {
				return fmt.Errorf("failed to get bids: %w", err)
			}

			receivedBidCount := len(bids)

			// Log warning every 10 seconds if still waiting
			if time.Since(lastLogTime) >= 10*time.Second {
				waitingFor := e.getAgentsStillToSubmitBids(bids)
				log.Printf("[Orchestrator] Still waiting for bids from: %v (waited %v)",
					waitingFor, time.Since(consensusStart).Round(time.Second))
				lastLogTime = time.Now()
			}

			// Check if consensus achieved
			if receivedBidCount == expectedBidCount {
				consensusDuration := time.Since(consensusStart)
				log.Printf("[Orchestrator] Consensus achieved for claim_id=%s: received %d/%d bids (took %v)",
					claim.ID, receivedBidCount, expectedBidCount, consensusDuration.Round(time.Millisecond))

				e.logEvent("consensus_achieved", map[string]interface{}{
					"claim_id":           claim.ID,
					"bid_count":          receivedBidCount,
					"consensus_duration": consensusDuration.Milliseconds(),
				})

				// Grant claim to winner
				return e.grantClaim(ctx, claim, bids)
			}
		}
	}
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

// grantClaim determines the winning agent and grants them the claim.
// M2.2 strategy: First agent with "exclusive" bid wins.
// Updates claim in Redis and publishes grant notification.
func (e *Engine) grantClaim(ctx context.Context, claim *blackboard.Claim, bids map[string]blackboard.BidType) error {
	// Find first exclusive bidder (M2.2 simple strategy)
	var winner string
	for agentName, bidType := range bids {
		if bidType == blackboard.BidTypeExclusive {
			winner = agentName
			break
		}
	}

	if winner == "" {
		log.Printf("[Orchestrator] No exclusive bids for claim %s, ignoring", claim.ID)
		e.logEvent("no_exclusive_bids", map[string]interface{}{
			"claim_id": claim.ID,
			"bids":     bids,
		})
		return nil
	}

	// Update claim with granted agent
	claim.GrantedExclusiveAgent = winner
	// Keep status as pending_review (M2.2 doesn't change status yet)

	if err := e.client.UpdateClaim(ctx, claim); err != nil {
		return fmt.Errorf("failed to update claim with grant: %w", err)
	}

	log.Printf("[Orchestrator] Granted claim_id=%s to agent '%s'", claim.ID, winner)

	e.logEvent("claim_granted", map[string]interface{}{
		"claim_id":   claim.ID,
		"winner":     winner,
		"bid_count":  len(bids),
		"bid_types":  bids,
	})

	// Publish claim_granted event to workflow_events channel (M2.6)
	if err := e.publishClaimGrantedEvent(ctx, claim, winner); err != nil {
		// Log error but don't fail the grant (best-effort delivery)
		log.Printf("[Orchestrator] Failed to publish claim_granted event: %v", err)
	}

	// Publish grant notification to agent's channel
	if err := e.publishGrantNotification(ctx, winner, claim.ID); err != nil {
		log.Printf("[Orchestrator] Failed to publish grant notification: %v", err)
		// Not a fatal error - claim is still granted in Redis
		return nil
	}

	return nil
}

// publishGrantNotification publishes a grant notification to the agent's event channel.
func (e *Engine) publishGrantNotification(ctx context.Context, agentName, claimID string) error {
	notification := map[string]string{
		"event_type": "grant",
		"claim_id":   claimID,
	}

	notificationJSON, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal grant notification: %w", err)
	}

	channel := blackboard.AgentEventsChannel(e.instanceName, agentName)

	log.Printf("[Orchestrator] Publishing grant notification to %s: claim_id=%s", channel, claimID)

	if err := e.client.PublishRaw(ctx, channel, string(notificationJSON)); err != nil {
		return fmt.Errorf("failed to publish grant notification: %w", err)
	}

	e.logEvent("grant_notification_published", map[string]interface{}{
		"claim_id":   claimID,
		"agent_name": agentName,
		"channel":    channel,
	})

	return nil
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
