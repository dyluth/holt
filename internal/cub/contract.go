package cub

import (
	"encoding/json"
	"fmt"

	"github.com/dyluth/sett/pkg/blackboard"
)

// ToolInput represents the JSON structure passed to agent tools via stdin.
// The agent tool reads this JSON from stdin to understand what work to perform.
//
// Contract: The cub marshals this struct to JSON and writes it to the tool's stdin,
// then immediately closes the stdin pipe.
//
// Example JSON:
//
//	{
//	  "claim_type": "exclusive",
//	  "target_artefact": {
//	    "id": "abc-123",
//	    "type": "GoalDefined",
//	    "payload": "Implement user login",
//	    ...
//	  },
//	  "context_chain": []
//	}
type ToolInput struct {
	// ClaimType indicates the type of claim granted ("exclusive", "claim", "review")
	// M2.3: Always "exclusive" (hardcoded)
	// M3+: May be "claim" or "review" for parallel/review phases
	ClaimType string `json:"claim_type"`

	// TargetArtefact is the full artefact object that the claim is for.
	// The tool operates on this artefact to produce new work.
	TargetArtefact *blackboard.Artefact `json:"target_artefact"`

	// ContextChain is the ordered list of ancestor artefacts providing context.
	// M2.3: Always empty array []
	// M2.4+: Populated by context assembly algorithm
	ContextChain []interface{} `json:"context_chain"`
}

// ToolOutput represents the JSON structure that agent tools write to stdout.
// The agent tool produces this JSON on stdout to describe the artefact it created.
//
// Contract: The tool must write exactly ONE valid JSON object to stdout and exit.
// Multiple JSON objects or partial JSON will result in a Failure artefact.
//
// Example JSON:
//
//	{
//	  "artefact_type": "CodeCommit",
//	  "artefact_payload": "abc123def",
//	  "summary": "Implemented user login feature",
//	  "structural_type": "Standard"
//	}
type ToolOutput struct {
	// ArtefactType is the user-defined domain type (e.g., "CodeCommit", "DesignSpec")
	// This becomes the artefact's Type field.
	// Required - must be non-empty string.
	ArtefactType string `json:"artefact_type"`

	// ArtefactPayload is the main content of the artefact (git hash, JSON, text)
	// This becomes the artefact's Payload field.
	// Required - may be empty string if semantically valid.
	ArtefactPayload string `json:"artefact_payload"`

	// Summary is a human-readable description of what the tool did.
	// Required - must be non-empty string.
	Summary string `json:"summary"`

	// StructuralType optionally specifies the artefact's structural type.
	// If omitted, defaults to "Standard".
	// Valid values: "Standard", "Review", "Question", "Answer", "Failure", "Terminal"
	StructuralType string `json:"structural_type,omitempty"`
}

// Validate checks that the ToolOutput has all required fields and valid values.
// Returns an error if validation fails.
func (o *ToolOutput) Validate() error {
	if o.ArtefactType == "" {
		return fmt.Errorf("artefact_type is required and cannot be empty")
	}

	if o.Summary == "" {
		return fmt.Errorf("summary is required and cannot be empty")
	}

	// ArtefactPayload may be empty (e.g., some artefacts have no payload)
	// StructuralType is optional - will default to "Standard" if empty

	// If StructuralType is provided, validate it's a known type
	if o.StructuralType != "" {
		st := blackboard.StructuralType(o.StructuralType)
		if err := st.Validate(); err != nil {
			return fmt.Errorf("invalid structural_type: %w", err)
		}
	}

	return nil
}

// GetStructuralType returns the structural type to use for the artefact,
// defaulting to "Standard" if not specified.
func (o *ToolOutput) GetStructuralType() blackboard.StructuralType {
	if o.StructuralType == "" {
		return blackboard.StructuralTypeStandard
	}
	return blackboard.StructuralType(o.StructuralType)
}

// FailureData represents the structured data stored in Failure artefact payloads.
// This provides detailed diagnostic information about tool execution failures.
type FailureData struct {
	// Reason is a high-level description of why the failure occurred
	Reason string `json:"reason"`

	// ExitCode is the subprocess exit code (0 = success, non-zero = failure)
	// May be -1 for timeout or signal termination
	ExitCode int `json:"exit_code,omitempty"`

	// Stdout contains the captured standard output from the tool
	Stdout string `json:"stdout,omitempty"`

	// Stderr contains the captured standard error from the tool
	Stderr string `json:"stderr,omitempty"`

	// Error provides additional error context (e.g., JSON parse error message)
	Error string `json:"error,omitempty"`
}

// MarshalFailurePayload converts FailureData to a pretty-printed JSON string
// suitable for storing in an artefact's Payload field.
func MarshalFailurePayload(data *FailureData) (string, error) {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal failure data: %w", err)
	}
	return string(bytes), nil
}
