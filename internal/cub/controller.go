package cub

import (
	"context"
	"log"

	"github.com/dyluth/sett/pkg/blackboard"
)

// RunControllerMode runs a controller that only bids, never executes.
// M3.4: Controllers eliminate race conditions by being the single bidder per role.
// When a controller wins a grant, the orchestrator launches ephemeral workers to execute.
func RunControllerMode(ctx context.Context, config *Config, bbClient *blackboard.Client) error {
	log.Printf("[Controller] Controller %s (role: %s) ready - bidder-only mode", config.AgentName, config.AgentRole)

	// Subscribe to claim events
	subscription, err := bbClient.SubscribeClaimEvents(ctx)
	if err != nil {
		return err
	}
	defer subscription.Close()

	log.Printf("[Controller] Subscribed to claim events")

	// Bidding loop (never executes work)
	for {
		select {
		case <-ctx.Done():
			log.Printf("[Controller] Shutting down...")
			return nil

		case claim, ok := <-subscription.Events():
			if !ok {
				log.Printf("[Controller] Claim events channel closed")
				return nil
			}

			// Evaluate claim using bidding strategy from config
			bid := config.BiddingStrategy

			// Submit bid
			if err := bbClient.SetBid(ctx, claim.ID, config.AgentName, bid); err != nil {
				log.Printf("[Controller] Failed to submit bid for claim %s: %v", claim.ID, err)
				continue
			}

			log.Printf("[Controller] Submitted bid: claim=%s type=%s status=%s", claim.ID, bid, claim.Status)

		case err, ok := <-subscription.Errors():
			if !ok {
				log.Printf("[Controller] Error channel closed")
				return nil
			}
			log.Printf("[Controller] Subscription error: %v", err)
		}
	}
}
