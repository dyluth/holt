package orchestrator

import (
	"context"
	"fmt"
	"log"

	"github.com/dyluth/sett/pkg/blackboard"
	"github.com/google/uuid"
)

// CreateFeedbackClaim creates a new claim assigned to the original producer of the rejected artefact.
// This bypasses bidding and provides the agent with feedback context for rework.
//
// M3.3: Called when review phase detects rejection feedback.
func (e *Engine) CreateFeedbackClaim(ctx context.Context, originalClaim *blackboard.Claim, feedbackArtefacts []*blackboard.Artefact) error {
	// Fetch the original artefact that was reviewed
	targetArtefact, err := e.client.GetArtefact(ctx, originalClaim.ArtefactID)
	if err != nil {
		return fmt.Errorf("failed to fetch target artefact: %w", err)
	}

	// Check iteration limit using version number
	iterationCount := targetArtefact.Version - 1
	maxIterations := e.config.Orchestrator.MaxReviewIterations

	if maxIterations > 0 && iterationCount >= maxIterations {
		// Create Failure artefact and terminate
		return e.terminateMaxIterations(ctx, originalClaim, targetArtefact, iterationCount)
	}

	// Find the agent that produced the original artefact (reverse-lookup by role)
	producerAgent, err := e.findAgentByRole(targetArtefact.ProducedByRole)
	if err != nil {
		// Agent no longer exists in config
		return e.terminateMissingAgent(ctx, originalClaim, targetArtefact)
	}

	// Extract Review artefact IDs for additional context
	reviewIDs := make([]string, len(feedbackArtefacts))
	for i, art := range feedbackArtefacts {
		reviewIDs[i] = art.ID
	}

	// Create feedback claim
	feedbackClaim := &blackboard.Claim{
		ID:                   uuid.New().String(),
		ArtefactID:           targetArtefact.ID, // Target is the original work, not the Review
		Status:               blackboard.ClaimStatusPendingAssignment,
		GrantedExclusiveAgent: producerAgent,
		AdditionalContextIDs: reviewIDs, // Inject Review artefacts into context
		GrantedReviewAgents:  []string{},
		GrantedParallelAgents: []string{},
	}

	if err := e.client.CreateClaim(ctx, feedbackClaim); err != nil {
		return fmt.Errorf("failed to create feedback claim: %w", err)
	}

	e.logEvent("feedback_claim_created", map[string]interface{}{
		"feedback_claim_id": feedbackClaim.ID,
		"original_claim_id": originalClaim.ID,
		"target_artefact":   targetArtefact.ID,
		"assigned_agent":    producerAgent,
		"review_artefacts":  reviewIDs,
		"iteration":         iterationCount + 1,
	})

	log.Printf("[Orchestrator] Created feedback claim %s for agent %s (iteration %d)",
		feedbackClaim.ID, producerAgent, iterationCount+1)

	// Track feedback claim for completion checking
	e.pendingAssignmentClaims[feedbackClaim.ID] = targetArtefact.ID

	// Note: CreateClaim already published to claim_events channel

	return nil
}

// findAgentByRole performs reverse-lookup in agent registry to find agent name by role.
// Returns error if no agent has the specified role.
func (e *Engine) findAgentByRole(role string) (string, error) {
	for agentName, agentRole := range e.agentRegistry {
		if agentRole == role {
			return agentName, nil
		}
	}
	return "", fmt.Errorf("no agent found with role '%s'", role)
}

// terminateMaxIterations creates Failure artefact when iteration limit is reached.
// M3.3: Called when artefact.version - 1 >= max_review_iterations.
func (e *Engine) terminateMaxIterations(ctx context.Context, claim *blackboard.Claim, artefact *blackboard.Artefact, iterations int) error {
	maxIterations := e.config.Orchestrator.MaxReviewIterations
	failurePayload := fmt.Sprintf("Max review iterations (%d) reached for artefact %s (version %d). Review feedback loop terminated.",
		maxIterations, artefact.ID, artefact.Version)

	failure := &blackboard.Artefact{
		ID:              uuid.New().String(),
		LogicalID:       uuid.New().String(),
		Version:         1,
		StructuralType:  blackboard.StructuralTypeFailure,
		Type:            "MaxIterationsExceeded",
		Payload:         failurePayload,
		SourceArtefacts: []string{artefact.ID},
		ProducedByRole:  "orchestrator",
	}

	if err := e.client.CreateArtefact(ctx, failure); err != nil {
		return fmt.Errorf("failed to create Failure artefact: %w", err)
	}

	claim.Status = blackboard.ClaimStatusTerminated
	claim.TerminationReason = fmt.Sprintf("Terminated after reaching max review iterations (%d).", iterations+1)

	e.logEvent("claim_terminated_max_iterations", map[string]interface{}{
		"claim_id":   claim.ID,
		"artefact_id": artefact.ID,
		"iterations": iterations + 1,
		"failure_id": failure.ID,
	})

	log.Printf("[Orchestrator] Claim %s terminated: max iterations (%d) reached",
		claim.ID, maxIterations)

	return e.client.UpdateClaim(ctx, claim)
}

// terminateMissingAgent creates Failure artefact when original agent no longer exists.
// M3.3: Called when findAgentByRole fails during feedback claim creation.
func (e *Engine) terminateMissingAgent(ctx context.Context, claim *blackboard.Claim, artefact *blackboard.Artefact) error {
	failurePayload := fmt.Sprintf("Cannot create feedback claim: agent with role '%s' no longer exists in configuration.",
		artefact.ProducedByRole)

	failure := &blackboard.Artefact{
		ID:              uuid.New().String(),
		LogicalID:       uuid.New().String(),
		Version:         1,
		StructuralType:  blackboard.StructuralTypeFailure,
		Type:            "MissingAgentConfiguration",
		Payload:         failurePayload,
		SourceArtefacts: []string{artefact.ID},
		ProducedByRole:  "orchestrator",
	}

	if err := e.client.CreateArtefact(ctx, failure); err != nil {
		return fmt.Errorf("failed to create Failure artefact: %w", err)
	}

	claim.Status = blackboard.ClaimStatusTerminated
	claim.TerminationReason = fmt.Sprintf("Terminated due to missing agent configuration (role: %s).", artefact.ProducedByRole)

	e.logEvent("claim_terminated_missing_agent", map[string]interface{}{
		"claim_id":     claim.ID,
		"missing_role": artefact.ProducedByRole,
		"failure_id":   failure.ID,
	})

	log.Printf("[Orchestrator] Claim %s terminated: agent with role '%s' not found",
		claim.ID, artefact.ProducedByRole)

	return e.client.UpdateClaim(ctx, claim)
}

// formatReviewRejectionReason creates human-readable termination reason for review feedback.
// M3.3: Used when setting claim.TerminationReason after review rejection.
func formatReviewRejectionReason(feedbackArtefacts []*blackboard.Artefact) string {
	ids := make([]string, len(feedbackArtefacts))
	for i, art := range feedbackArtefacts {
		ids[i] = art.ID
	}
	return fmt.Sprintf("Terminated due to negative review feedback. See artefacts: %v", ids)
}
