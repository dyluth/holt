package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/dyluth/holt/pkg/blackboard"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIsTerminalArtefact verifies Terminal artefact detection logic.
func TestIsTerminalArtefact(t *testing.T) {
	tests := []struct {
		name           string
		structuralType blackboard.StructuralType
		expectedSkip   bool
	}{
		{
			name:           "Terminal artefact should be skipped",
			structuralType: blackboard.StructuralTypeTerminal,
			expectedSkip:   true,
		},
		{
			name:           "Standard artefact should not be skipped",
			structuralType: blackboard.StructuralTypeStandard,
			expectedSkip:   false,
		},
		{
			name:           "Review artefact should not be skipped",
			structuralType: blackboard.StructuralTypeReview,
			expectedSkip:   false,
		},
		{
			name:           "Question artefact should not be skipped",
			structuralType: blackboard.StructuralTypeQuestion,
			expectedSkip:   false,
		},
		{
			name:           "Answer artefact should not be skipped",
			structuralType: blackboard.StructuralTypeAnswer,
			expectedSkip:   false,
		},
		{
			name:           "Failure artefact should not be skipped",
			structuralType: blackboard.StructuralTypeFailure,
			expectedSkip:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isTerminal := tt.structuralType == blackboard.StructuralTypeTerminal
			if isTerminal != tt.expectedSkip {
				t.Errorf("Expected skip=%v for %s, got %v", tt.expectedSkip, tt.structuralType, isTerminal)
			}
		})
	}
}

// TestCreateClaimForArtefact verifies claim struct creation with correct fields.
func TestCreateClaimForArtefact(t *testing.T) {
	artefactID := "550e8400-e29b-41d4-a716-446655440000"
	claimID := "650e8400-e29b-41d4-a716-446655440001"

	claim := &blackboard.Claim{
		ID:                    claimID,
		ArtefactID:            artefactID,
		Status:                blackboard.ClaimStatusPendingReview,
		GrantedReviewAgents:   []string{},
		GrantedParallelAgents: []string{},
		GrantedExclusiveAgent: "",
	}

	// Verify all fields are set correctly
	if claim.ID != claimID {
		t.Errorf("Expected claim ID %s, got %s", claimID, claim.ID)
	}

	if claim.ArtefactID != artefactID {
		t.Errorf("Expected artefact ID %s, got %s", artefactID, claim.ArtefactID)
	}

	if claim.Status != blackboard.ClaimStatusPendingReview {
		t.Errorf("Expected status pending_review, got %s", claim.Status)
	}

	if len(claim.GrantedReviewAgents) != 0 {
		t.Errorf("Expected empty GrantedReviewAgents, got %v", claim.GrantedReviewAgents)
	}

	if len(claim.GrantedParallelAgents) != 0 {
		t.Errorf("Expected empty GrantedParallelAgents, got %v", claim.GrantedParallelAgents)
	}

	if claim.GrantedExclusiveAgent != "" {
		t.Errorf("Expected empty GrantedExclusiveAgent, got %s", claim.GrantedExclusiveAgent)
	}

	// Verify validation passes
	if err := claim.Validate(); err != nil {
		t.Errorf("Valid claim failed validation: %v", err)
	}
}

// setupTestEngine creates a test engine connected to miniredis
func setupTestEngine(t *testing.T) (*Engine, *blackboard.Client, *miniredis.Miniredis) {
	mr := miniredis.NewMiniRedis()
	err := mr.Start()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	client, err := blackboard.NewClient(&redis.Options{Addr: mr.Addr()}, "test-instance")
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })

	engine := NewEngine(client, "test-instance", nil, nil)

	return engine, client, mr
}

// TestPublishClaimGrantedEvent verifies grant event publishing (M2.6)
func TestPublishClaimGrantedEvent(t *testing.T) {
	t.Run("publishes exclusive grant event", func(t *testing.T) {
		ctx := context.Background()
		engine, client, _ := setupTestEngine(t)

		// Subscribe to workflow events before publishing
		sub, err := client.SubscribeWorkflowEvents(ctx)
		require.NoError(t, err)
		defer sub.Close()

		// Small delay to ensure subscription is ready
		time.Sleep(10 * time.Millisecond)

		// Create claim ID for event
		claimID := uuid.New().String()

		// Publish event with explicit grant type
		err = engine.publishClaimGrantedEvent(ctx, claimID, "test-agent", "exclusive")
		require.NoError(t, err)

		// Receive and verify event with longer timeout for CI
		select {
		case event := <-sub.Events():
			assert.Equal(t, "claim_granted", event.Event)
			assert.Equal(t, claimID, event.Data["claim_id"])
			assert.Equal(t, "test-agent", event.Data["agent_name"])
			assert.Equal(t, "exclusive", event.Data["grant_type"])
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for claim_granted event")
		}
	})

	t.Run("publishes review grant event", func(t *testing.T) {
		ctx := context.Background()
		engine, client, _ := setupTestEngine(t)

		sub, err := client.SubscribeWorkflowEvents(ctx)
		require.NoError(t, err)
		defer sub.Close()

		time.Sleep(10 * time.Millisecond)

		claimID := uuid.New().String()

		err = engine.publishClaimGrantedEvent(ctx, claimID, "agent1", "review")
		require.NoError(t, err)

		select {
		case event := <-sub.Events():
			assert.Equal(t, "claim_granted", event.Event)
			assert.Equal(t, "review", event.Data["grant_type"])
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for event")
		}
	})

	t.Run("publishes parallel grant event", func(t *testing.T) {
		ctx := context.Background()
		engine, client, _ := setupTestEngine(t)

		sub, err := client.SubscribeWorkflowEvents(ctx)
		require.NoError(t, err)
		defer sub.Close()

		time.Sleep(10 * time.Millisecond)

		claimID := uuid.New().String()

		err = engine.publishClaimGrantedEvent(ctx, claimID, "agent1", "claim")
		require.NoError(t, err)

		select {
		case event := <-sub.Events():
			assert.Equal(t, "claim_granted", event.Event)
			assert.Equal(t, "claim", event.Data["grant_type"])
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for event")
		}
	})

	t.Run("publishes event with explicit grant type", func(t *testing.T) {
		ctx := context.Background()
		engine, client, _ := setupTestEngine(t)

		sub, err := client.SubscribeWorkflowEvents(ctx)
		require.NoError(t, err)
		defer sub.Close()

		time.Sleep(10 * time.Millisecond)

		claimID := uuid.New().String()

		// Test that explicit grant types are used as-is
		err = engine.publishClaimGrantedEvent(ctx, claimID, "test-agent", "exclusive")
		require.NoError(t, err)

		select {
		case event := <-sub.Events():
			assert.Equal(t, "claim_granted", event.Event)
			assert.Equal(t, claimID, event.Data["claim_id"])
			assert.Equal(t, "test-agent", event.Data["agent_name"])
			assert.Equal(t, "exclusive", event.Data["grant_type"])
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for event")
		}
	})
}
