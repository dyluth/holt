package pup

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"

	"github.com/dyluth/holt/pkg/blackboard"
)

// Engine represents the core execution logic of the agent pup.
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

// New creates a new agent pup engine with the provided configuration and blackboard client.
// The engine is ready to be started but does not begin execution until Start() is called.
//
// Parameters:
//   - config: Agent pup runtime configuration (instance name, agent name, etc.)
//   - bbClient: Blackboard client for Redis operations
//
// Returns a configured Engine ready to start.
func New(config *Config, bbClient *blackboard.Client) *Engine {
	return &Engine{
		config:   config,
		bbClient: bbClient,
	}
}

// Start launches the agent pup's concurrent goroutines and blocks until context cancellation.
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
	log.Printf("[INFO] Agent pup starting for agent='%s' instance='%s'", e.config.AgentName, e.config.InstanceName)

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
// When a claim event is received, the pup always bids "exclusive" (M2.2 hardcoded strategy).
// When a grant notification is received, the pup validates it and pushes the claim to the work queue.
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
			// Handle claim event - submit bid or handle pending_assignment
			e.handleClaimEvent(ctx, claim, workQueue)

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

// handleClaimEvent processes a claim event by submitting a bid or handling pre-assigned work.
// M3.3: Detects pending_assignment claims (feedback claims) and pushes directly to work queue.
// M3.4: Uses dynamic bidding script if available, otherwise falls back to static strategy.
func (e *Engine) handleClaimEvent(ctx context.Context, claim *blackboard.Claim, workQueue chan *blackboard.Claim) {
	log.Printf("[INFO] Received claim event: claim_id=%s artefact_id=%s status=%s",
		claim.ID, claim.ArtefactID, claim.Status)

	// M3.3: Handle pending_assignment claims (feedback claims)
	if claim.Status == blackboard.ClaimStatusPendingAssignment {
		if claim.GrantedExclusiveAgent == e.config.AgentName {
			log.Printf("[INFO] Feedback claim %s is assigned to this agent, pushing to work queue", claim.ID)
			select {
			case workQueue <- claim:
				log.Printf("[DEBUG] Feedback claim %s successfully queued for execution", claim.ID)
			case <-ctx.Done():
				log.Printf("[DEBUG] Context cancelled while queuing feedback claim %s", claim.ID)
			}
		} else {
			log.Printf("[DEBUG] Feedback claim %s assigned to %s, ignoring (we are %s)",
				claim.ID, claim.GrantedExclusiveAgent, e.config.AgentName)
		}
		return // No bidding for pending_assignment claims
	}

	// Regular claim - proceed with bidding logic
	targetArtefact, err := e.bbClient.GetArtefact(ctx, claim.ArtefactID)
	if err != nil {
		log.Printf("[ERROR] Failed to fetch target artefact %s for bid decision: %v", claim.ArtefactID, err)
		return
	}
	if targetArtefact == nil {
		log.Printf("[ERROR] Target artefact %s not found for bid decision", claim.ArtefactID)
		return
	}

	// Determine bid type dynamically or from static config
	bidType, err := e.determineBidType(ctx, targetArtefact)
	if err != nil {
		log.Printf("[ERROR] Failed to determine bid type for claim %s: %v", claim.ID, err)
		// Submit an "ignore" bid as a safe default on error
		bidType = blackboard.BidTypeIgnore
	}

	err = e.bbClient.SetBid(ctx, claim.ID, e.config.AgentName, bidType)
	if err != nil {
		log.Printf("[ERROR] Failed to submit bid for claim_id=%s: %v", claim.ID, err)
		return
	}

	log.Printf("[INFO] Submitted %s bid for claim_id=%s", bidType, claim.ID)
}

// determineBidType determines the bid type for a claim. If the agent config includes a
// `bid_script`, it executes the script with the target artefact as JSON on stdin.
// The script's stdout is read as the bid type. If no script is provided, or if the
// script fails, it falls back to the static `bidding_strategy` from the config.
func (e *Engine) determineBidType(ctx context.Context, targetArtefact *blackboard.Artefact) (blackboard.BidType, error) {
	// Fallback to static bidding strategy if no bid script is defined.
	if len(e.config.BidScript) == 0 {
		return e.config.BiddingStrategy, nil
	}

	// A bid script is defined, execute it dynamically.
	log.Printf("[DEBUG] Executing bid script: %v", e.config.BidScript)

	// Prepare the command
	cmd := exec.CommandContext(ctx, e.config.BidScript[0], e.config.BidScript[1:]...)
	cmd.Dir = "/workspace"

	// Prepare stdin
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return e.handleBidScriptFailure("failed to create stdin pipe for bid script", err)
	}

	// Write artefact to stdin in a separate goroutine to avoid deadlocks
	go func() {
		defer stdin.Close()
		json.NewEncoder(stdin).Encode(targetArtefact)
	}()

	// Execute command and capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return e.handleBidScriptFailure("bid script execution failed",
			fmt.Errorf("%w\nOutput:\n%s", err, string(output)))
	}

	// Read bid type from stdout
	bidTypeStr := strings.TrimSpace(string(output))
	bidType := blackboard.BidType(bidTypeStr)

	// Validate the bid type returned by the script
	if err := bidType.Validate(); err != nil {
		return e.handleBidScriptFailure(
			fmt.Sprintf("bid script returned invalid bid type '%s'", bidTypeStr), err)
	}

	log.Printf("[DEBUG] Bid script returned: %s", bidType)
	return bidType, nil
}

// handleBidScriptFailure logs the error and returns fallback bidding strategy.
// M3.6: Implements graceful degradation when bid scripts fail.
func (e *Engine) handleBidScriptFailure(msg string, err error) (blackboard.BidType, error) {
	log.Printf("[ERROR] %s: %v", msg, err)

	// If we have a fallback strategy, use it
	if e.config.BiddingStrategy != "" {
		log.Printf("[WARN] Falling back to static bidding_strategy: %s", e.config.BiddingStrategy)
		return e.config.BiddingStrategy, nil
	}

	// No fallback available, return ignore as safe default
	log.Printf("[WARN] No fallback bidding_strategy available, returning 'ignore'")
	return blackboard.BidTypeIgnore, nil
}

// GrantNotification represents the JSON structure of grant notifications.
type GrantNotification struct {
	EventType string `json:"event_type"`
	ClaimID   string `json:"claim_id"`
	ClaimType string `json:"claim_type,omitempty"` // M3.2: "review", "claim", or "exclusive"
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
	// M3.2: Check review, parallel, and exclusive grant fields
	isGranted := false

	// Check review grants
	for _, grantedAgent := range claim.GrantedReviewAgents {
		if grantedAgent == e.config.AgentName {
			isGranted = true
			break
		}
	}

	// Check parallel grants
	if !isGranted {
		for _, grantedAgent := range claim.GrantedParallelAgents {
			if grantedAgent == e.config.AgentName {
				isGranted = true
				break
			}
		}
	}

	// Check exclusive grant
	if !isGranted && claim.GrantedExclusiveAgent == e.config.AgentName {
		isGranted = true
	}

	if !isGranted {
		log.Printf("[WARN] Grant notification for claim %s not granted to this agent (name: %s)",
			grant.ClaimID, e.config.AgentName)
		log.Printf("[DEBUG] Claim grants - review: %v, parallel: %v, exclusive: %s",
			claim.GrantedReviewAgents, claim.GrantedParallelAgents, claim.GrantedExclusiveAgent)
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
