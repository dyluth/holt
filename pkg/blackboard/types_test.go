package blackboard

import (
	"testing"

	"github.com/google/uuid"
)

// TestArtefactValidate_Valid tests that valid artefacts pass validation
func TestArtefactValidate_Valid(t *testing.T) {
	artefact := &Artefact{
		ID:              uuid.New().String(),
		LogicalID:       uuid.New().String(),
		Version:         1,
		StructuralType:  StructuralTypeStandard,
		Type:            "CodeCommit",
		Payload:         "abc123",
		SourceArtefacts: []string{uuid.New().String(), uuid.New().String()},
		ProducedByRole:  "go-coder",
	}

	if err := artefact.Validate(); err != nil {
		t.Errorf("valid artefact failed validation: %v", err)
	}
}

// TestArtefactValidate_EmptySourceArtefacts tests that empty source artefacts array is valid
func TestArtefactValidate_EmptySourceArtefacts(t *testing.T) {
	artefact := &Artefact{
		ID:              uuid.New().String(),
		LogicalID:       uuid.New().String(),
		Version:         1,
		StructuralType:  StructuralTypeStandard,
		Type:            "GoalDefined",
		Payload:         "Create a REST API",
		SourceArtefacts: []string{}, // Empty is valid for root artefacts
		ProducedByRole:  "user",
	}

	if err := artefact.Validate(); err != nil {
		t.Errorf("artefact with empty source artefacts failed validation: %v", err)
	}
}

// TestArtefactValidate_InvalidID tests that invalid artefact ID fails validation
func TestArtefactValidate_InvalidID(t *testing.T) {
	artefact := &Artefact{
		ID:             "not-a-uuid",
		LogicalID:      uuid.New().String(),
		Version:        1,
		StructuralType: StructuralTypeStandard,
		Type:           "CodeCommit",
		Payload:        "abc123",
		ProducedByRole: "go-coder",
	}

	if err := artefact.Validate(); err == nil {
		t.Error("expected validation to fail for invalid ID, but it passed")
	}
}

// TestArtefactValidate_InvalidLogicalID tests that invalid logical ID fails validation
func TestArtefactValidate_InvalidLogicalID(t *testing.T) {
	artefact := &Artefact{
		ID:             uuid.New().String(),
		LogicalID:      "not-a-uuid",
		Version:        1,
		StructuralType: StructuralTypeStandard,
		Type:           "CodeCommit",
		Payload:        "abc123",
		ProducedByRole: "go-coder",
	}

	if err := artefact.Validate(); err == nil {
		t.Error("expected validation to fail for invalid logical ID, but it passed")
	}
}

// TestArtefactValidate_InvalidVersion tests that version < 1 fails validation
func TestArtefactValidate_InvalidVersion(t *testing.T) {
	testCases := []struct {
		name    string
		version int
	}{
		{"version 0", 0},
		{"negative version", -1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			artefact := &Artefact{
				ID:             uuid.New().String(),
				LogicalID:      uuid.New().String(),
				Version:        tc.version,
				StructuralType: StructuralTypeStandard,
				Type:           "CodeCommit",
				Payload:        "abc123",
				ProducedByRole: "go-coder",
			}

			if err := artefact.Validate(); err == nil {
				t.Errorf("expected validation to fail for version %d, but it passed", tc.version)
			}
		})
	}
}

// TestArtefactValidate_InvalidStructuralType tests that invalid structural type fails validation
func TestArtefactValidate_InvalidStructuralType(t *testing.T) {
	artefact := &Artefact{
		ID:             uuid.New().String(),
		LogicalID:      uuid.New().String(),
		Version:        1,
		StructuralType: "InvalidType",
		Type:           "CodeCommit",
		Payload:        "abc123",
		ProducedByRole: "go-coder",
	}

	if err := artefact.Validate(); err == nil {
		t.Error("expected validation to fail for invalid structural type, but it passed")
	}
}

// TestArtefactValidate_EmptyType tests that empty type fails validation
func TestArtefactValidate_EmptyType(t *testing.T) {
	artefact := &Artefact{
		ID:             uuid.New().String(),
		LogicalID:      uuid.New().String(),
		Version:        1,
		StructuralType: StructuralTypeStandard,
		Type:           "",
		Payload:        "abc123",
		ProducedByRole: "go-coder",
	}

	if err := artefact.Validate(); err == nil {
		t.Error("expected validation to fail for empty type, but it passed")
	}
}

// TestArtefactValidate_EmptyProducedByRole tests that empty produced_by_role fails validation
func TestArtefactValidate_EmptyProducedByRole(t *testing.T) {
	artefact := &Artefact{
		ID:             uuid.New().String(),
		LogicalID:      uuid.New().String(),
		Version:        1,
		StructuralType: StructuralTypeStandard,
		Type:           "CodeCommit",
		Payload:        "abc123",
		ProducedByRole: "",
	}

	if err := artefact.Validate(); err == nil {
		t.Error("expected validation to fail for empty produced_by_role, but it passed")
	}
}

// TestArtefactValidate_InvalidSourceArtefact tests that invalid UUID in source artefacts fails validation
func TestArtefactValidate_InvalidSourceArtefact(t *testing.T) {
	artefact := &Artefact{
		ID:              uuid.New().String(),
		LogicalID:       uuid.New().String(),
		Version:         1,
		StructuralType:  StructuralTypeStandard,
		Type:            "CodeCommit",
		Payload:         "abc123",
		SourceArtefacts: []string{uuid.New().String(), "not-a-uuid", uuid.New().String()},
		ProducedByRole:  "go-coder",
	}

	if err := artefact.Validate(); err == nil {
		t.Error("expected validation to fail for invalid source artefact UUID, but it passed")
	}
}

// TestStructuralTypeValidate_AllValid tests all valid structural types
func TestStructuralTypeValidate_AllValid(t *testing.T) {
	validTypes := []StructuralType{
		StructuralTypeStandard,
		StructuralTypeReview,
		StructuralTypeQuestion,
		StructuralTypeAnswer,
		StructuralTypeFailure,
		StructuralTypeTerminal,
	}

	for _, st := range validTypes {
		t.Run(string(st), func(t *testing.T) {
			if err := st.Validate(); err != nil {
				t.Errorf("valid structural type %q failed validation: %v", st, err)
			}
		})
	}
}

// TestStructuralTypeValidate_Invalid tests invalid structural type
func TestStructuralTypeValidate_Invalid(t *testing.T) {
	invalidType := StructuralType("InvalidType")
	if err := invalidType.Validate(); err == nil {
		t.Error("expected validation to fail for invalid structural type, but it passed")
	}
}

// TestClaimValidate_Valid tests that valid claims pass validation
func TestClaimValidate_Valid(t *testing.T) {
	claim := &Claim{
		ID:                    uuid.New().String(),
		ArtefactID:            uuid.New().String(),
		Status:                ClaimStatusPendingReview,
		GrantedReviewAgents:   []string{"agent-1", "agent-2"},
		GrantedParallelAgents: []string{"agent-3"},
		GrantedExclusiveAgent: "",
	}

	if err := claim.Validate(); err != nil {
		t.Errorf("valid claim failed validation: %v", err)
	}
}

// TestClaimValidate_EmptyAgentArrays tests that empty agent arrays are valid
func TestClaimValidate_EmptyAgentArrays(t *testing.T) {
	claim := &Claim{
		ID:                    uuid.New().String(),
		ArtefactID:            uuid.New().String(),
		Status:                ClaimStatusPendingReview,
		GrantedReviewAgents:   []string{},
		GrantedParallelAgents: []string{},
		GrantedExclusiveAgent: "",
	}

	if err := claim.Validate(); err != nil {
		t.Errorf("claim with empty agent arrays failed validation: %v", err)
	}
}

// TestClaimValidate_InvalidID tests that invalid claim ID fails validation
func TestClaimValidate_InvalidID(t *testing.T) {
	claim := &Claim{
		ID:         "not-a-uuid",
		ArtefactID: uuid.New().String(),
		Status:     ClaimStatusPendingReview,
	}

	if err := claim.Validate(); err == nil {
		t.Error("expected validation to fail for invalid claim ID, but it passed")
	}
}

// TestClaimValidate_InvalidArtefactID tests that invalid artefact ID fails validation
func TestClaimValidate_InvalidArtefactID(t *testing.T) {
	claim := &Claim{
		ID:         uuid.New().String(),
		ArtefactID: "not-a-uuid",
		Status:     ClaimStatusPendingReview,
	}

	if err := claim.Validate(); err == nil {
		t.Error("expected validation to fail for invalid artefact ID, but it passed")
	}
}

// TestClaimValidate_InvalidStatus tests that invalid status fails validation
func TestClaimValidate_InvalidStatus(t *testing.T) {
	claim := &Claim{
		ID:         uuid.New().String(),
		ArtefactID: uuid.New().String(),
		Status:     "invalid-status",
	}

	if err := claim.Validate(); err == nil {
		t.Error("expected validation to fail for invalid status, but it passed")
	}
}

// TestClaimStatusValidate_AllValid tests all valid claim statuses
func TestClaimStatusValidate_AllValid(t *testing.T) {
	validStatuses := []ClaimStatus{
		ClaimStatusPendingReview,
		ClaimStatusPendingParallel,
		ClaimStatusPendingExclusive,
		ClaimStatusComplete,
		ClaimStatusTerminated,
	}

	for _, status := range validStatuses {
		t.Run(string(status), func(t *testing.T) {
			if err := status.Validate(); err != nil {
				t.Errorf("valid claim status %q failed validation: %v", status, err)
			}
		})
	}
}

// TestClaimStatusValidate_Invalid tests invalid claim status
func TestClaimStatusValidate_Invalid(t *testing.T) {
	invalidStatus := ClaimStatus("invalid-status")
	if err := invalidStatus.Validate(); err == nil {
		t.Error("expected validation to fail for invalid claim status, but it passed")
	}
}

// TestBidTypeValidate_AllValid tests all valid bid types
func TestBidTypeValidate_AllValid(t *testing.T) {
	validBidTypes := []BidType{
		BidTypeReview,
		BidTypeParallel,
		BidTypeExclusive,
		BidTypeIgnore,
	}

	for _, bt := range validBidTypes {
		t.Run(string(bt), func(t *testing.T) {
			if err := bt.Validate(); err != nil {
				t.Errorf("valid bid type %q failed validation: %v", bt, err)
			}
		})
	}
}

// TestBidTypeValidate_Invalid tests invalid bid type
func TestBidTypeValidate_Invalid(t *testing.T) {
	invalidBidType := BidType("invalid-bid")
	if err := invalidBidType.Validate(); err == nil {
		t.Error("expected validation to fail for invalid bid type, but it passed")
	}
}

// TestIsValidUUID tests the UUID validation helper
func TestIsValidUUID(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid UUID v4", uuid.New().String(), true},
		{"valid UUID with hyphens", "550e8400-e29b-41d4-a716-446655440000", true},
		{"valid UUID without hyphens", "550e8400e29b41d4a716446655440000", true}, // google/uuid accepts this format
		{"invalid - not a UUID", "not-a-uuid", false},
		{"invalid - empty string", "", false},
		{"invalid - random string", "abc123", false},
		{"invalid - too short", "550e8400", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isValidUUID(tc.input)
			if result != tc.expected {
				t.Errorf("isValidUUID(%q) = %v, expected %v", tc.input, result, tc.expected)
			}
		})
	}
}
