package watch

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dyluth/holt/pkg/blackboard"
	"github.com/stretchr/testify/assert"
)

// TestNewWorkflowEvents tests the formatting of the unified workflow events
func TestNewWorkflowEvents(t *testing.T) {
	tests := []struct {
		name     string
		event    *blackboard.WorkflowEvent
		expected string
	}{
		{
			name: "review_approved",
			event: &blackboard.WorkflowEvent{
				Event: "review_approved",
				Data: map[string]interface{}{
					"reviewer_role":        "Validator",
					"original_artefact_id": "def45678-1234-1234-1234-123456789012",
				},
			},
			expected: "✅ Review Approved: by=Validator for artefact def45678-1234-1234-1234-123456789012",
		},
		{
			name: "review_rejected",
			event: &blackboard.WorkflowEvent{
				Event: "review_rejected",
				Data: map[string]interface{}{
					"reviewer_role":        "Validator",
					"original_artefact_id": "def45678-1234-1234-1234-123456789012",
					"feedback":             "This needs improvement",
				},
			},
			expected: "❌ Review Rejected: by=Validator for artefact def45678-1234-1234-1234-123456789012",
		},
		{
			name: "feedback_claim_created",
			event: &blackboard.WorkflowEvent{
				Event: "feedback_claim_created",
				Data: map[string]interface{}{
					"feedback_claim_id": "ghi78901-1234-1234-1234-123456789012",
					"original_claim_id": "abc123",
					"target_agent_role": "Writer",
					"iteration":         2,
				},
			},
			expected: "🔄 Rework Assigned: to=Writer for claim ghi78901-1234-1234-1234-123456789012 (iteration 2)",
		},
		{
			name: "artefact_reworked",
			event: &blackboard.WorkflowEvent{
				Event: "artefact_reworked",
				Data: map[string]interface{}{
					"new_artefact_id":     "jkl34567-1234-1234-1234-123456789012",
					"logical_id":          "mno56789-1234-1234-1234-123456789012",
					"new_version":         2,
					"previous_version_id": "pqr67890-1234-1234-1234-123456789012",
					"artefact_type":       "RecipeYAML",
					"produced_by_role":    "Writer",
				},
			},
			expected: "🔄 Artefact Reworked (v2): by=Writer, type=RecipeYAML, id=jkl34567-1234-1234-1234-123456789012",
		},
		{
			name: "artefact_reworked with iteration as float64 (JSON unmarshaling)",
			event: &blackboard.WorkflowEvent{
				Event: "artefact_reworked",
				Data: map[string]interface{}{
					"new_artefact_id":     "jkl34567-1234-1234-1234-123456789012",
					"logical_id":          "mno56789-1234-1234-1234-123456789012",
					"new_version":         float64(3), // JSON numbers unmarshal as float64
					"previous_version_id": "pqr67890-1234-1234-1234-123456789012",
					"artefact_type":       "RecipeYAML",
					"produced_by_role":    "Writer",
				},
			},
			expected: "🔄 Artefact Reworked (v3): by=Writer, type=RecipeYAML, id=jkl34567-1234-1234-1234-123456789012",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			formatter := &defaultFormatter{writer: buf}

			err := formatter.FormatWorkflow(tt.event)
			assert.NoError(t, err)

			output := buf.String()
			// Check that the expected string is in the output (ignoring timestamp)
			assert.True(t, strings.Contains(output, tt.expected),
				"Expected output to contain '%s', got: %s", tt.expected, output)
		})
	}
}

// TestArtefactFiltering tests that Review and reworked artefacts are filtered out
func TestArtefactFiltering(t *testing.T) {
	tests := []struct {
		name       string
		artefact   *blackboard.Artefact
		shouldShow bool
	}{
		{
			name: "Standard artefact v1 should be shown",
			artefact: &blackboard.Artefact{
				ID:             "test-id",
				StructuralType: blackboard.StructuralTypeStandard,
				Version:        1,
				ProducedByRole: "Writer",
				Type:           "CodeCommit",
			},
			shouldShow: true,
		},
		{
			name: "Review artefact should be filtered",
			artefact: &blackboard.Artefact{
				ID:             "test-id",
				StructuralType: blackboard.StructuralTypeReview,
				Version:        1,
				ProducedByRole: "Validator",
				Type:           "Review",
			},
			shouldShow: false,
		},
		{
			name: "Reworked artefact (v2) should be filtered",
			artefact: &blackboard.Artefact{
				ID:             "test-id",
				StructuralType: blackboard.StructuralTypeStandard,
				Version:        2,
				ProducedByRole: "Writer",
				Type:           "CodeCommit",
			},
			shouldShow: false,
		},
		{
			name: "Reworked artefact (v3) should be filtered",
			artefact: &blackboard.Artefact{
				ID:             "test-id",
				StructuralType: blackboard.StructuralTypeStandard,
				Version:        3,
				ProducedByRole: "Writer",
				Type:           "RecipeYAML",
			},
			shouldShow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			formatter := &defaultFormatter{writer: buf}

			err := formatter.FormatArtefact(tt.artefact)
			assert.NoError(t, err)

			output := buf.String()
			if tt.shouldShow {
				assert.Contains(t, output, "✨ Artefact created")
			} else {
				assert.Empty(t, output, "Artefact should be filtered out")
			}
		})
	}
}
