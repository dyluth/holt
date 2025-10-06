package watch

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/dyluth/sett/pkg/blackboard"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestPollForClaim(t *testing.T) {
	// Start miniredis server
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	ctx := context.Background()

	// Create blackboard client
	redisOpts := &redis.Options{
		Addr: mr.Addr(),
	}
	client, err := blackboard.NewClient(redisOpts, "test-instance")
	require.NoError(t, err)
	defer client.Close()

	t.Run("returns claim when found immediately", func(t *testing.T) {
		// Create artefact
		artefactID := uuid.New().String()
		artefact := &blackboard.Artefact{
			ID:              artefactID,
			LogicalID:       uuid.New().String(),
			Version:         1,
			StructuralType:  blackboard.StructuralTypeStandard,
			Type:            "TestType",
			Payload:         "test",
			SourceArtefacts: []string{},
			ProducedByRole:  "user",
		}
		err := client.CreateArtefact(ctx, artefact)
		require.NoError(t, err)

		// Create claim immediately
		claimID := uuid.New().String()
		claim := &blackboard.Claim{
			ID:                    claimID,
			ArtefactID:            artefactID,
			Status:                blackboard.ClaimStatusPendingReview,
			GrantedReviewAgents:   []string{},
			GrantedParallelAgents: []string{},
			GrantedExclusiveAgent: "",
		}
		err = client.CreateClaim(ctx, claim)
		require.NoError(t, err)

		// Poll should find it immediately
		foundClaim, err := PollForClaim(ctx, client, artefactID, 2*time.Second)
		require.NoError(t, err)
		require.NotNil(t, foundClaim)
		require.Equal(t, claimID, foundClaim.ID)
		require.Equal(t, artefactID, foundClaim.ArtefactID)
	})

	t.Run("returns claim when found after delay", func(t *testing.T) {
		// Create artefact
		artefactID := uuid.New().String()
		artefact := &blackboard.Artefact{
			ID:              artefactID,
			LogicalID:       uuid.New().String(),
			Version:         1,
			StructuralType:  blackboard.StructuralTypeStandard,
			Type:            "TestType",
			Payload:         "test",
			SourceArtefacts: []string{},
			ProducedByRole:  "user",
		}
		err := client.CreateArtefact(ctx, artefact)
		require.NoError(t, err)

		// Create claim after a delay
		claimID := uuid.New().String()
		go func() {
			time.Sleep(500 * time.Millisecond)
			claim := &blackboard.Claim{
				ID:                    claimID,
				ArtefactID:            artefactID,
				Status:                blackboard.ClaimStatusPendingReview,
				GrantedReviewAgents:   []string{},
				GrantedParallelAgents: []string{},
				GrantedExclusiveAgent: "",
			}
			client.CreateClaim(context.Background(), claim)
		}()

		// Poll should find it after delay
		start := time.Now()
		foundClaim, err := PollForClaim(ctx, client, artefactID, 2*time.Second)
		elapsed := time.Since(start)

		require.NoError(t, err)
		require.NotNil(t, foundClaim)
		require.Equal(t, claimID, foundClaim.ID)
		require.GreaterOrEqual(t, elapsed, 500*time.Millisecond)
		require.Less(t, elapsed, 2*time.Second)
	})

	t.Run("returns error on timeout", func(t *testing.T) {
		// Create artefact but no claim
		artefactID := uuid.New().String()
		artefact := &blackboard.Artefact{
			ID:              artefactID,
			LogicalID:       uuid.New().String(),
			Version:         1,
			StructuralType:  blackboard.StructuralTypeStandard,
			Type:            "TestType",
			Payload:         "test",
			SourceArtefacts: []string{},
			ProducedByRole:  "user",
		}
		err := client.CreateArtefact(ctx, artefact)
		require.NoError(t, err)

		// Poll should timeout
		start := time.Now()
		_, err = PollForClaim(ctx, client, artefactID, 500*time.Millisecond)
		elapsed := time.Since(start)

		require.Error(t, err)
		require.Contains(t, err.Error(), "timeout waiting for claim")
		require.GreaterOrEqual(t, elapsed, 500*time.Millisecond)
		require.Less(t, elapsed, 1*time.Second)
	})

	t.Run("returns error when context cancelled", func(t *testing.T) {
		// Create artefact but no claim
		artefactID := uuid.New().String()
		artefact := &blackboard.Artefact{
			ID:              artefactID,
			LogicalID:       uuid.New().String(),
			Version:         1,
			StructuralType:  blackboard.StructuralTypeStandard,
			Type:            "TestType",
			Payload:         "test",
			SourceArtefacts: []string{},
			ProducedByRole:  "user",
		}
		err := client.CreateArtefact(ctx, artefact)
		require.NoError(t, err)

		// Cancel context after 100ms
		cancelCtx, cancel := context.WithCancel(ctx)
		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()

		// Poll should be cancelled
		_, err = PollForClaim(cancelCtx, client, artefactID, 2*time.Second)
		require.Error(t, err)
		require.Equal(t, context.Canceled, err)
	})

	t.Run("handles multiple polling attempts", func(t *testing.T) {
		// Create artefact
		artefactID := uuid.New().String()
		artefact := &blackboard.Artefact{
			ID:              artefactID,
			LogicalID:       uuid.New().String(),
			Version:         1,
			StructuralType:  blackboard.StructuralTypeStandard,
			Type:            "TestType",
			Payload:         "test",
			SourceArtefacts: []string{},
			ProducedByRole:  "user",
		}
		err := client.CreateArtefact(ctx, artefact)
		require.NoError(t, err)

		// Create claim after multiple poll intervals (>400ms, so at least 2 polls)
		claimID := uuid.New().String()
		go func() {
			time.Sleep(450 * time.Millisecond)
			claim := &blackboard.Claim{
				ID:                    claimID,
				ArtefactID:            artefactID,
				Status:                blackboard.ClaimStatusPendingReview,
				GrantedReviewAgents:   []string{},
				GrantedParallelAgents: []string{},
				GrantedExclusiveAgent: "",
			}
			client.CreateClaim(context.Background(), claim)
		}()

		// Poll should find it after multiple attempts
		foundClaim, err := PollForClaim(ctx, client, artefactID, 2*time.Second)
		require.NoError(t, err)
		require.NotNil(t, foundClaim)
		require.Equal(t, claimID, foundClaim.ID)
	})
}
