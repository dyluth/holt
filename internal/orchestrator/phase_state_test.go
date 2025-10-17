package orchestrator

import (
	"testing"

	"github.com/dyluth/sett/pkg/blackboard"
	"github.com/stretchr/testify/assert"
)

func TestNewPhaseState(t *testing.T) {
	bids := map[string]blackboard.BidType{
		"agent1": blackboard.BidTypeReview,
		"agent2": blackboard.BidTypeExclusive,
	}

	ps := NewPhaseState("claim-123", "review", []string{"agent1"}, bids)

	assert.Equal(t, "claim-123", ps.ClaimID)
	assert.Equal(t, "review", ps.Phase)
	assert.Equal(t, []string{"agent1"}, ps.GrantedAgents)
	assert.Empty(t, ps.ReceivedArtefacts)
	assert.Equal(t, bids, ps.AllBids)
	assert.False(t, ps.StartTime.IsZero())
}

func TestPhaseState_IsComplete(t *testing.T) {
	ps := &PhaseState{
		GrantedAgents:     []string{"agent1", "agent2", "agent3"},
		ReceivedArtefacts: make(map[string]string),
	}

	// Not complete initially
	assert.False(t, ps.IsComplete())

	// Add one artefact
	ps.ReceivedArtefacts["agent1"] = "artefact-1"
	assert.False(t, ps.IsComplete())

	// Add second artefact
	ps.ReceivedArtefacts["agent2"] = "artefact-2"
	assert.False(t, ps.IsComplete())

	// Add third artefact - now complete
	ps.ReceivedArtefacts["agent3"] = "artefact-3"
	assert.True(t, ps.IsComplete())

	// Still complete with extra artefacts
	ps.ReceivedArtefacts["agent4"] = "artefact-4"
	assert.True(t, ps.IsComplete())
}

func TestHasBidsForPhase_Review(t *testing.T) {
	bids := map[string]blackboard.BidType{
		"agent1": blackboard.BidTypeReview,
		"agent2": blackboard.BidTypeExclusive,
	}

	assert.True(t, HasBidsForPhase(bids, "review"))
	assert.False(t, HasBidsForPhase(bids, "parallel"))
	assert.True(t, HasBidsForPhase(bids, "exclusive"))
}

func TestHasBidsForPhase_Parallel(t *testing.T) {
	bids := map[string]blackboard.BidType{
		"agent1": blackboard.BidTypeParallel,
		"agent2": blackboard.BidTypeParallel,
	}

	assert.False(t, HasBidsForPhase(bids, "review"))
	assert.True(t, HasBidsForPhase(bids, "parallel"))
	assert.False(t, HasBidsForPhase(bids, "exclusive"))
}

func TestHasBidsForPhase_Exclusive(t *testing.T) {
	bids := map[string]blackboard.BidType{
		"agent1": blackboard.BidTypeExclusive,
	}

	assert.False(t, HasBidsForPhase(bids, "review"))
	assert.False(t, HasBidsForPhase(bids, "parallel"))
	assert.True(t, HasBidsForPhase(bids, "exclusive"))
}

func TestHasBidsForPhase_NoBids(t *testing.T) {
	bids := map[string]blackboard.BidType{
		"agent1": blackboard.BidTypeIgnore,
		"agent2": blackboard.BidTypeIgnore,
	}

	assert.False(t, HasBidsForPhase(bids, "review"))
	assert.False(t, HasBidsForPhase(bids, "parallel"))
	assert.False(t, HasBidsForPhase(bids, "exclusive"))
}

func TestDetermineInitialPhase_ReviewFirst(t *testing.T) {
	bids := map[string]blackboard.BidType{
		"reviewer": blackboard.BidTypeReview,
		"worker":   blackboard.BidTypeParallel,
		"coder":    blackboard.BidTypeExclusive,
	}

	status, phase := DetermineInitialPhase(bids)
	assert.Equal(t, blackboard.ClaimStatusPendingReview, status)
	assert.Equal(t, "review", phase)
}

func TestDetermineInitialPhase_SkipToParallel(t *testing.T) {
	bids := map[string]blackboard.BidType{
		"worker": blackboard.BidTypeParallel,
		"coder":  blackboard.BidTypeExclusive,
	}

	status, phase := DetermineInitialPhase(bids)
	assert.Equal(t, blackboard.ClaimStatusPendingParallel, status)
	assert.Equal(t, "parallel", phase)
}

func TestDetermineInitialPhase_SkipToExclusive(t *testing.T) {
	bids := map[string]blackboard.BidType{
		"coder": blackboard.BidTypeExclusive,
	}

	status, phase := DetermineInitialPhase(bids)
	assert.Equal(t, blackboard.ClaimStatusPendingExclusive, status)
	assert.Equal(t, "exclusive", phase)
}

func TestDetermineInitialPhase_NoBids(t *testing.T) {
	bids := map[string]blackboard.BidType{
		"agent1": blackboard.BidTypeIgnore,
		"agent2": blackboard.BidTypeIgnore,
	}

	status, phase := DetermineInitialPhase(bids)
	assert.Equal(t, blackboard.ClaimStatusPendingReview, status)
	assert.Equal(t, "", phase) // Empty phase indicates dormant
}

func TestDetermineInitialPhase_EmptyBids(t *testing.T) {
	bids := map[string]blackboard.BidType{}

	status, phase := DetermineInitialPhase(bids)
	assert.Equal(t, blackboard.ClaimStatusPendingReview, status)
	assert.Equal(t, "", phase) // Empty phase indicates dormant
}
