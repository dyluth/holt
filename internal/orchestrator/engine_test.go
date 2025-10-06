package orchestrator

import (
	"testing"

	"github.com/dyluth/sett/pkg/blackboard"
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
