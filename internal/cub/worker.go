package cub

import (
	"context"
	"fmt"
	"log"

	"github.com/dyluth/sett/pkg/blackboard"
)

// RunWorkerMode executes a specific claim and exits.
// M3.4: Workers are ephemeral containers launched by the orchestrator when a controller wins a grant.
// They perform the actual work and exit immediately after completion.
func RunWorkerMode(ctx context.Context, config *Config, bbClient *blackboard.Client, claimID string) error {
	log.Printf("[Worker] Executing claim %s", claimID)

	// Fetch the claim
	claim, err := bbClient.GetClaim(ctx, claimID)
	if err != nil {
		return fmt.Errorf("failed to fetch claim: %w", err)
	}

	// Verify claim is granted (can be pending_exclusive or pending_assignment)
	if claim.Status != blackboard.ClaimStatusPendingExclusive && claim.Status != blackboard.ClaimStatusPendingAssignment {
		return fmt.Errorf("claim %s is not in valid status for worker execution (status: %s)", claimID, claim.Status)
	}

	log.Printf("[Worker] Claim status: %s", claim.Status)

	// Create an engine to reuse existing execution logic
	engine := New(config, bbClient)

	// Execute the work using the existing executeWork method
	// This handles everything: fetching artefacts, assembling context, executing tool, creating result
	engine.executeWork(ctx, claim)

	log.Printf("[Worker] Work execution complete, exiting")

	// Exit cleanly - orchestrator will detect completion
	return nil
}
