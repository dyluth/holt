package orchestrator

import (
	"context"
	"log"
	"time"

	"github.com/dyluth/sett/pkg/blackboard"
)

// WaitForConsensus implements the full consensus bidding model for M3.1.
// Polls for bids every 100ms until all known agents have submitted bids.
// Tracks which bids have been received and logs each new bid arrival.
// Logs periodic waiting messages every 5 seconds if consensus not achieved.
//
// Returns:
//   - map[string]blackboard.BidType: All bids received (agent_name -> bid_type)
//   - error: If context cancelled or Redis error
func (e *Engine) WaitForConsensus(ctx context.Context, claimID string) (map[string]blackboard.BidType, error) {
	log.Printf("[Orchestrator] Waiting for consensus on claim_id=%s (expecting %d bids)", claimID, len(e.agentRegistry))

	expectedBidCount := len(e.agentRegistry)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	consensusStart := time.Now()
	lastLogTime := time.Now()
	seenBids := make(map[string]bool) // Track which agents we've logged bids for

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()

		case <-ticker.C:
			// Poll for all bids
			bids, err := e.client.GetAllBids(ctx, claimID)
			if err != nil {
				return nil, err
			}

			// Log any new bids that have arrived since last check
			for agentName, bidType := range bids {
				if !seenBids[agentName] {
					// New bid detected - log it
					e.logBidArrival(claimID, agentName, bidType)
					seenBids[agentName] = true
				}
			}

			// Check if consensus achieved
			receivedBidCount := len(bids)
			if receivedBidCount == expectedBidCount {
				consensusDuration := time.Since(consensusStart)
				log.Printf("[Orchestrator] Consensus achieved for claim_id=%s: received %d/%d bids (took %v)",
					claimID, receivedBidCount, expectedBidCount, consensusDuration.Round(time.Millisecond))

				e.logEvent("consensus_achieved", map[string]interface{}{
					"claim_id":           claimID,
					"bid_count":          receivedBidCount,
					"consensus_duration": consensusDuration.Milliseconds(),
				})

				// Validate and sanitize bids before returning
				return e.validateAndSanitizeBids(claimID, bids), nil
			}

			// Log periodic waiting messages every 5 seconds
			if time.Since(lastLogTime) >= 5*time.Second {
				waitingFor := e.getAgentsStillToSubmitBids(bids)
				log.Printf("[Orchestrator] Waiting for bids from: %v (waited %v)",
					waitingFor, time.Since(consensusStart).Round(time.Second))
				lastLogTime = time.Now()
			}
		}
	}
}

// logBidArrival logs a single bid arrival event.
func (e *Engine) logBidArrival(claimID, agentName string, bidType blackboard.BidType) {
	log.Printf("[Orchestrator] Received %s bid from %s for claim %s", bidType, agentName, claimID)
	e.logEvent("bid_received", map[string]interface{}{
		"claim_id":   claimID,
		"agent_name": agentName,
		"bid_type":   string(bidType),
	})
}

// validateAndSanitizeBids checks each bid for validity and treats invalid bids as "ignore".
// Invalid bids are logged with warnings but do not block consensus.
//
// Returns a sanitized map where all invalid bids have been converted to BidTypeIgnore.
func (e *Engine) validateAndSanitizeBids(claimID string, bids map[string]blackboard.BidType) map[string]blackboard.BidType {
	sanitized := make(map[string]blackboard.BidType)

	for agentName, bidType := range bids {
		// Validate bid type
		if err := bidType.Validate(); err != nil {
			// Invalid bid - treat as ignore and log warning
			log.Printf("[Orchestrator] WARN: Agent %s submitted invalid bid type '%s' for claim %s, treating as 'ignore'",
				agentName, bidType, claimID)

			e.logEvent("invalid_bid", map[string]interface{}{
				"claim_id":      claimID,
				"agent_name":    agentName,
				"bid_type":      string(bidType),
				"action_taken":  "treated_as_ignore",
			})

			sanitized[agentName] = blackboard.BidTypeIgnore
		} else {
			// Valid bid
			sanitized[agentName] = bidType
		}
	}

	return sanitized
}
