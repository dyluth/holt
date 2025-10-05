package blackboard

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"

	"github.com/google/uuid"
)

// TestArtefactRoundTrip tests that artefact serialization and deserialization maintains perfect fidelity
func TestArtefactRoundTrip(t *testing.T) {
	original := &Artefact{
		ID:              uuid.New().String(),
		LogicalID:       uuid.New().String(),
		Version:         1,
		StructuralType:  StructuralTypeStandard,
		Type:            "CodeCommit",
		Payload:         "abc123def",
		SourceArtefacts: []string{uuid.New().String(), uuid.New().String()},
		ProducedByRole:  "go-coder",
	}

	// Convert to hash
	hash, err := ArtefactToHash(original)
	if err != nil {
		t.Fatalf("ArtefactToHash failed: %v", err)
	}

	// Convert hash to string map (simulating Redis storage)
	stringHash := make(map[string]string)
	for k, v := range hash {
		stringHash[k] = toString(v)
	}

	// Convert back to artefact
	result, err := HashToArtefact(stringHash)
	if err != nil {
		t.Fatalf("HashToArtefact failed: %v", err)
	}

	// Verify perfect round-trip
	if !reflect.DeepEqual(original, result) {
		t.Errorf("round-trip failed:\noriginal: %+v\nresult:   %+v", original, result)
	}
}

// TestArtefactRoundTrip_EmptySourceArtefacts tests round-trip with empty source artefacts array
func TestArtefactRoundTrip_EmptySourceArtefacts(t *testing.T) {
	original := &Artefact{
		ID:              uuid.New().String(),
		LogicalID:       uuid.New().String(),
		Version:         1,
		StructuralType:  StructuralTypeStandard,
		Type:            "GoalDefined",
		Payload:         "Create a REST API",
		SourceArtefacts: []string{}, // Empty array
		ProducedByRole:  "user",
	}

	hash, err := ArtefactToHash(original)
	if err != nil {
		t.Fatalf("ArtefactToHash failed: %v", err)
	}

	stringHash := make(map[string]string)
	for k, v := range hash {
		stringHash[k] = toString(v)
	}

	result, err := HashToArtefact(stringHash)
	if err != nil {
		t.Fatalf("HashToArtefact failed: %v", err)
	}

	if !reflect.DeepEqual(original, result) {
		t.Errorf("round-trip with empty array failed:\noriginal: %+v\nresult:   %+v", original, result)
	}

	// Specifically check that result has empty slice, not nil
	if result.SourceArtefacts == nil {
		t.Error("deserialized source artefacts should be empty slice, not nil")
	}
}

// TestArtefactRoundTrip_NilSourceArtefacts tests that nil source artefacts converts to empty array
func TestArtefactRoundTrip_NilSourceArtefacts(t *testing.T) {
	original := &Artefact{
		ID:              uuid.New().String(),
		LogicalID:       uuid.New().String(),
		Version:         1,
		StructuralType:  StructuralTypeStandard,
		Type:            "GoalDefined",
		Payload:         "Create a REST API",
		SourceArtefacts: nil, // Nil slice
		ProducedByRole:  "user",
	}

	hash, err := ArtefactToHash(original)
	if err != nil {
		t.Fatalf("ArtefactToHash failed: %v", err)
	}

	stringHash := make(map[string]string)
	for k, v := range hash {
		stringHash[k] = toString(v)
	}

	result, err := HashToArtefact(stringHash)
	if err != nil {
		t.Fatalf("HashToArtefact failed: %v", err)
	}

	// Result should have empty slice, not nil
	if result.SourceArtefacts == nil {
		t.Error("nil source artefacts should deserialize to empty slice")
	}
	if len(result.SourceArtefacts) != 0 {
		t.Errorf("nil source artefacts should deserialize to empty slice, got length %d", len(result.SourceArtefacts))
	}
}

// TestHashToArtefact_MalformedJSON tests that malformed JSON in hash fails gracefully
func TestHashToArtefact_MalformedJSON(t *testing.T) {
	hash := map[string]string{
		"id":               uuid.New().String(),
		"logical_id":       uuid.New().String(),
		"version":          "1",
		"structural_type":  "Standard",
		"type":             "CodeCommit",
		"payload":          "abc123",
		"source_artefacts": "not-valid-json", // Malformed JSON
		"produced_by_role": "go-coder",
	}

	_, err := HashToArtefact(hash)
	if err == nil {
		t.Error("expected error for malformed source_artefacts JSON, got nil")
	}
}

// TestHashToArtefact_InvalidVersion tests that invalid version fails gracefully
func TestHashToArtefact_InvalidVersion(t *testing.T) {
	hash := map[string]string{
		"id":               uuid.New().String(),
		"logical_id":       uuid.New().String(),
		"version":          "not-a-number", // Invalid version
		"structural_type":  "Standard",
		"type":             "CodeCommit",
		"payload":          "abc123",
		"source_artefacts": "[]",
		"produced_by_role": "go-coder",
	}

	_, err := HashToArtefact(hash)
	if err == nil {
		t.Error("expected error for invalid version, got nil")
	}
}

// TestClaimRoundTrip tests that claim serialization and deserialization maintains perfect fidelity
func TestClaimRoundTrip(t *testing.T) {
	original := &Claim{
		ID:                    uuid.New().String(),
		ArtefactID:            uuid.New().String(),
		Status:                ClaimStatusPendingReview,
		GrantedReviewAgents:   []string{"agent-1", "agent-2"},
		GrantedParallelAgents: []string{"agent-3"},
		GrantedExclusiveAgent: "",
	}

	// Convert to hash
	hash, err := ClaimToHash(original)
	if err != nil {
		t.Fatalf("ClaimToHash failed: %v", err)
	}

	// Convert hash to string map (simulating Redis storage)
	stringHash := make(map[string]string)
	for k, v := range hash {
		stringHash[k] = toString(v)
	}

	// Convert back to claim
	result, err := HashToClaim(stringHash)
	if err != nil {
		t.Fatalf("HashToClaim failed: %v", err)
	}

	// Verify perfect round-trip
	if !reflect.DeepEqual(original, result) {
		t.Errorf("round-trip failed:\noriginal: %+v\nresult:   %+v", original, result)
	}
}

// TestClaimRoundTrip_EmptyAgentArrays tests round-trip with empty agent arrays
func TestClaimRoundTrip_EmptyAgentArrays(t *testing.T) {
	original := &Claim{
		ID:                    uuid.New().String(),
		ArtefactID:            uuid.New().String(),
		Status:                ClaimStatusPendingReview,
		GrantedReviewAgents:   []string{},
		GrantedParallelAgents: []string{},
		GrantedExclusiveAgent: "",
	}

	hash, err := ClaimToHash(original)
	if err != nil {
		t.Fatalf("ClaimToHash failed: %v", err)
	}

	stringHash := make(map[string]string)
	for k, v := range hash {
		stringHash[k] = toString(v)
	}

	result, err := HashToClaim(stringHash)
	if err != nil {
		t.Fatalf("HashToClaim failed: %v", err)
	}

	if !reflect.DeepEqual(original, result) {
		t.Errorf("round-trip with empty arrays failed:\noriginal: %+v\nresult:   %+v", original, result)
	}

	// Specifically check that arrays are empty slices, not nil
	if result.GrantedReviewAgents == nil {
		t.Error("deserialized granted_review_agents should be empty slice, not nil")
	}
	if result.GrantedParallelAgents == nil {
		t.Error("deserialized granted_parallel_agents should be empty slice, not nil")
	}
}

// TestHashToClaim_MalformedJSON tests that malformed JSON in hash fails gracefully
func TestHashToClaim_MalformedJSON(t *testing.T) {
	hash := map[string]string{
		"id":                      uuid.New().String(),
		"artefact_id":             uuid.New().String(),
		"status":                  "pending_review",
		"granted_review_agents":   "not-valid-json", // Malformed JSON
		"granted_parallel_agents": "[]",
		"granted_exclusive_agent": "",
	}

	_, err := HashToClaim(hash)
	if err == nil {
		t.Error("expected error for malformed granted_review_agents JSON, got nil")
	}
}

// TestArtefactToHash_AllStructuralTypes tests serialization of all structural types
func TestArtefactToHash_AllStructuralTypes(t *testing.T) {
	structuralTypes := []StructuralType{
		StructuralTypeStandard,
		StructuralTypeReview,
		StructuralTypeQuestion,
		StructuralTypeAnswer,
		StructuralTypeFailure,
		StructuralTypeTerminal,
	}

	for _, st := range structuralTypes {
		t.Run(string(st), func(t *testing.T) {
			artefact := &Artefact{
				ID:              uuid.New().String(),
				LogicalID:       uuid.New().String(),
				Version:         1,
				StructuralType:  st,
				Type:            "Test",
				Payload:         "test payload",
				SourceArtefacts: []string{},
				ProducedByRole:  "tester",
			}

			hash, err := ArtefactToHash(artefact)
			if err != nil {
				t.Fatalf("ArtefactToHash failed for %s: %v", st, err)
			}

			if hash["structural_type"] != string(st) {
				t.Errorf("structural_type mismatch: got %q, expected %q", hash["structural_type"], st)
			}
		})
	}
}

// Helper function to convert interface{} to string (simulates Redis storage)
func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case int:
		return strconv.Itoa(val)
	default:
		return fmt.Sprintf("%v", v)
	}
}
