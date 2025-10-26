package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/dyluth/holt/pkg/blackboard"
)

// GrantReviewPhase grants the claim to all agents that bid "review".
// Updates the claim's GrantedReviewAgents field and keeps status as pending_review.
func (e *Engine) GrantReviewPhase(ctx context.Context, claim *blackboard.Claim, bids map[string]blackboard.BidType) error {
	// Collect all agents with review bids
	var reviewBidders []string
	for agentName, bidType := range bids {
		if bidType == blackboard.BidTypeReview {
			reviewBidders = append(reviewBidders, agentName)
		}
	}

	if len(reviewBidders) == 0 {
		return fmt.Errorf("GrantReviewPhase called with no review bidders")
	}

	log.Printf("[Orchestrator] Granting review phase to %d agents: %v for claim %s",
		len(reviewBidders), reviewBidders, claim.ID)

	// Update claim with granted review agents
	claim.GrantedReviewAgents = reviewBidders
	claim.Status = blackboard.ClaimStatusPendingReview

	if err := e.client.UpdateClaim(ctx, claim); err != nil {
		return fmt.Errorf("failed to update claim with review grants: %w", err)
	}

	e.logEvent("review_phase_granted", map[string]interface{}{
		"claim_id":      claim.ID,
		"review_agents": reviewBidders,
		"agent_count":   len(reviewBidders),
	})

	// Publish grant notifications to all review agents
	for _, agentName := range reviewBidders {
		if err := e.publishGrantNotificationWithType(ctx, agentName, claim.ID, "review"); err != nil {
			log.Printf("[Orchestrator] Failed to publish review grant notification to %s: %v", agentName, err)
		}
		// Publish event for watching
		if err := e.publishClaimGrantedEvent(ctx, claim.ID, agentName, "review"); err != nil {
			log.Printf("[Orchestrator] Failed to publish workflow event for review grant to %s: %v", agentName, err)
		}
	}

	return nil
}

// CheckReviewPhaseCompletion checks if all review agents have submitted their reviews,
// and if so, checks for feedback and either terminates or transitions to next phase.
func (e *Engine) CheckReviewPhaseCompletion(ctx context.Context, claim *blackboard.Claim, phaseState *PhaseState) error {
	// Check if all granted review agents have submitted
	if !phaseState.IsComplete() {
		log.Printf("[Orchestrator] Review phase incomplete for claim %s: %d/%d reviews received",
			claim.ID, len(phaseState.ReceivedArtefacts), len(phaseState.GrantedAgents))
		return nil // Still waiting for reviews
	}

	log.Printf("[Orchestrator] All review artefacts received for claim %s, checking for feedback",
		claim.ID)

	// Fetch all Review artefacts and check for feedback
	// M3.3: Collect ALL feedback artefacts (not just first rejection)
	var feedbackArtefacts []*blackboard.Artefact

	for agentRole, artefactID := range phaseState.ReceivedArtefacts {
		artefact, err := e.client.GetArtefact(ctx, artefactID)
		if err != nil {
			e.logError("failed to fetch review artefact", err)
			continue
		}

		// Parse review payload
		if !isApproval(artefact.Payload) {
			feedbackArtefacts = append(feedbackArtefacts, artefact)

			e.logEvent("review_rejection", map[string]interface{}{
				"claim_id":    claim.ID,
				"reviewer":    agentRole,
				"artefact_id": artefact.ID,
			})

			// Publish review_rejected workflow event
			if err := e.publishReviewRejectedEvent(ctx, claim.ArtefactID, agentRole, artefact.Payload); err != nil {
				log.Printf("[Orchestrator] Failed to publish review_rejected event: %v", err)
			}
		} else {
			e.logEvent("review_approved", map[string]interface{}{
				"claim_id":    claim.ID,
				"reviewer":    agentRole,
				"artefact_id": artefact.ID,
			})

			// Publish review_approved workflow event
			if err := e.publishReviewApprovedEvent(ctx, claim.ArtefactID, agentRole); err != nil {
				log.Printf("[Orchestrator] Failed to publish review_approved event: %v", err)
			}
		}
	}

	if len(feedbackArtefacts) > 0 {
		// M3.3: Create feedback claim instead of just terminating
		if err := e.CreateFeedbackClaim(ctx, claim, feedbackArtefacts); err != nil {
			e.logError("failed to create feedback claim", err)
		}

		// Terminate original claim with reason
		claim.Status = blackboard.ClaimStatusTerminated
		claim.TerminationReason = formatReviewRejectionReason(feedbackArtefacts)
		if err := e.client.UpdateClaim(ctx, claim); err != nil {
			return fmt.Errorf("failed to terminate claim: %w", err)
		}

		// Delete phase state
		delete(e.phaseStates, claim.ID)

		e.logEvent("claim_terminated_review_feedback", map[string]interface{}{
			"claim_id":           claim.ID,
			"feedback_artefacts": extractIDs(feedbackArtefacts),
		})

		log.Printf("[Orchestrator] Claim %s terminated due to review feedback (%d reviewers)",
			claim.ID, len(feedbackArtefacts))

		return nil
	}

	// All approved - transition to next phase
	log.Printf("[Orchestrator] All reviews approved for claim %s, transitioning to next phase",
		claim.ID)

	return e.TransitionToNextPhase(ctx, claim, phaseState)
}

// isApproval checks if a review payload represents approval.
// Approval is indicated by an empty JSON object {} or empty JSON array [].
// Any other content (including invalid JSON) is treated as feedback.
//
// Edge cases:
//   - "{}" → approval (empty object)
//   - "[]" → approval (empty array)
//   - "{\"issue\": \"fix this\"}" → feedback (non-empty object)
//   - "[\"problem\"]" → feedback (non-empty array)
//   - "" → feedback (empty string, not JSON)
//   - "true" → feedback (JSON boolean, not object/array)
//   - "42" → feedback (JSON number)
//   - Invalid JSON → feedback
func isApproval(payload string) bool {
	// Attempt to parse as JSON
	var jsonData interface{}
	err := json.Unmarshal([]byte(payload), &jsonData)
	if err != nil {
		// Not valid JSON → feedback
		return false
	}

	// Check if empty object or empty array
	switch v := jsonData.(type) {
	case map[string]interface{}:
		return len(v) == 0 // {} = approval
	case []interface{}:
		return len(v) == 0 // [] = approval
	default:
		return false // Any other JSON type = feedback
	}
}

// publishGrantNotificationWithType publishes a grant notification with claim_type field.
func (e *Engine) publishGrantNotificationWithType(ctx context.Context, agentName, claimID, claimType string) error {
	notification := map[string]string{
		"event_type": "grant",
		"claim_id":   claimID,
		"claim_type": claimType,
	}

	notificationJSON, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal grant notification: %w", err)
	}

	channel := blackboard.AgentEventsChannel(e.instanceName, agentName)

	log.Printf("[Orchestrator] Publishing %s grant notification to %s: claim_id=%s", claimType, channel, claimID)

	if err := e.client.PublishRaw(ctx, channel, string(notificationJSON)); err != nil {
		return fmt.Errorf("failed to publish grant notification: %w", err)
	}

	e.logEvent("grant_notification_published", map[string]interface{}{
		"claim_id":   claimID,
		"agent_name": agentName,
		"claim_type": claimType,
		"channel":    channel,
	})

	return nil
}

// logError logs an error with structured context.
func (e *Engine) logError(message string, err error) {
	e.logEvent("error", map[string]interface{}{
		"message": message,
		"error":   err.Error(),
	})
}

// extractIDs extracts artefact IDs from a slice of artefacts.
// M3.3: Helper for logging feedback artefact IDs.
func extractIDs(artefacts []*blackboard.Artefact) []string {
	ids := make([]string, len(artefacts))
	for i, art := range artefacts {
		ids[i] = art.ID
	}
	return ids
}

// publishReviewApprovedEvent publishes a review_approved workflow event.
// Called when a Review artefact with empty payload (approval) is processed.
func (e *Engine) publishReviewApprovedEvent(ctx context.Context, originalArtefactID, reviewerRole string) error {
	eventData := map[string]interface{}{
		"original_artefact_id": originalArtefactID,
		"reviewer_role":        reviewerRole,
	}

	if err := e.client.PublishWorkflowEvent(ctx, "review_approved", eventData); err != nil {
		return fmt.Errorf("failed to publish review_approved event: %w", err)
	}

	log.Printf("[Orchestrator] Published review_approved event: artefact=%s, reviewer=%s",
		originalArtefactID, reviewerRole)

	return nil
}

// publishReviewRejectedEvent publishes a review_rejected workflow event.
// Called when a Review artefact with non-empty payload (feedback) is processed.
func (e *Engine) publishReviewRejectedEvent(ctx context.Context, originalArtefactID, reviewerRole, feedback string) error {
	// Truncate feedback for event payload
	feedbackSummary := feedback
	if len(feedbackSummary) > 200 {
		feedbackSummary = feedbackSummary[:200] + "..."
	}

	eventData := map[string]interface{}{
		"original_artefact_id": originalArtefactID,
		"reviewer_role":        reviewerRole,
		"feedback":             feedbackSummary,
	}

	if err := e.client.PublishWorkflowEvent(ctx, "review_rejected", eventData); err != nil {
		return fmt.Errorf("failed to publish review_rejected event: %w", err)
	}

	log.Printf("[Orchestrator] Published review_rejected event: artefact=%s, reviewer=%s",
		originalArtefactID, reviewerRole)

	return nil
}
