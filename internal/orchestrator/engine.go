package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/dyluth/sett/pkg/blackboard"
	"github.com/google/uuid"
)

// Engine is the core orchestrator that watches for artefacts and creates claims.
// It implements the event-driven coordination logic for Phase 1.
type Engine struct {
	client       *blackboard.Client
	instanceName string
	healthServer *HealthServer
}

// NewEngine creates a new orchestrator engine.
func NewEngine(client *blackboard.Client, instanceName string) *Engine {
	return &Engine{
		client:       client,
		instanceName: instanceName,
		healthServer: NewHealthServer(client),
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
// Creates a claim if appropriate, or skips if Terminal type.
func (e *Engine) processArtefact(ctx context.Context, artefact *blackboard.Artefact) error {
	// Check if this is a Terminal artefact
	if artefact.StructuralType == blackboard.StructuralTypeTerminal {
		e.logEvent("terminal_skipped", map[string]interface{}{
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
