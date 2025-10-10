package cub

import (
	"testing"

	"github.com/dyluth/sett/pkg/blackboard"
)

// TestFilterContextArtefacts verifies filtering to Standard and Answer only
func TestFilterContextArtefacts(t *testing.T) {
	contextMap := map[string]*blackboard.Artefact{
		"log-1": {
			LogicalID:      "log-1",
			Type:           "GoalDefined",
			StructuralType: blackboard.StructuralTypeStandard,
		},
		"log-2": {
			LogicalID:      "log-2",
			Type:           "DesignSpec",
			StructuralType: blackboard.StructuralTypeStandard,
		},
		"log-3": {
			LogicalID:      "log-3",
			Type:           "ToolFailure",
			StructuralType: blackboard.StructuralTypeFailure,
		},
		"log-4": {
			LogicalID:      "log-4",
			Type:           "UserAnswer",
			StructuralType: blackboard.StructuralTypeAnswer,
		},
		"log-5": {
			LogicalID:      "log-5",
			Type:           "CodeReview",
			StructuralType: blackboard.StructuralTypeReview,
		},
	}

	filtered := filterContextArtefacts(contextMap)

	// Should include Standard and Answer only (3 artefacts)
	if len(filtered) != 3 {
		t.Errorf("Expected 3 filtered artefacts, got %d", len(filtered))
	}

	// Verify only Standard and Answer types present
	for _, art := range filtered {
		if art.StructuralType != blackboard.StructuralTypeStandard &&
			art.StructuralType != blackboard.StructuralTypeAnswer {
			t.Errorf("Filtered artefact has wrong structural_type: %s", art.StructuralType)
		}
	}

	// Verify Failure and Review were filtered out
	for _, art := range filtered {
		if art.LogicalID == "log-3" || art.LogicalID == "log-5" {
			t.Errorf("Failure/Review artefact should have been filtered out: %s", art.LogicalID)
		}
	}
}

// TestFilterContextArtefacts_EmptyMap verifies empty map returns empty slice
func TestFilterContextArtefacts_EmptyMap(t *testing.T) {
	contextMap := make(map[string]*blackboard.Artefact)
	filtered := filterContextArtefacts(contextMap)

	if len(filtered) != 0 {
		t.Errorf("Expected empty filtered slice, got %d artefacts", len(filtered))
	}
}

// TestFilterContextArtefacts_AllFiltered verifies all artefacts can be filtered
func TestFilterContextArtefacts_AllFiltered(t *testing.T) {
	contextMap := map[string]*blackboard.Artefact{
		"log-1": {
			LogicalID:      "log-1",
			StructuralType: blackboard.StructuralTypeFailure,
		},
		"log-2": {
			LogicalID:      "log-2",
			StructuralType: blackboard.StructuralTypeReview,
		},
	}

	filtered := filterContextArtefacts(contextMap)

	if len(filtered) != 0 {
		t.Errorf("Expected all artefacts filtered out, got %d", len(filtered))
	}
}

// TestSortContextChronologically verifies oldest-first ordering
func TestSortContextChronologically(t *testing.T) {
	// Input in BFS order (newest first, discovered from target backwards)
	artefacts := []*blackboard.Artefact{
		{LogicalID: "newest", Type: "Third"},
		{LogicalID: "middle", Type: "Second"},
		{LogicalID: "oldest", Type: "First"},
	}

	sorted := sortContextChronologically(artefacts)

	// Should be reversed (oldest first)
	if len(sorted) != 3 {
		t.Fatalf("Expected 3 sorted artefacts, got %d", len(sorted))
	}

	if sorted[0].LogicalID != "oldest" {
		t.Errorf("Expected oldest artefact first, got %s", sorted[0].LogicalID)
	}

	if sorted[1].LogicalID != "middle" {
		t.Errorf("Expected middle artefact second, got %s", sorted[1].LogicalID)
	}

	if sorted[2].LogicalID != "newest" {
		t.Errorf("Expected newest artefact last, got %s", sorted[2].LogicalID)
	}
}

// TestSortContextChronologically_EmptySlice verifies empty slice handled
func TestSortContextChronologically_EmptySlice(t *testing.T) {
	artefacts := []*blackboard.Artefact{}
	sorted := sortContextChronologically(artefacts)

	if len(sorted) != 0 {
		t.Errorf("Expected empty sorted slice, got %d artefacts", len(sorted))
	}
}

// TestSortContextChronologically_SingleArtefact verifies single artefact
func TestSortContextChronologically_SingleArtefact(t *testing.T) {
	artefacts := []*blackboard.Artefact{
		{LogicalID: "only", Type: "OnlyOne"},
	}

	sorted := sortContextChronologically(artefacts)

	if len(sorted) != 1 {
		t.Fatalf("Expected 1 sorted artefact, got %d", len(sorted))
	}

	if sorted[0].LogicalID != "only" {
		t.Errorf("Expected 'only' artefact, got %s", sorted[0].LogicalID)
	}
}
