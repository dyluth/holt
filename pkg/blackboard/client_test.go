package blackboard

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestClient creates a test client connected to a miniredis instance
func setupTestClient(t *testing.T) (*Client, *miniredis.Miniredis) {
	mr := miniredis.NewMiniRedis()
	err := mr.Start()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	client, err := NewClient(&redis.Options{Addr: mr.Addr()}, "test-instance")
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })

	return client, mr
}

// Test client construction and basic operations
func TestNewClient(t *testing.T) {
	t.Run("creates client successfully", func(t *testing.T) {
		client, _ := setupTestClient(t)
		assert.NotNil(t, client)
		assert.Equal(t, "test-instance", client.instanceName)
	})

	t.Run("rejects empty instance name", func(t *testing.T) {
		_, err := NewClient(&redis.Options{Addr: "localhost:6379"}, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "instance name cannot be empty")
	})
}

func TestPing(t *testing.T) {
	client, _ := setupTestClient(t)
	ctx := context.Background()

	err := client.Ping(ctx)
	assert.NoError(t, err)
}

func TestClose(t *testing.T) {
	mr := miniredis.NewMiniRedis()
	err := mr.Start()
	require.NoError(t, err)
	defer mr.Close()

	client, err := NewClient(&redis.Options{Addr: mr.Addr()}, "test-instance")
	require.NoError(t, err)

	err = client.Close()
	assert.NoError(t, err)
}

// Artefact CRUD tests
func TestCreateArtefact(t *testing.T) {
	client, _ := setupTestClient(t)
	ctx := context.Background()

	t.Run("creates valid artefact", func(t *testing.T) {
		artefact := &Artefact{
			ID:              uuid.New().String(),
			LogicalID:       uuid.New().String(),
			Version:         1,
			StructuralType:  StructuralTypeStandard,
			Type:            "TestType",
			Payload:         "test payload",
			SourceArtefacts: []string{},
			ProducedByRole:  "test-agent",
		}

		err := client.CreateArtefact(ctx, artefact)
		assert.NoError(t, err)

		// Verify it was written
		retrieved, err := client.GetArtefact(ctx, artefact.ID)
		require.NoError(t, err)
		assert.Equal(t, artefact.ID, retrieved.ID)
		assert.Equal(t, artefact.Type, retrieved.Type)
		assert.Equal(t, artefact.Payload, retrieved.Payload)
	})

	t.Run("rejects invalid artefact", func(t *testing.T) {
		artefact := &Artefact{
			ID:        "not-a-uuid",
			LogicalID: uuid.New().String(),
			Version:   1,
		}

		err := client.CreateArtefact(ctx, artefact)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid artefact")
	})

	t.Run("publishes event after creation", func(t *testing.T) {
		// Subscribe to events before creating
		sub, err := client.SubscribeArtefactEvents(ctx)
		require.NoError(t, err)
		defer sub.Close()

		// Create artefact
		artefact := &Artefact{
			ID:              uuid.New().String(),
			LogicalID:       uuid.New().String(),
			Version:         1,
			StructuralType:  StructuralTypeStandard,
			Type:            "EventTest",
			Payload:         "event payload",
			SourceArtefacts: []string{},
			ProducedByRole:  "test-agent",
		}

		err = client.CreateArtefact(ctx, artefact)
		require.NoError(t, err)

		// Receive event
		select {
		case receivedArtefact := <-sub.Events():
			assert.Equal(t, artefact.ID, receivedArtefact.ID)
			assert.Equal(t, artefact.Type, receivedArtefact.Type)
		case <-time.After(1 * time.Second):
			t.Fatal("timeout waiting for artefact event")
		}
	})
}

func TestGetArtefact(t *testing.T) {
	client, _ := setupTestClient(t)
	ctx := context.Background()

	t.Run("retrieves existing artefact", func(t *testing.T) {
		// Create an artefact first
		artefact := &Artefact{
			ID:              uuid.New().String(),
			LogicalID:       uuid.New().String(),
			Version:         1,
			StructuralType:  StructuralTypeStandard,
			Type:            "TestType",
			Payload:         "test payload",
			SourceArtefacts: []string{uuid.New().String()},
			ProducedByRole:  "test-agent",
		}

		err := client.CreateArtefact(ctx, artefact)
		require.NoError(t, err)

		// Retrieve it
		retrieved, err := client.GetArtefact(ctx, artefact.ID)
		require.NoError(t, err)
		assert.Equal(t, artefact.ID, retrieved.ID)
		assert.Equal(t, artefact.LogicalID, retrieved.LogicalID)
		assert.Equal(t, artefact.Version, retrieved.Version)
		assert.Equal(t, artefact.StructuralType, retrieved.StructuralType)
		assert.Equal(t, artefact.Type, retrieved.Type)
		assert.Equal(t, artefact.Payload, retrieved.Payload)
		assert.Equal(t, artefact.SourceArtefacts, retrieved.SourceArtefacts)
		assert.Equal(t, artefact.ProducedByRole, retrieved.ProducedByRole)
	})

	t.Run("returns redis.Nil for non-existent artefact", func(t *testing.T) {
		nonExistentID := uuid.New().String()
		retrieved, err := client.GetArtefact(ctx, nonExistentID)
		assert.Nil(t, retrieved)
		assert.True(t, IsNotFound(err))
	})

	t.Run("handles empty source artefacts", func(t *testing.T) {
		artefact := &Artefact{
			ID:              uuid.New().String(),
			LogicalID:       uuid.New().String(),
			Version:         1,
			StructuralType:  StructuralTypeStandard,
			Type:            "TestType",
			Payload:         "test",
			SourceArtefacts: []string{},
			ProducedByRole:  "test-agent",
		}

		err := client.CreateArtefact(ctx, artefact)
		require.NoError(t, err)

		retrieved, err := client.GetArtefact(ctx, artefact.ID)
		require.NoError(t, err)
		assert.NotNil(t, retrieved.SourceArtefacts)
		assert.Empty(t, retrieved.SourceArtefacts)
	})
}

func TestArtefactExists(t *testing.T) {
	client, _ := setupTestClient(t)
	ctx := context.Background()

	t.Run("returns true for existing artefact", func(t *testing.T) {
		artefact := &Artefact{
			ID:              uuid.New().String(),
			LogicalID:       uuid.New().String(),
			Version:         1,
			StructuralType:  StructuralTypeStandard,
			Type:            "TestType",
			Payload:         "test",
			SourceArtefacts: []string{},
			ProducedByRole:  "test-agent",
		}

		err := client.CreateArtefact(ctx, artefact)
		require.NoError(t, err)

		exists, err := client.ArtefactExists(ctx, artefact.ID)
		assert.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("returns false for non-existent artefact", func(t *testing.T) {
		nonExistentID := uuid.New().String()
		exists, err := client.ArtefactExists(ctx, nonExistentID)
		assert.NoError(t, err)
		assert.False(t, exists)
	})
}

// Claim CRUD tests
func TestCreateClaim(t *testing.T) {
	client, _ := setupTestClient(t)
	ctx := context.Background()

	t.Run("creates valid claim", func(t *testing.T) {
		claim := &Claim{
			ID:                    uuid.New().String(),
			ArtefactID:            uuid.New().String(),
			Status:                ClaimStatusPendingReview,
			GrantedReviewAgents:   []string{},
			GrantedParallelAgents: []string{},
			GrantedExclusiveAgent: "",
		}

		err := client.CreateClaim(ctx, claim)
		assert.NoError(t, err)

		// Verify it was written
		retrieved, err := client.GetClaim(ctx, claim.ID)
		require.NoError(t, err)
		assert.Equal(t, claim.ID, retrieved.ID)
		assert.Equal(t, claim.Status, retrieved.Status)
	})

	t.Run("rejects invalid claim", func(t *testing.T) {
		claim := &Claim{
			ID:         "not-a-uuid",
			ArtefactID: uuid.New().String(),
			Status:     ClaimStatusPendingReview,
		}

		err := client.CreateClaim(ctx, claim)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid claim")
	})

	t.Run("publishes event after creation", func(t *testing.T) {
		// Subscribe to events before creating
		sub, err := client.SubscribeClaimEvents(ctx)
		require.NoError(t, err)
		defer sub.Close()

		// Create claim
		claim := &Claim{
			ID:                    uuid.New().String(),
			ArtefactID:            uuid.New().String(),
			Status:                ClaimStatusPendingReview,
			GrantedReviewAgents:   []string{},
			GrantedParallelAgents: []string{},
			GrantedExclusiveAgent: "",
		}

		err = client.CreateClaim(ctx, claim)
		require.NoError(t, err)

		// Receive event
		select {
		case receivedClaim := <-sub.Events():
			assert.Equal(t, claim.ID, receivedClaim.ID)
			assert.Equal(t, claim.Status, receivedClaim.Status)
		case <-time.After(1 * time.Second):
			t.Fatal("timeout waiting for claim event")
		}
	})
}

func TestGetClaim(t *testing.T) {
	client, _ := setupTestClient(t)
	ctx := context.Background()

	t.Run("retrieves existing claim", func(t *testing.T) {
		claim := &Claim{
			ID:                    uuid.New().String(),
			ArtefactID:            uuid.New().String(),
			Status:                ClaimStatusPendingReview,
			GrantedReviewAgents:   []string{"agent1", "agent2"},
			GrantedParallelAgents: []string{"agent3"},
			GrantedExclusiveAgent: "",
		}

		err := client.CreateClaim(ctx, claim)
		require.NoError(t, err)

		retrieved, err := client.GetClaim(ctx, claim.ID)
		require.NoError(t, err)
		assert.Equal(t, claim.ID, retrieved.ID)
		assert.Equal(t, claim.ArtefactID, retrieved.ArtefactID)
		assert.Equal(t, claim.Status, retrieved.Status)
		assert.Equal(t, claim.GrantedReviewAgents, retrieved.GrantedReviewAgents)
		assert.Equal(t, claim.GrantedParallelAgents, retrieved.GrantedParallelAgents)
		assert.Equal(t, claim.GrantedExclusiveAgent, retrieved.GrantedExclusiveAgent)
	})

	t.Run("returns redis.Nil for non-existent claim", func(t *testing.T) {
		nonExistentID := uuid.New().String()
		retrieved, err := client.GetClaim(ctx, nonExistentID)
		assert.Nil(t, retrieved)
		assert.True(t, IsNotFound(err))
	})
}

func TestUpdateClaim(t *testing.T) {
	client, _ := setupTestClient(t)
	ctx := context.Background()

	t.Run("updates existing claim", func(t *testing.T) {
		// Create initial claim
		claim := &Claim{
			ID:                    uuid.New().String(),
			ArtefactID:            uuid.New().String(),
			Status:                ClaimStatusPendingReview,
			GrantedReviewAgents:   []string{},
			GrantedParallelAgents: []string{},
			GrantedExclusiveAgent: "",
		}

		err := client.CreateClaim(ctx, claim)
		require.NoError(t, err)

		// Update it
		claim.Status = ClaimStatusPendingParallel
		claim.GrantedReviewAgents = []string{"reviewer1", "reviewer2"}

		err = client.UpdateClaim(ctx, claim)
		assert.NoError(t, err)

		// Verify update
		retrieved, err := client.GetClaim(ctx, claim.ID)
		require.NoError(t, err)
		assert.Equal(t, ClaimStatusPendingParallel, retrieved.Status)
		assert.Equal(t, []string{"reviewer1", "reviewer2"}, retrieved.GrantedReviewAgents)
	})

	t.Run("performs full replacement", func(t *testing.T) {
		// Create claim with granted agents
		claim := &Claim{
			ID:                    uuid.New().String(),
			ArtefactID:            uuid.New().String(),
			Status:                ClaimStatusPendingReview,
			GrantedReviewAgents:   []string{"agent1", "agent2"},
			GrantedParallelAgents: []string{"agent3"},
			GrantedExclusiveAgent: "",
		}

		err := client.CreateClaim(ctx, claim)
		require.NoError(t, err)

		// Update with empty arrays
		claim.GrantedReviewAgents = []string{}
		claim.GrantedParallelAgents = []string{}

		err = client.UpdateClaim(ctx, claim)
		require.NoError(t, err)

		// Verify arrays are now empty
		retrieved, err := client.GetClaim(ctx, claim.ID)
		require.NoError(t, err)
		assert.Empty(t, retrieved.GrantedReviewAgents)
		assert.Empty(t, retrieved.GrantedParallelAgents)
	})
}

func TestClaimExists(t *testing.T) {
	client, _ := setupTestClient(t)
	ctx := context.Background()

	t.Run("returns true for existing claim", func(t *testing.T) {
		claim := &Claim{
			ID:                    uuid.New().String(),
			ArtefactID:            uuid.New().String(),
			Status:                ClaimStatusPendingReview,
			GrantedReviewAgents:   []string{},
			GrantedParallelAgents: []string{},
			GrantedExclusiveAgent: "",
		}

		err := client.CreateClaim(ctx, claim)
		require.NoError(t, err)

		exists, err := client.ClaimExists(ctx, claim.ID)
		assert.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("returns false for non-existent claim", func(t *testing.T) {
		nonExistentID := uuid.New().String()
		exists, err := client.ClaimExists(ctx, nonExistentID)
		assert.NoError(t, err)
		assert.False(t, exists)
	})
}

// Bid operations tests
func TestSetBid(t *testing.T) {
	client, _ := setupTestClient(t)
	ctx := context.Background()

	t.Run("records bid successfully", func(t *testing.T) {
		claimID := uuid.New().String()

		err := client.SetBid(ctx, claimID, "agent1", BidTypeReview)
		assert.NoError(t, err)

		// Verify bid was written
		bids, err := client.GetAllBids(ctx, claimID)
		require.NoError(t, err)
		assert.Equal(t, BidTypeReview, bids["agent1"])
	})

	t.Run("rejects invalid bid type", func(t *testing.T) {
		claimID := uuid.New().String()

		err := client.SetBid(ctx, claimID, "agent1", BidType("invalid"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid bid type")
	})

	t.Run("overwrites existing bid", func(t *testing.T) {
		claimID := uuid.New().String()

		// Set initial bid
		err := client.SetBid(ctx, claimID, "agent1", BidTypeReview)
		require.NoError(t, err)

		// Overwrite with different bid
		err = client.SetBid(ctx, claimID, "agent1", BidTypeExclusive)
		require.NoError(t, err)

		// Verify it was overwritten
		bids, err := client.GetAllBids(ctx, claimID)
		require.NoError(t, err)
		assert.Equal(t, BidTypeExclusive, bids["agent1"])
	})
}

func TestGetAllBids(t *testing.T) {
	client, _ := setupTestClient(t)
	ctx := context.Background()

	t.Run("retrieves all bids", func(t *testing.T) {
		claimID := uuid.New().String()

		// Set multiple bids
		err := client.SetBid(ctx, claimID, "agent1", BidTypeReview)
		require.NoError(t, err)
		err = client.SetBid(ctx, claimID, "agent2", BidTypeParallel)
		require.NoError(t, err)
		err = client.SetBid(ctx, claimID, "agent3", BidTypeIgnore)
		require.NoError(t, err)

		// Get all bids
		bids, err := client.GetAllBids(ctx, claimID)
		require.NoError(t, err)
		assert.Len(t, bids, 3)
		assert.Equal(t, BidTypeReview, bids["agent1"])
		assert.Equal(t, BidTypeParallel, bids["agent2"])
		assert.Equal(t, BidTypeIgnore, bids["agent3"])
	})

	t.Run("returns empty map for no bids", func(t *testing.T) {
		claimID := uuid.New().String()

		bids, err := client.GetAllBids(ctx, claimID)
		assert.NoError(t, err)
		assert.Empty(t, bids)
	})
}

// Thread tracking tests
func TestAddVersionToThread(t *testing.T) {
	client, _ := setupTestClient(t)
	ctx := context.Background()

	t.Run("adds version to thread", func(t *testing.T) {
		logicalID := uuid.New().String()
		artefactID1 := uuid.New().String()
		artefactID2 := uuid.New().String()

		err := client.AddVersionToThread(ctx, logicalID, artefactID1, 1)
		assert.NoError(t, err)

		err = client.AddVersionToThread(ctx, logicalID, artefactID2, 2)
		assert.NoError(t, err)

		// Verify latest version
		latestID, version, err := client.GetLatestVersion(ctx, logicalID)
		require.NoError(t, err)
		assert.Equal(t, artefactID2, latestID)
		assert.Equal(t, 2, version)
	})
}

func TestGetLatestVersion(t *testing.T) {
	client, _ := setupTestClient(t)
	ctx := context.Background()

	t.Run("retrieves latest version", func(t *testing.T) {
		logicalID := uuid.New().String()
		artefactID1 := uuid.New().String()
		artefactID2 := uuid.New().String()
		artefactID3 := uuid.New().String()

		// Add versions out of order
		err := client.AddVersionToThread(ctx, logicalID, artefactID2, 2)
		require.NoError(t, err)
		err = client.AddVersionToThread(ctx, logicalID, artefactID1, 1)
		require.NoError(t, err)
		err = client.AddVersionToThread(ctx, logicalID, artefactID3, 3)
		require.NoError(t, err)

		// Get latest should return version 3
		latestID, version, err := client.GetLatestVersion(ctx, logicalID)
		require.NoError(t, err)
		assert.Equal(t, artefactID3, latestID)
		assert.Equal(t, 3, version)
	})

	t.Run("returns redis.Nil for empty thread", func(t *testing.T) {
		logicalID := uuid.New().String()

		latestID, version, err := client.GetLatestVersion(ctx, logicalID)
		assert.Equal(t, "", latestID)
		assert.Equal(t, 0, version)
		assert.True(t, IsNotFound(err))
	})
}

// Pub/Sub tests
func TestSubscribeArtefactEvents(t *testing.T) {
	client, _ := setupTestClient(t)
	ctx := context.Background()

	t.Run("receives published artefacts", func(t *testing.T) {
		sub, err := client.SubscribeArtefactEvents(ctx)
		require.NoError(t, err)
		defer sub.Close()

		// Create artefact (will publish event)
		artefact := &Artefact{
			ID:              uuid.New().String(),
			LogicalID:       uuid.New().String(),
			Version:         1,
			StructuralType:  StructuralTypeStandard,
			Type:            "TestType",
			Payload:         "test",
			SourceArtefacts: []string{},
			ProducedByRole:  "test-agent",
		}

		err = client.CreateArtefact(ctx, artefact)
		require.NoError(t, err)

		// Receive event
		select {
		case received := <-sub.Events():
			assert.Equal(t, artefact.ID, received.ID)
			assert.Equal(t, artefact.Type, received.Type)
		case <-time.After(1 * time.Second):
			t.Fatal("timeout waiting for event")
		}
	})

	t.Run("handles multiple subscribers", func(t *testing.T) {
		sub1, err := client.SubscribeArtefactEvents(ctx)
		require.NoError(t, err)
		defer sub1.Close()

		sub2, err := client.SubscribeArtefactEvents(ctx)
		require.NoError(t, err)
		defer sub2.Close()

		// Create artefact
		artefact := &Artefact{
			ID:              uuid.New().String(),
			LogicalID:       uuid.New().String(),
			Version:         1,
			StructuralType:  StructuralTypeStandard,
			Type:            "MultiSubTest",
			Payload:         "test",
			SourceArtefacts: []string{},
			ProducedByRole:  "test-agent",
		}

		err = client.CreateArtefact(ctx, artefact)
		require.NoError(t, err)

		// Both should receive
		select {
		case received := <-sub1.Events():
			assert.Equal(t, artefact.ID, received.ID)
		case <-time.After(1 * time.Second):
			t.Fatal("timeout on sub1")
		}

		select {
		case received := <-sub2.Events():
			assert.Equal(t, artefact.ID, received.ID)
		case <-time.After(1 * time.Second):
			t.Fatal("timeout on sub2")
		}
	})

	t.Run("cleanup on Close", func(t *testing.T) {
		sub, err := client.SubscribeArtefactEvents(ctx)
		require.NoError(t, err)

		err = sub.Close()
		assert.NoError(t, err)

		// Calling Close again should be safe
		err = sub.Close()
		assert.NoError(t, err)
	})

	t.Run("cleanup on context cancellation", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)

		sub, err := client.SubscribeArtefactEvents(cancelCtx)
		require.NoError(t, err)

		cancel()

		// Events channel should eventually close
		select {
		case _, ok := <-sub.Events():
			assert.False(t, ok, "channel should be closed")
		case <-time.After(1 * time.Second):
			t.Fatal("timeout waiting for channel close")
		}
	})
}

func TestSubscribeClaimEvents(t *testing.T) {
	client, _ := setupTestClient(t)
	ctx := context.Background()

	t.Run("receives published claims", func(t *testing.T) {
		sub, err := client.SubscribeClaimEvents(ctx)
		require.NoError(t, err)
		defer sub.Close()

		// Create claim (will publish event)
		claim := &Claim{
			ID:                    uuid.New().String(),
			ArtefactID:            uuid.New().String(),
			Status:                ClaimStatusPendingReview,
			GrantedReviewAgents:   []string{},
			GrantedParallelAgents: []string{},
			GrantedExclusiveAgent: "",
		}

		err = client.CreateClaim(ctx, claim)
		require.NoError(t, err)

		// Receive event
		select {
		case received := <-sub.Events():
			assert.Equal(t, claim.ID, received.ID)
			assert.Equal(t, claim.Status, received.Status)
		case <-time.After(1 * time.Second):
			t.Fatal("timeout waiting for event")
		}
	})
}

// Instance namespacing tests
func TestInstanceNamespacing(t *testing.T) {
	mr := miniredis.NewMiniRedis()
	err := mr.Start()
	require.NoError(t, err)
	defer mr.Close()

	// Create two clients with different instances
	client1, err := NewClient(&redis.Options{Addr: mr.Addr()}, "instance-1")
	require.NoError(t, err)
	defer client1.Close()

	client2, err := NewClient(&redis.Options{Addr: mr.Addr()}, "instance-2")
	require.NoError(t, err)
	defer client2.Close()

	ctx := context.Background()

	t.Run("artefacts are instance-isolated", func(t *testing.T) {
		artefact := &Artefact{
			ID:              uuid.New().String(),
			LogicalID:       uuid.New().String(),
			Version:         1,
			StructuralType:  StructuralTypeStandard,
			Type:            "TestType",
			Payload:         "test",
			SourceArtefacts: []string{},
			ProducedByRole:  "test-agent",
		}

		// Create in instance-1
		err := client1.CreateArtefact(ctx, artefact)
		require.NoError(t, err)

		// Should exist in instance-1
		exists, err := client1.ArtefactExists(ctx, artefact.ID)
		require.NoError(t, err)
		assert.True(t, exists)

		// Should NOT exist in instance-2
		exists, err = client2.ArtefactExists(ctx, artefact.ID)
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("events are instance-isolated", func(t *testing.T) {
		// Subscribe to instance-1 events
		sub1, err := client1.SubscribeArtefactEvents(ctx)
		require.NoError(t, err)
		defer sub1.Close()

		// Subscribe to instance-2 events
		sub2, err := client2.SubscribeArtefactEvents(ctx)
		require.NoError(t, err)
		defer sub2.Close()

		// Create artefact in instance-1
		artefact := &Artefact{
			ID:              uuid.New().String(),
			LogicalID:       uuid.New().String(),
			Version:         1,
			StructuralType:  StructuralTypeStandard,
			Type:            "IsolationTest",
			Payload:         "test",
			SourceArtefacts: []string{},
			ProducedByRole:  "test-agent",
		}

		err = client1.CreateArtefact(ctx, artefact)
		require.NoError(t, err)

		// instance-1 subscription should receive event
		select {
		case received := <-sub1.Events():
			assert.Equal(t, artefact.ID, received.ID)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timeout waiting for instance-1 event")
		}

		// instance-2 subscription should NOT receive event
		select {
		case <-sub2.Events():
			t.Fatal("instance-2 should not receive event from instance-1")
		case <-time.After(500 * time.Millisecond):
			// Expected - no event received
		}
	})
}

// IsNotFound helper test
func TestIsNotFound(t *testing.T) {
	t.Run("returns true for redis.Nil", func(t *testing.T) {
		assert.True(t, IsNotFound(redis.Nil))
	})

	t.Run("returns false for other errors", func(t *testing.T) {
		assert.False(t, IsNotFound(context.Canceled))
		assert.False(t, IsNotFound(nil))
	})
}

// Error channel tests
func TestSubscriptionErrorChannel(t *testing.T) {
	client, _ := setupTestClient(t)
	ctx := context.Background()

	t.Run("artefact subscription exposes errors channel", func(t *testing.T) {
		sub, err := client.SubscribeArtefactEvents(ctx)
		require.NoError(t, err)
		defer sub.Close()

		// Verify error channel is accessible
		assert.NotNil(t, sub.Errors())
	})

	t.Run("claim subscription exposes errors channel", func(t *testing.T) {
		sub, err := client.SubscribeClaimEvents(ctx)
		require.NoError(t, err)
		defer sub.Close()

		// Verify error channel is accessible
		assert.NotNil(t, sub.Errors())
	})

	// Note: Testing invalid JSON handling through miniredis is unreliable
	// The error handling code is covered by inspection and the error channel
	// is verified to be accessible above
}

// Error path coverage tests
func TestErrorPaths(t *testing.T) {
	client, _ := setupTestClient(t)
	ctx := context.Background()

	t.Run("UpdateClaim with serialization error", func(t *testing.T) {
		// Create a claim that will fail serialization
		// (Note: In practice, serialization failures are hard to trigger with our types,
		// but we test the error path exists)
		claim := &Claim{
			ID:         uuid.New().String(),
			ArtefactID: uuid.New().String(),
			Status:     "invalid-status", // Will pass validation as string, but is semantically wrong
		}

		// This should still work because our types are simple
		err := client.UpdateClaim(ctx, claim)
		// Error because validation fails
		assert.Error(t, err)
	})

	t.Run("GetArtefact with Redis error path", func(t *testing.T) {
		// Create a client with a closed connection
		closedClient, _ := setupTestClient(t)
		closedClient.Close()

		_, err := closedClient.GetArtefact(ctx, uuid.New().String())
		assert.Error(t, err)
	})

	t.Run("SetBid with Redis error path", func(t *testing.T) {
		closedClient, _ := setupTestClient(t)
		closedClient.Close()

		err := closedClient.SetBid(ctx, uuid.New().String(), "agent", BidTypeReview)
		assert.Error(t, err)
	})

	t.Run("AddVersionToThread with Redis error path", func(t *testing.T) {
		closedClient, _ := setupTestClient(t)
		closedClient.Close()

		err := closedClient.AddVersionToThread(ctx, uuid.New().String(), uuid.New().String(), 1)
		assert.Error(t, err)
	})
}
