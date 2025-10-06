package watch

import (
	"context"
	"fmt"
	"time"

	"github.com/dyluth/sett/pkg/blackboard"
)

// PollForClaim polls for claim creation for a given artefact ID.
// Returns the created claim or an error if timeout occurs.
// Polls every 200ms for the specified timeout duration.
func PollForClaim(ctx context.Context, client *blackboard.Client, artefactID string, timeout time.Duration) (*blackboard.Claim, error) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	timeoutCh := time.After(timeout)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()

		case <-timeoutCh:
			return nil, fmt.Errorf("timeout waiting for claim after %v", timeout)

		case <-ticker.C:
			claim, err := client.GetClaimByArtefactID(ctx, artefactID)
			if err != nil {
				if blackboard.IsNotFound(err) {
					// Not found yet, continue polling
					continue
				}
				return nil, fmt.Errorf("failed to query for claim: %w", err)
			}

			// Success!
			return claim, nil
		}
	}
}
