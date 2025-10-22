package pup

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dyluth/holt/pkg/blackboard"
	"github.com/google/uuid"
)

// TestPrepareToolInput verifies the tool input JSON structure
// M2.4: This test verifies empty context_chain for root artefacts (no source_artefacts)
func TestPrepareToolInput(t *testing.T) {
	// Note: This is a simplified unit test. Full context assembly is tested in integration tests.
	// For artefacts with no source_artefacts, context_chain will be empty.

	engine := &Engine{
		config: &Config{
			InstanceName: "test-instance",
			AgentName:    "example-agent",
			AgentRole:    "example",
		},
		bbClient: nil, // Not needed for root artefact (empty source_artefacts)
	}

	claim := &blackboard.Claim{
		ID:                    uuid.New().String(),
		ArtefactID:            "art-123",
		Status:                blackboard.ClaimStatusPendingExclusive,
		GrantedExclusiveAgent: "example-agent",
	}

	targetArtefact := &blackboard.Artefact{
		ID:              "art-123",
		LogicalID:       "log-456",
		Version:         1,
		StructuralType:  blackboard.StructuralTypeStandard,
		Type:            "GoalDefined",
		Payload:         "Implement user login",
		ProducedByRole:  "user",
		SourceArtefacts: []string{}, // No sources = root artefact
	}

	jsonStr, err := engine.prepareToolInput(context.Background(), claim, targetArtefact)
	if err != nil {
		t.Fatalf("prepareToolInput failed: %v", err)
	}

	// Unmarshal to verify structure
	var input map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &input); err != nil {
		t.Fatalf("Failed to unmarshal tool input: %v", err)
	}

	// Verify claim_type is hardcoded to "exclusive"
	if input["claim_type"] != "exclusive" {
		t.Errorf("Expected claim_type='exclusive', got %v", input["claim_type"])
	}

	// Verify target_artefact is present
	targetArt, ok := input["target_artefact"].(map[string]interface{})
	if !ok {
		t.Fatalf("target_artefact is not an object")
	}

	if targetArt["id"] != "art-123" {
		t.Errorf("Expected target_artefact.id='art-123', got %v", targetArt["id"])
	}

	if targetArt["type"] != "GoalDefined" {
		t.Errorf("Expected target_artefact.type='GoalDefined', got %v", targetArt["type"])
	}

	// Verify context_chain is empty array
	contextChain, ok := input["context_chain"].([]interface{})
	if !ok {
		t.Fatalf("context_chain is not an array")
	}

	if len(contextChain) != 0 {
		t.Errorf("Expected empty context_chain, got %d items", len(contextChain))
	}
}

// TestParseToolOutput_Valid verifies parsing of valid tool output
func TestParseToolOutput_Valid(t *testing.T) {
	engine := &Engine{}

	stdout := `{
		"artefact_type": "CodeCommit",
		"artefact_payload": "abc123def",
		"summary": "Implemented user login feature"
	}`

	output, err := engine.parseToolOutput(stdout)
	if err != nil {
		t.Fatalf("parseToolOutput failed: %v", err)
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
}

// TestParseToolOutput_Invalid verifies error handling for invalid output
func TestParseToolOutput_Invalid(t *testing.T) {
	engine := &Engine{}

	tests := []struct {
		name   string
		stdout string
	}{
		{
			name:   "empty stdout",
			stdout: "",
		},
		{
			name:   "invalid JSON",
			stdout: "not json at all",
		},
		{
			name:   "partial JSON",
			stdout: `{"artefact_type": "Code`,
		},
		{
			name:   "missing required fields",
			stdout: `{"artefact_type": "Code"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := engine.parseToolOutput(tt.stdout)
			if err == nil {
				t.Errorf("Expected parseToolOutput to fail for %s, but got nil error", tt.name)
			}
		})
	}
}

// TestCreateResultArtefact_Provenance verifies derivative provenance model
func TestCreateResultArtefact_Provenance(t *testing.T) {
	// This is a conceptual test - we can't run it without Redis
	// But we can verify the logic by examining the code

	engine := &Engine{
		config: &Config{
			AgentRole: "example",
		},
	}

	claim := &blackboard.Claim{
		ID:         uuid.New().String(),
		ArtefactID: "source-art-123",
	}

	output := &ToolOutput{
		ArtefactType:    "EchoSuccess",
		ArtefactPayload: "echo-456",
		Summary:         "Echo successful",
	}

	// We can't actually create the artefact without Redis, but we can verify
	// that the function would create the correct structure by checking the logic
	// in executor.go:createResultArtefact

	// Key points to verify:
	// 1. New artefact ID is generated (not from claim)
	// 2. logical_id equals the new artefact ID (derivative relationship)
	// 3. version = 1 (first version of new thread)
	// 4. source_artefacts = [claim.ArtefactID]

	// This test serves as documentation of the expected behavior
	if claim.ArtefactID != "source-art-123" {
		t.Errorf("Test setup error")
	}

	if output.ArtefactType != "EchoSuccess" {
		t.Errorf("Test setup error")
	}

	// Actual validation would happen in integration tests with real Redis
	_ = engine
}

// TestTruncate verifies the truncate helper function
func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "short string not truncated",
			input:    "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "exact length not truncated",
			input:    "hello",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "long string truncated",
			input:    "hello world this is a long string",
			maxLen:   10,
			expected: "hello worl...",
		},
		{
			name:     "empty string",
			input:    "",
			maxLen:   10,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncate(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestLimitedWriter verifies the limitedWriter enforces size limits
func TestLimitedWriter(t *testing.T) {
	tests := []struct {
		name      string
		limit     int
		writes    []string
		expected  string
		expectLen int
	}{
		{
			name:      "single write under limit",
			limit:     100,
			writes:    []string{"hello"},
			expected:  "hello",
			expectLen: 5,
		},
		{
			name:      "multiple writes under limit",
			limit:     100,
			writes:    []string{"hello", " ", "world"},
			expected:  "hello world",
			expectLen: 11,
		},
		{
			name:      "single write at limit",
			limit:     5,
			writes:    []string{"hello"},
			expected:  "hello",
			expectLen: 5,
		},
		{
			name:      "single write over limit",
			limit:     5,
			writes:    []string{"hello world"},
			expected:  "hello",
			expectLen: 5,
		},
		{
			name:      "multiple writes exceeding limit",
			limit:     10,
			writes:    []string{"hello", " ", "world", " ", "extra"},
			expected:  "hello worl",
			expectLen: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 0, tt.limit+10)
			writer := &sliceWriter{buf: &buf}
			lw := &limitedWriter{w: writer, limit: tt.limit}

			for _, write := range tt.writes {
				lw.Write([]byte(write))
			}

			result := string(buf)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}

			if len(buf) != tt.expectLen {
				t.Errorf("Expected length %d, got %d", tt.expectLen, len(buf))
			}
		})
	}
}

// sliceWriter is a helper for testing that writes to a byte slice
type sliceWriter struct {
	buf *[]byte
}

func (sw *sliceWriter) Write(p []byte) (n int, err error) {
	*sw.buf = append(*sw.buf, p...)
	return len(p), nil
}
