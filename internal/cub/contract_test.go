package cub

import (
	"encoding/json"
	"testing"

	"github.com/dyluth/sett/pkg/blackboard"
)

// TestToolInput_JSONMarshaling verifies that ToolInput marshals to correct JSON structure
func TestToolInput_JSONMarshaling(t *testing.T) {
	artefact := &blackboard.Artefact{
		ID:             "art-123",
		LogicalID:      "log-456",
		Version:        1,
		StructuralType: blackboard.StructuralTypeStandard,
		Type:           "GoalDefined",
		Payload:        "Implement user login",
		ProducedByRole: "user",
	}

	input := &ToolInput{
		ClaimType:      "exclusive",
		TargetArtefact: artefact,
		ContextChain:   []interface{}{},
	}

	jsonBytes, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal ToolInput: %v", err)
	}

	// Unmarshal to verify structure
	var unmarshaled map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Verify top-level fields
	if unmarshaled["claim_type"] != "exclusive" {
		t.Errorf("Expected claim_type='exclusive', got %v", unmarshaled["claim_type"])
	}

	targetArt, ok := unmarshaled["target_artefact"].(map[string]interface{})
	if !ok {
		t.Fatalf("target_artefact is not an object")
	}

	if targetArt["id"] != "art-123" {
		t.Errorf("Expected target_artefact.id='art-123', got %v", targetArt["id"])
	}

	if targetArt["type"] != "GoalDefined" {
		t.Errorf("Expected target_artefact.type='GoalDefined', got %v", targetArt["type"])
	}

	contextChain, ok := unmarshaled["context_chain"].([]interface{})
	if !ok {
		t.Fatalf("context_chain is not an array")
	}

	if len(contextChain) != 0 {
		t.Errorf("Expected empty context_chain, got %d items", len(contextChain))
	}
}

// TestToolOutput_JSONUnmarshaling_AllFields verifies unmarshaling with all fields present
func TestToolOutput_JSONUnmarshaling_AllFields(t *testing.T) {
	jsonStr := `{
		"artefact_type": "CodeCommit",
		"artefact_payload": "abc123def",
		"summary": "Implemented user login feature",
		"structural_type": "Standard"
	}`

	var output ToolOutput
	if err := json.Unmarshal([]byte(jsonStr), &output); err != nil {
		t.Fatalf("Failed to unmarshal ToolOutput: %v", err)
	}

	if output.ArtefactType != "CodeCommit" {
		t.Errorf("Expected ArtefactType='CodeCommit', got %q", output.ArtefactType)
	}

	if output.ArtefactPayload != "abc123def" {
		t.Errorf("Expected ArtefactPayload='abc123def', got %q", output.ArtefactPayload)
	}

	if output.Summary != "Implemented user login feature" {
		t.Errorf("Expected Summary='Implemented user login feature', got %q", output.Summary)
	}

	if output.StructuralType != "Standard" {
		t.Errorf("Expected StructuralType='Standard', got %q", output.StructuralType)
	}
}

// TestToolOutput_JSONUnmarshaling_RequiredFieldsOnly verifies unmarshaling with optional fields omitted
func TestToolOutput_JSONUnmarshaling_RequiredFieldsOnly(t *testing.T) {
	jsonStr := `{
		"artefact_type": "EchoSuccess",
		"artefact_payload": "echo-123",
		"summary": "Echo agent processed the claim"
	}`

	var output ToolOutput
	if err := json.Unmarshal([]byte(jsonStr), &output); err != nil {
		t.Fatalf("Failed to unmarshal ToolOutput: %v", err)
	}

	if output.ArtefactType != "EchoSuccess" {
		t.Errorf("Expected ArtefactType='EchoSuccess', got %q", output.ArtefactType)
	}

	if output.ArtefactPayload != "echo-123" {
		t.Errorf("Expected ArtefactPayload='echo-123', got %q", output.ArtefactPayload)
	}

	if output.Summary != "Echo agent processed the claim" {
		t.Errorf("Expected Summary='Echo agent processed the claim', got %q", output.Summary)
	}

	if output.StructuralType != "" {
		t.Errorf("Expected StructuralType='', got %q", output.StructuralType)
	}
}

// TestToolOutput_Validate_Success verifies validation passes for valid output
func TestToolOutput_Validate_Success(t *testing.T) {
	tests := []struct {
		name   string
		output *ToolOutput
	}{
		{
			name: "all fields present",
			output: &ToolOutput{
				ArtefactType:    "CodeCommit",
				ArtefactPayload: "abc123",
				Summary:         "Implemented feature",
				StructuralType:  "Standard",
			},
		},
		{
			name: "required fields only",
			output: &ToolOutput{
				ArtefactType:    "EchoSuccess",
				ArtefactPayload: "echo-123",
				Summary:         "Echo successful",
			},
		},
		{
			name: "empty payload is valid",
			output: &ToolOutput{
				ArtefactType:    "EmptyResult",
				ArtefactPayload: "",
				Summary:         "No payload needed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.output.Validate(); err != nil {
				t.Errorf("Expected validation to pass, got error: %v", err)
			}
		})
	}
}

// TestToolOutput_Validate_Failures verifies validation fails for invalid output
func TestToolOutput_Validate_Failures(t *testing.T) {
	tests := []struct {
		name        string
		output      *ToolOutput
		expectedErr string
	}{
		{
			name: "missing artefact_type",
			output: &ToolOutput{
				ArtefactPayload: "abc123",
				Summary:         "Done",
			},
			expectedErr: "artefact_type is required",
		},
		{
			name: "missing summary",
			output: &ToolOutput{
				ArtefactType:    "CodeCommit",
				ArtefactPayload: "abc123",
			},
			expectedErr: "summary is required",
		},
		{
			name: "invalid structural_type",
			output: &ToolOutput{
				ArtefactType:    "CodeCommit",
				ArtefactPayload: "abc123",
				Summary:         "Done",
				StructuralType:  "InvalidType",
			},
			expectedErr: "invalid structural_type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.output.Validate()
			if err == nil {
				t.Fatalf("Expected validation to fail with %q, but got nil", tt.expectedErr)
			}
			if !contains(err.Error(), tt.expectedErr) {
				t.Errorf("Expected error to contain %q, got %q", tt.expectedErr, err.Error())
			}
		})
	}
}

// TestToolOutput_GetStructuralType verifies default behavior
func TestToolOutput_GetStructuralType(t *testing.T) {
	tests := []struct {
		name     string
		output   *ToolOutput
		expected blackboard.StructuralType
	}{
		{
			name: "defaults to Standard when empty",
			output: &ToolOutput{
				StructuralType: "",
			},
			expected: blackboard.StructuralTypeStandard,
		},
		{
			name: "returns specified type",
			output: &ToolOutput{
				StructuralType: "Failure",
			},
			expected: blackboard.StructuralTypeFailure,
		},
		{
			name: "returns Terminal type",
			output: &ToolOutput{
				StructuralType: "Terminal",
			},
			expected: blackboard.StructuralTypeTerminal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.output.GetStructuralType()
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestMarshalFailurePayload verifies Failure artefact payload marshaling
func TestMarshalFailurePayload(t *testing.T) {
	data := &FailureData{
		Reason:   "Tool execution failed",
		ExitCode: 1,
		Stdout:   "partial output",
		Stderr:   "error: something went wrong",
		Error:    "process exited with code 1",
	}

	payload, err := MarshalFailurePayload(data)
	if err != nil {
		t.Fatalf("Failed to marshal failure payload: %v", err)
	}

	// Verify it's valid JSON
	var unmarshaled FailureData
	if err := json.Unmarshal([]byte(payload), &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal failure payload: %v", err)
	}

	// Verify fields are preserved
	if unmarshaled.Reason != "Tool execution failed" {
		t.Errorf("Expected Reason='Tool execution failed', got %q", unmarshaled.Reason)
	}

	if unmarshaled.ExitCode != 1 {
		t.Errorf("Expected ExitCode=1, got %d", unmarshaled.ExitCode)
	}

	if unmarshaled.Stdout != "partial output" {
		t.Errorf("Expected Stdout='partial output', got %q", unmarshaled.Stdout)
	}

	if unmarshaled.Stderr != "error: something went wrong" {
		t.Errorf("Expected Stderr='error: something went wrong', got %q", unmarshaled.Stderr)
	}

	// Verify pretty-printing (contains newlines and indentation)
	if !contains(payload, "\n") {
		t.Errorf("Expected pretty-printed JSON with newlines, got: %s", payload)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
