package orchestrator

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/dyluth/holt/internal/config"
	"github.com/dyluth/holt/pkg/blackboard"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestEngineWithMaxIterations creates an engine with test configuration for unit tests
func setupTestEngineWithMaxIterations(t *testing.T, maxIterations int) (*Engine, *blackboard.Client) {
	// Use miniredis for embedded Redis
	mr := miniredis.NewMiniRedis()
	err := mr.Start()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	instanceName := "test-" + uuid.New().String()[:8]
	bbClient, err := blackboard.NewClient(&redis.Options{Addr: mr.Addr()}, instanceName)
	require.NoError(t, err)
	t.Cleanup(func() { bbClient.Close() })

	cfg := &config.HoltConfig{
		Version: "1.0",
		Orchestrator: &config.OrchestratorConfig{
			MaxReviewIterations: &maxIterations,
		},
		Agents: map[string]config.Agent{
			"coder-agent": {
				Role:            "Coder",
				Image:           "test:latest",
				Command:         []string{"test"},
				BiddingStrategy: "exclusive",
			},
			"reviewer-agent": {
				Role:            "Reviewer",
				Image:           "test:latest",
				Command:         []string{"test"},
				BiddingStrategy: "review",
			},
		},
	}

	engine := NewEngine(bbClient, instanceName, cfg, nil)

	return engine, bbClient
}

func TestFindAgentByRole(t *testing.T) {
	engine, _ := setupTestEngineWithMaxIterations(t, 3)

	tests := []struct {
		name        string
		role        string
		expectAgent string
		expectError bool
	}{
		{
			name:        "finds coder agent",
			role:        "Coder",
			expectAgent: "coder-agent",
			expectError: false,
		},
		{
			name:        "finds reviewer agent",
			role:        "Reviewer",
			expectAgent: "reviewer-agent",
			expectError: false,
		},
		{
			name:        "missing role returns error",
			role:        "NonExistent",
			expectAgent: "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent, err := engine.findAgentByRole(tt.role)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "no agent found")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectAgent, agent)
			}
		})
	}
}

func TestFormatReviewRejectionReason(t *testing.T) {
	artefacts := []*blackboard.Artefact{
		{ID: "review-1"},
		{ID: "review-2"},
		{ID: "review-3"},
	}

	reason := formatReviewRejectionReason(artefacts)

	assert.Contains(t, reason, "Terminated due to negative review feedback")
	assert.Contains(t, reason, "review-1")
	assert.Contains(t, reason, "review-2")
	assert.Contains(t, reason, "review-3")
}

func TestCreateFeedbackClaim_IterationLimitCheck(t *testing.T) {
	ctx := context.Background()
	maxIterations := 2
	engine, bbClient := setupTestEngineWithMaxIterations(t, maxIterations)

	// Create target artefact with version 3 (iteration count = 2)
	targetArtefact := &blackboard.Artefact{
		ID:              uuid.New().String(),
		LogicalID:       uuid.New().String(),
		Version:         3, // iteration = version - 1 = 2
		StructuralType:  blackboard.StructuralTypeStandard,
		Type:            "CodeCommit",
		Payload:         "test-commit-hash",
		SourceArtefacts: []string{},
		ProducedByRole:  "Coder",
	}
	err := bbClient.CreateArtefact(ctx, targetArtefact)
	require.NoError(t, err)

	// Create original claim
	originalClaim := &blackboard.Claim{
		ID:                    uuid.New().String(),
		ArtefactID:            targetArtefact.ID,
		Status:                blackboard.ClaimStatusPendingReview,
		GrantedReviewAgents:   []string{"reviewer-agent"},
		GrantedParallelAgents: []string{},
		GrantedExclusiveAgent: "",
	}
	err = bbClient.CreateClaim(ctx, originalClaim)
	require.NoError(t, err)

	// Create review feedback artefacts
	reviewArtefact := &blackboard.Artefact{
		ID:              uuid.New().String(),
		LogicalID:       uuid.New().String(),
		Version:         1,
		StructuralType:  blackboard.StructuralTypeReview,
		Type:            "Review",
		Payload:         `{"issue": "needs tests"}`,
		SourceArtefacts: []string{targetArtefact.ID},
		ProducedByRole:  "Reviewer",
	}
	err = bbClient.CreateArtefact(ctx, reviewArtefact)
	require.NoError(t, err)

	feedbackArtefacts := []*blackboard.Artefact{reviewArtefact}

	// Call CreateFeedbackClaim - should hit max iterations
	err = engine.CreateFeedbackClaim(ctx, originalClaim, feedbackArtefacts)
	require.NoError(t, err)

	// Verify claim was terminated (not a feedback claim created)
	updatedClaim, err := bbClient.GetClaim(ctx, originalClaim.ID)
	require.NoError(t, err)
	assert.Equal(t, blackboard.ClaimStatusTerminated, updatedClaim.Status)
	assert.Contains(t, updatedClaim.TerminationReason, "max review iterations")
	assert.Contains(t, updatedClaim.TerminationReason, "2")

	// Verify Failure artefact was created
	// (We'd need to query for it - simplified for this test)
}

func TestCreateFeedbackClaim_Success(t *testing.T) {
	ctx := context.Background()
	maxIterations := 3
	engine, bbClient := setupTestEngineWithMaxIterations(t, maxIterations)

	// Create target artefact with version 1 (iteration count = 0)
	targetArtefact := &blackboard.Artefact{
		ID:              uuid.New().String(),
		LogicalID:       uuid.New().String(),
		Version:         1, // iteration = version - 1 = 0
		StructuralType:  blackboard.StructuralTypeStandard,
		Type:            "CodeCommit",
		Payload:         "test-commit-hash",
		SourceArtefacts: []string{},
		ProducedByRole:  "Coder",
	}
	err := bbClient.CreateArtefact(ctx, targetArtefact)
	require.NoError(t, err)

	// Create original claim
	originalClaim := &blackboard.Claim{
		ID:                    uuid.New().String(),
		ArtefactID:            targetArtefact.ID,
		Status:                blackboard.ClaimStatusPendingReview,
		GrantedReviewAgents:   []string{"reviewer-agent"},
		GrantedParallelAgents: []string{},
		GrantedExclusiveAgent: "",
	}
	err = bbClient.CreateClaim(ctx, originalClaim)
	require.NoError(t, err)

	// Create review feedback artefacts
	review1 := &blackboard.Artefact{
		ID:              uuid.New().String(),
		LogicalID:       uuid.New().String(),
		Version:         1,
		StructuralType:  blackboard.StructuralTypeReview,
		Type:            "Review",
		Payload:         `{"issue": "needs tests"}`,
		SourceArtefacts: []string{targetArtefact.ID},
		ProducedByRole:  "Reviewer",
	}
	err = bbClient.CreateArtefact(ctx, review1)
	require.NoError(t, err)

	review2 := &blackboard.Artefact{
		ID:              uuid.New().String(),
		LogicalID:       uuid.New().String(),
		Version:         1,
		StructuralType:  blackboard.StructuralTypeReview,
		Type:            "Review",
		Payload:         `{"issue": "add documentation"}`,
		SourceArtefacts: []string{targetArtefact.ID},
		ProducedByRole:  "Reviewer",
	}
	err = bbClient.CreateArtefact(ctx, review2)
	require.NoError(t, err)

	feedbackArtefacts := []*blackboard.Artefact{review1, review2}

	// Call CreateFeedbackClaim - should create feedback claim
	err = engine.CreateFeedbackClaim(ctx, originalClaim, feedbackArtefacts)
	require.NoError(t, err)

	// Verify feedback claim was created and tracked
	assert.Len(t, engine.pendingAssignmentClaims, 1)

	// Get the feedback claim ID
	var feedbackClaimID string
	for claimID := range engine.pendingAssignmentClaims {
		feedbackClaimID = claimID
		break
	}

	// Verify feedback claim properties
	feedbackClaim, err := bbClient.GetClaim(ctx, feedbackClaimID)
	require.NoError(t, err)
	assert.Equal(t, blackboard.ClaimStatusPendingAssignment, feedbackClaim.Status)
	assert.Equal(t, "coder-agent", feedbackClaim.GrantedExclusiveAgent)
	assert.Equal(t, targetArtefact.ID, feedbackClaim.ArtefactID)
	assert.Len(t, feedbackClaim.AdditionalContextIDs, 2)
	assert.Contains(t, feedbackClaim.AdditionalContextIDs, review1.ID)
	assert.Contains(t, feedbackClaim.AdditionalContextIDs, review2.ID)
}

func TestCreateFeedbackClaim_MissingAgent(t *testing.T) {
	ctx := context.Background()
	maxIterations := 3
	engine, bbClient := setupTestEngineWithMaxIterations(t, maxIterations)

	// Create target artefact produced by non-existent role
	targetArtefact := &blackboard.Artefact{
		ID:              uuid.New().String(),
		LogicalID:       uuid.New().String(),
		Version:         1,
		StructuralType:  blackboard.StructuralTypeStandard,
		Type:            "CodeCommit",
		Payload:         "test-commit-hash",
		SourceArtefacts: []string{},
		ProducedByRole:  "NonExistentRole", // No agent has this role
	}
	err := bbClient.CreateArtefact(ctx, targetArtefact)
	require.NoError(t, err)

	// Create original claim
	originalClaim := &blackboard.Claim{
		ID:                    uuid.New().String(),
		ArtefactID:            targetArtefact.ID,
		Status:                blackboard.ClaimStatusPendingReview,
		GrantedReviewAgents:   []string{"reviewer-agent"},
		GrantedParallelAgents: []string{},
		GrantedExclusiveAgent: "",
	}
	err = bbClient.CreateClaim(ctx, originalClaim)
	require.NoError(t, err)

	// Create review feedback
	reviewArtefact := &blackboard.Artefact{
		ID:              uuid.New().String(),
		LogicalID:       uuid.New().String(),
		Version:         1,
		StructuralType:  blackboard.StructuralTypeReview,
		Type:            "Review",
		Payload:         `{"issue": "needs tests"}`,
		SourceArtefacts: []string{targetArtefact.ID},
		ProducedByRole:  "Reviewer",
	}
	err = bbClient.CreateArtefact(ctx, reviewArtefact)
	require.NoError(t, err)

	feedbackArtefacts := []*blackboard.Artefact{reviewArtefact}

	// Call CreateFeedbackClaim - should terminate due to missing agent
	err = engine.CreateFeedbackClaim(ctx, originalClaim, feedbackArtefacts)
	require.NoError(t, err)

	// Verify claim was terminated
	updatedClaim, err := bbClient.GetClaim(ctx, originalClaim.ID)
	require.NoError(t, err)
	assert.Equal(t, blackboard.ClaimStatusTerminated, updatedClaim.Status)
	assert.Contains(t, updatedClaim.TerminationReason, "missing agent configuration")
	assert.Contains(t, updatedClaim.TerminationReason, "NonExistentRole")
}

func TestCheckPendingAssignmentClaims(t *testing.T) {
	ctx := context.Background()
	maxIterations := 3
	engine, bbClient := setupTestEngineWithMaxIterations(t, maxIterations)

	// Create target artefact
	targetArtefact := &blackboard.Artefact{
		ID:              uuid.New().String(),
		LogicalID:       uuid.New().String(),
		Version:         1,
		StructuralType:  blackboard.StructuralTypeStandard,
		Type:            "CodeCommit",
		Payload:         "v1-hash",
		SourceArtefacts: []string{},
		ProducedByRole:  "Coder",
	}
	err := bbClient.CreateArtefact(ctx, targetArtefact)
	require.NoError(t, err)

	// Create review artefact for context
	reviewArtefact := &blackboard.Artefact{
		ID:              uuid.New().String(),
		LogicalID:       uuid.New().String(),
		Version:         1,
		StructuralType:  blackboard.StructuralTypeReview,
		Type:            "Review",
		Payload:         `{"issue": "needs fixes"}`,
		SourceArtefacts: []string{targetArtefact.ID},
		ProducedByRole:  "Reviewer",
	}
	err = bbClient.CreateArtefact(ctx, reviewArtefact)
	require.NoError(t, err)

	// Create feedback claim manually
	feedbackClaim := &blackboard.Claim{
		ID:                    uuid.New().String(),
		ArtefactID:            targetArtefact.ID,
		Status:                blackboard.ClaimStatusPendingAssignment,
		GrantedExclusiveAgent: "coder-agent",
		AdditionalContextIDs:  []string{reviewArtefact.ID},
		GrantedReviewAgents:   []string{},
		GrantedParallelAgents: []string{},
	}
	err = bbClient.CreateClaim(ctx, feedbackClaim)
	require.NoError(t, err)

	// Track the feedback claim in engine
	engine.pendingAssignmentClaims[feedbackClaim.ID] = targetArtefact.ID

	// Create v2 artefact (result of feedback claim)
	v2Artefact := &blackboard.Artefact{
		ID:              uuid.New().String(),
		LogicalID:       targetArtefact.LogicalID, // Same thread
		Version:         2,                        // Incremented version
		StructuralType:  blackboard.StructuralTypeStandard,
		Type:            "CodeCommit",
		Payload:         "v2-hash",
		SourceArtefacts: []string{targetArtefact.ID, reviewArtefact.ID},
		ProducedByRole:  "Coder", // Produced by granted agent
	}
	err = bbClient.CreateArtefact(ctx, v2Artefact)
	require.NoError(t, err)

	// Call checkPendingAssignmentClaims
	engine.checkPendingAssignmentClaims(ctx, v2Artefact)

	// Verify claim was marked complete
	updatedClaim, err := bbClient.GetClaim(ctx, feedbackClaim.ID)
	require.NoError(t, err)
	assert.Equal(t, blackboard.ClaimStatusComplete, updatedClaim.Status)

	// Verify claim was removed from tracking
	assert.Len(t, engine.pendingAssignmentClaims, 0)
}
