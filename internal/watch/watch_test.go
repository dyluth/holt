package watch

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/dyluth/holt/pkg/blackboard"
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
			ProducedByRole:  "test-agent",
			Payload:         "test",
			SourceArtefacts: []string{},
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
			ProducedByRole:  "test-agent",
			Payload:         "test",
			SourceArtefacts: []string{},
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
			ProducedByRole:  "test-agent",
			Payload:         "test",
			SourceArtefacts: []string{},
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
			ProducedByRole:  "test-agent",
			Payload:         "test",
			SourceArtefacts: []string{},
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
			ProducedByRole:  "test-agent",
			Payload:         "test",
			SourceArtefacts: []string{},
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

// M2.6 formatter tests
func TestFormatters(t *testing.T) {
	t.Run("defaultFormatter formats artefact events", func(t *testing.T) {
		var buf []byte
		writer := &testWriter{buf: &buf}
		formatter := &defaultFormatter{writer: writer}

		artefact := &blackboard.Artefact{
			ID:   "abc-123",
			Type: "GoalDefined",
			ProducedByRole:  "test-agent",
		}

		err := formatter.FormatArtefact(artefact)
		require.NoError(t, err)

		output := string(buf)
		require.Contains(t, output, "✨ Artefact created")
		require.Contains(t, output, "type=GoalDefined")
		require.Contains(t, output, "id=abc-123")
	})

	t.Run("defaultFormatter formats Terminal artefact with completion message", func(t *testing.T) {
		var buf []byte
		writer := &testWriter{buf: &buf}
		formatter := &defaultFormatter{writer: writer}

		artefact := &blackboard.Artefact{
			ID:             "terminal-123",
			Type:           "PackagedModule",
			StructuralType: blackboard.StructuralTypeTerminal,
			ProducedByRole: "ModulePackager",
		}

		err := formatter.FormatArtefact(artefact)
		require.NoError(t, err)

		output := string(buf)
		require.Contains(t, output, "✨ Artefact created")
		require.Contains(t, output, "type=PackagedModule")
		require.Contains(t, output, "id=terminal-123")
		require.Contains(t, output, "🎉 Workflow completed")
		require.Contains(t, output, "Terminal artefact created")
		require.Contains(t, output, "id=terminal-123")
	})

	t.Run("defaultFormatter formats claim events", func(t *testing.T) {
		var buf []byte
		writer := &testWriter{buf: &buf}
		formatter := &defaultFormatter{writer: writer}

		claim := &blackboard.Claim{
			ID:         "claim-123",
			ArtefactID: "artefact-456",
			Status:     blackboard.ClaimStatusPendingReview,
		}

		err := formatter.FormatClaim(claim)
		require.NoError(t, err)

		output := string(buf)
		require.Contains(t, output, "⏳ Claim created")
		require.Contains(t, output, "claim=claim-123")
		require.Contains(t, output, "artefact=artefact-456")
		require.Contains(t, output, "status=pending_review")
	})

	t.Run("defaultFormatter formats bid_submitted events", func(t *testing.T) {
		var buf []byte
		writer := &testWriter{buf: &buf}
		formatter := &defaultFormatter{writer: writer}

		event := &blackboard.WorkflowEvent{
			Event: "bid_submitted",
			Data: map[string]interface{}{
				"claim_id":   "claim-123",
				"agent_name": "test-agent",
				"bid_type":   "exclusive",
			},
		}

		err := formatter.FormatWorkflow(event)
		require.NoError(t, err)

		output := string(buf)
		require.Contains(t, output, "🙋 Bid submitted")
		require.Contains(t, output, "agent=test-agent")
		require.Contains(t, output, "claim=claim-123")
		require.Contains(t, output, "type=exclusive")
	})

	t.Run("defaultFormatter formats claim_granted events", func(t *testing.T) {
		var buf []byte
		writer := &testWriter{buf: &buf}
		formatter := &defaultFormatter{writer: writer}

		event := &blackboard.WorkflowEvent{
			Event: "claim_granted",
			Data: map[string]interface{}{
				"claim_id":   "claim-123",
				"agent_name": "test-agent",
				"grant_type": "exclusive",
			},
		}

		err := formatter.FormatWorkflow(event)
		require.NoError(t, err)

		output := string(buf)
		require.Contains(t, output, "🏆 Claim granted")
		require.Contains(t, output, "agent=test-agent")
		require.Contains(t, output, "claim=claim-123")
		require.Contains(t, output, "type=exclusive")
	})

	t.Run("jsonFormatter formats artefact events", func(t *testing.T) {
		var buf []byte
		writer := &testWriter{buf: &buf}
		formatter := &jsonFormatter{writer: writer}

		artefact := &blackboard.Artefact{
			ID:   "abc-123",
			Type: "GoalDefined",
			ProducedByRole:  "test-agent",
		}

		err := formatter.FormatArtefact(artefact)
		require.NoError(t, err)

		output := string(buf)
		require.Contains(t, output, `"event":"artefact_created"`)
		require.Contains(t, output, `"id":"abc-123"`)
		require.Contains(t, output, `"type":"GoalDefined"`)
	})

	t.Run("jsonFormatter formats claim events", func(t *testing.T) {
		var buf []byte
		writer := &testWriter{buf: &buf}
		formatter := &jsonFormatter{writer: writer}

		claim := &blackboard.Claim{
			ID:         "claim-123",
			ArtefactID: "artefact-456",
			Status:     blackboard.ClaimStatusPendingReview,
		}

		err := formatter.FormatClaim(claim)
		require.NoError(t, err)

		output := string(buf)
		require.Contains(t, output, `"event":"claim_created"`)
		require.Contains(t, output, `"id":"claim-123"`)
		require.Contains(t, output, `"artefact_id":"artefact-456"`)
	})

	t.Run("jsonFormatter formats workflow events", func(t *testing.T) {
		var buf []byte
		writer := &testWriter{buf: &buf}
		formatter := &jsonFormatter{writer: writer}

		event := &blackboard.WorkflowEvent{
			Event: "bid_submitted",
			Data: map[string]interface{}{
				"claim_id":   "claim-123",
				"agent_name": "test-agent",
			},
		}

		err := formatter.FormatWorkflow(event)
		require.NoError(t, err)

		output := string(buf)
		require.Contains(t, output, `"event":"bid_submitted"`)
		require.Contains(t, output, `"claim_id":"claim-123"`)
		require.Contains(t, output, `"agent_name":"test-agent"`)
	})
}

// testWriter is a simple writer for testing formatters
type testWriter struct {
	buf *[]byte
}

func (w *testWriter) Write(p []byte) (n int, err error) {
	*w.buf = append(*w.buf, p...)
	return len(p), nil
}
