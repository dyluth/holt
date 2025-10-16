package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"

	"github.com/dyluth/sett/pkg/blackboard"
)

// GrantClaim determines the winning agent and grants them the claim.
// M3.1: Only processes exclusive bids with deterministic alphabetical tie-breaking.
// Review and parallel bids are collected but not granted (deferred to M3.2).
//
// Returns error if Redis operations fail. No error if no exclusive bids (claim stays pending).
func (e *Engine) GrantClaim(ctx context.Context, claim *blackboard.Claim, bids map[string]blackboard.BidType) error {
	// Collect all agents with exclusive bids
	var exclusiveBidders []string
	for agentName, bidType := range bids {
		if bidType == blackboard.BidTypeExclusive {
			exclusiveBidders = append(exclusiveBidders, agentName)
		}
	}

	// Check if any exclusive bids exist
	if len(exclusiveBidders) == 0 {
		log.Printf("[Orchestrator] No exclusive bids for claim %s (review/parallel bids not processed in M3.1), claim remains pending",
			claim.ID)
		e.logEvent("no_exclusive_bids", map[string]interface{}{
			"claim_id": claim.ID,
			"bids":     bids,
		})
		return nil
	}

	// Select winner using deterministic alphabetical ordering
	winner := SelectExclusiveWinner(exclusiveBidders)

	// Log grant decision with rationale
	if len(exclusiveBidders) == 1 {
		log.Printf("[Orchestrator] Granted exclusive to %s (only exclusive bidder) for claim %s", winner, claim.ID)
	} else {
		log.Printf("[Orchestrator] Granted exclusive to %s (selected from %d exclusive bidders: %v) for claim %s",
			winner, len(exclusiveBidders), exclusiveBidders, claim.ID)
	}

	e.logEvent("claim_granted", map[string]interface{}{
		"claim_id":          claim.ID,
		"winner":            winner,
		"exclusive_bidders": exclusiveBidders,
		"selection_method":  "alphabetical",
		"bid_count":         len(bids),
		"all_bids":          bids,
	})

	// Update claim with granted agent
	claim.GrantedExclusiveAgent = winner
	// Keep status as pending_review (M3.1 doesn't change status yet)

	if err := e.client.UpdateClaim(ctx, claim); err != nil {
		return fmt.Errorf("failed to update claim with grant: %w", err)
	}

	// Publish claim_granted event to workflow_events channel
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
