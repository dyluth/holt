package cub

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"github.com/dyluth/sett/pkg/blackboard"
)

// Engine represents the core execution logic of the agent cub.
// It manages two concurrent goroutines:
//   - Claim Watcher: Monitors for new claims and evaluates bidding opportunities (M2.2+)
//   - Work Executor: Executes granted work and posts results (M2.3+)
//
// The engine coordinates these goroutines via a work queue channel and
// handles graceful shutdown through context cancellation.
type Engine struct {
	config   *Config
	bbClient *blackboard.Client
	wg       sync.WaitGroup
}

// New creates a new agent cub engine with the provided configuration and blackboard client.
// The engine is ready to be started but does not begin execution until Start() is called.
//
// Parameters:
//   - config: Agent cub runtime configuration (instance name, agent name, etc.)
//   - bbClient: Blackboard client for Redis operations
//
// Returns a configured Engine ready to start.
func New(config *Config, bbClient *blackboard.Client) *Engine {
	return &Engine{
		config:   config,
		bbClient: bbClient,
	}
}

// Start launches the agent cub's concurrent goroutines and blocks until context cancellation.
// Creates a work queue channel and starts both the Claim Watcher and Work Executor goroutines.
//
// The method blocks until:
//   - The provided context is cancelled (normal shutdown)
//   - All goroutines complete their shutdown sequence
//
// Graceful shutdown sequence:
//  1. Context is cancelled (typically via SIGTERM signal)
//  2. Both goroutines detect cancellation via select on ctx.Done()
//  3. Goroutines exit their loops and perform cleanup
//  4. Start() returns once all goroutines complete
//
// Returns nil when shutdown completes successfully.
func (e *Engine) Start(ctx context.Context) error {
	log.Printf("[INFO] Agent cub starting for agent='%s' instance='%s'", e.config.AgentName, e.config.InstanceName)

	// Create work queue with buffer size 1
	// Buffer size 1 allows Claim Watcher to post one claim without blocking
	workQueue := make(chan *blackboard.Claim, 1)

	// Launch Claim Watcher goroutine
	e.wg.Add(1)
	go e.claimWatcher(ctx, workQueue)

	// Launch Work Executor goroutine
	e.wg.Add(1)
	go e.workExecutor(ctx, workQueue)

	// Wait for context cancellation
	<-ctx.Done()
	log.Printf("[INFO] Shutdown signal received, initiating graceful shutdown")

	// Close work queue to signal Work Executor that no more work will arrive
	close(workQueue)

	// Wait for all goroutines to complete
	e.wg.Wait()
	log.Printf("[INFO] All goroutines exited, shutdown complete")

	return nil
}

// claimWatcher monitors for new claims and grant notifications.
// Implements dual-subscription pattern:
//  1. Subscribes to claim_events - receives all new claims, submits bids
//  2. Subscribes to agent:{name}:events - receives grant notifications from orchestrator
//
// When a claim event is received, the cub always bids "exclusive" (M2.2 hardcoded strategy).
// When a grant notification is received, the cub validates it and pushes the claim to the work queue.
//
// The goroutine runs until the context is cancelled, then exits cleanly.
func (e *Engine) claimWatcher(ctx context.Context, workQueue chan *blackboard.Claim) {
	defer e.wg.Done()
	defer log.Printf("[DEBUG] Claim Watcher exited cleanly")

	log.Printf("[DEBUG] Claim Watcher starting")

	// Subscribe to claim events
	claimSub, err := e.bbClient.SubscribeClaimEvents(ctx)
	if err != nil {
		log.Printf("[ERROR] Failed to subscribe to claim events: %v", err)
		return
	}
	defer claimSub.Close()

	// Subscribe to agent-specific grant notifications
	agentChannel := blackboard.AgentEventsChannel(e.config.InstanceName, e.config.AgentName)
	grantSub, err := e.bbClient.SubscribeRawChannel(ctx, agentChannel)
	if err != nil {
		log.Printf("[ERROR] Failed to subscribe to agent events channel: %v", err)
		return
	}
	defer grantSub.Close()

	log.Printf("[INFO] Claim Watcher subscribed to claim_events and %s", agentChannel)

	// Dual-subscription select loop
	for {
		select {
		case <-ctx.Done():
			// Context cancelled - shutdown requested
			log.Printf("[DEBUG] Claim Watcher received shutdown signal")
			return

		case claim, ok := <-claimSub.Events():
			if !ok {
				// Claim events channel closed
				log.Printf("[WARN] Claim events channel closed")
				return
			}
			// Handle claim event - submit bid
			e.handleClaimEvent(ctx, claim)

		case grantMsg, ok := <-grantSub.Messages():
			if !ok {
				// Grant events channel closed
				log.Printf("[WARN] Grant events channel closed")
				return
			}
			// Handle grant notification - validate and push to work queue
			e.handleGrantNotification(ctx, grantMsg, workQueue)

		case err, ok := <-claimSub.Errors():
			if !ok {
				log.Printf("[WARN] Claim subscription error channel closed")
				return
			}
			log.Printf("[ERROR] Claim subscription error: %v", err)
			// Continue processing - errors are non-fatal
		}
	}
}

// handleClaimEvent processes a claim event by submitting a bid.
// Determines bid type based on artefact type to avoid infinite loops.
func (e *Engine) handleClaimEvent(ctx context.Context, claim *blackboard.Claim) {
	log.Printf("[INFO] Received claim event: claim_id=%s artefact_id=%s", claim.ID, claim.ArtefactID)

	// Fetch the artefact to determine if we're interested
	artefact, err := e.bbClient.GetArtefact(ctx, claim.ArtefactID)
	if err != nil {
		log.Printf("[ERROR] Failed to fetch artefact %s for claim %s: %v", claim.ArtefactID, claim.ID, err)
		// Bid ignore if we can't fetch the artefact
		e.bbClient.SetBid(ctx, claim.ID, e.config.AgentName, blackboard.BidTypeIgnore)
		return
	}

	// Determine bid type based on artefact type
	var bidType blackboard.BidType
	if artefact.Type == "GoalDefined" {
		// We're interested in GoalDefined artefacts
		bidType = blackboard.BidTypeExclusive
		log.Printf("[INFO] Artefact type=GoalDefined, bidding exclusive")
	} else {
		// Ignore all other artefact types (including our own CodeCommit outputs)
		bidType = blackboard.BidTypeIgnore
		log.Printf("[INFO] Artefact type=%s, bidding ignore", artefact.Type)
	}

	err = e.bbClient.SetBid(ctx, claim.ID, e.config.AgentName, bidType)
	if err != nil {
		log.Printf("[ERROR] Failed to submit bid for claim_id=%s: %v", claim.ID, err)
		// Continue watching - don't crash on bid failure
		return
	}

	log.Printf("[INFO] Submitted %s bid for claim_id=%s", bidType, claim.ID)
}

// GrantNotification represents the JSON structure of grant notifications.
type GrantNotification struct {
	EventType string `json:"event_type"`
	ClaimID   string `json:"claim_id"`
}

// handleGrantNotification processes a grant notification from the orchestrator.
// Validates that the claim is actually granted to this agent, then pushes to work queue.
func (e *Engine) handleGrantNotification(ctx context.Context, msgPayload string, workQueue chan *blackboard.Claim) {
	// Parse grant notification JSON
	var grant GrantNotification
	if err := json.Unmarshal([]byte(msgPayload), &grant); err != nil {
		log.Printf("[WARN] Failed to parse grant notification: %v", err)
		return
	}

	if grant.EventType != "grant" {
		log.Printf("[WARN] Unexpected event_type in grant notification: %s", grant.EventType)
		return
	}

	log.Printf("[INFO] Received grant notification: claim_id=%s", grant.ClaimID)

	// Fetch full claim from blackboard
	claim, err := e.bbClient.GetClaim(ctx, grant.ClaimID)
	if err != nil {
		log.Printf("[ERROR] Failed to fetch claim %s: %v", grant.ClaimID, err)
		return
	}

	// Security check: Verify claim is actually granted to this agent
	if claim.GrantedExclusiveAgent != e.config.AgentName {
		log.Printf("[WARN] Grant notification for claim %s not granted to this agent (granted to: %s)",
			grant.ClaimID, claim.GrantedExclusiveAgent)
		return
	}

	log.Printf("[INFO] Grant validated for claim_id=%s, pushing to work queue", grant.ClaimID)

	// Push claim to work queue (buffered channel, may block briefly if queue full)
	select {
	case workQueue <- claim:
		log.Printf("[DEBUG] Claim %s successfully queued for execution", claim.ID)
	case <-ctx.Done():
		log.Printf("[DEBUG] Context cancelled while queuing claim %s", claim.ID)
		return
	}
}

// workExecutor receives granted claims from the work queue and executes them.
// M2.3: Executes agent tools via subprocess, creates result artefacts.
//
// The goroutine runs until:
//   - The context is cancelled (shutdown signal)
//   - The work queue channel is closed (no more work will arrive)
//
// Work execution never crashes - all errors create Failure artefacts and continue processing.
func (e *Engine) workExecutor(ctx context.Context, workQueue chan *blackboard.Claim) {
	defer e.wg.Done()
	defer log.Printf("[DEBUG] Work Executor exited cleanly")

	log.Printf("[DEBUG] Work Executor starting")

	for {
		select {
		case <-ctx.Done():
			// Context cancelled - shutdown requested
			log.Printf("[DEBUG] Work Executor received shutdown signal")
			return

		case claim, ok := <-workQueue:
			if !ok {
				// Work queue closed - no more work will arrive
				log.Printf("[DEBUG] Work queue closed, Work Executor shutting down")
				return
			}

			// Execute work for this claim
			// Note: executeWork handles all errors internally and never panics
			e.executeWork(ctx, claim)
		}
	}
}
