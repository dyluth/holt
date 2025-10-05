package blackboard

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// Serialization helpers for converting between Go structs and Redis hashes
//
// Redis stores data as string-to-string maps (hashes). Complex fields like arrays
// are JSON-encoded into single hash fields. This provides a balance between
// queryability (individual fields) and flexibility (complex structures).

// ArtefactToHash converts an Artefact struct to a Redis hash format.
// Array fields (source_artefacts) are JSON-encoded.
func ArtefactToHash(a *Artefact) (map[string]interface{}, error) {
	// Encode source artefacts array as JSON
	sourceArtefactsJSON, err := json.Marshal(a.SourceArtefacts)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal source artefacts: %w", err)
	}

	hash := map[string]interface{}{
		"id":               a.ID,
		"logical_id":       a.LogicalID,
		"version":          a.Version,
		"structural_type":  string(a.StructuralType),
		"type":             a.Type,
		"payload":          a.Payload,
		"source_artefacts": string(sourceArtefactsJSON),
		"produced_by_role": a.ProducedByRole,
	}

	return hash, nil
}

// HashToArtefact converts a Redis hash to an Artefact struct.
// JSON fields are decoded back to Go types.
func HashToArtefact(hash map[string]string) (*Artefact, error) {
	// Parse version number
	version, err := strconv.Atoi(hash["version"])
	if err != nil {
		return nil, fmt.Errorf("invalid version field: %w", err)
	}

	// Decode source artefacts JSON array
	var sourceArtefacts []string
	if sourceArtefactsJSON := hash["source_artefacts"]; sourceArtefactsJSON != "" {
		if err := json.Unmarshal([]byte(sourceArtefactsJSON), &sourceArtefacts); err != nil {
			return nil, fmt.Errorf("failed to unmarshal source_artefacts: %w", err)
		}
	}

	// Ensure we have an empty slice instead of nil for consistency
	if sourceArtefacts == nil {
		sourceArtefacts = []string{}
	}

	artefact := &Artefact{
		ID:              hash["id"],
		LogicalID:       hash["logical_id"],
		Version:         version,
		StructuralType:  StructuralType(hash["structural_type"]),
		Type:            hash["type"],
		Payload:         hash["payload"],
		SourceArtefacts: sourceArtefacts,
		ProducedByRole:  hash["produced_by_role"],
	}

	return artefact, nil
}

// ClaimToHash converts a Claim struct to a Redis hash format.
// Array fields (granted_review_agents, granted_parallel_agents) are JSON-encoded.
func ClaimToHash(c *Claim) (map[string]interface{}, error) {
	// Encode agent arrays as JSON
	reviewAgentsJSON, err := json.Marshal(c.GrantedReviewAgents)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal granted_review_agents: %w", err)
	}

	parallelAgentsJSON, err := json.Marshal(c.GrantedParallelAgents)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal granted_parallel_agents: %w", err)
	}

	hash := map[string]interface{}{
		"id":                      c.ID,
		"artefact_id":             c.ArtefactID,
		"status":                  string(c.Status),
		"granted_review_agents":   string(reviewAgentsJSON),
		"granted_parallel_agents": string(parallelAgentsJSON),
		"granted_exclusive_agent": c.GrantedExclusiveAgent,
	}

	return hash, nil
}

// HashToClaim converts a Redis hash to a Claim struct.
// JSON fields are decoded back to Go types.
func HashToClaim(hash map[string]string) (*Claim, error) {
	// Decode granted review agents JSON array
	var reviewAgents []string
	if reviewAgentsJSON := hash["granted_review_agents"]; reviewAgentsJSON != "" {
		if err := json.Unmarshal([]byte(reviewAgentsJSON), &reviewAgents); err != nil {
			return nil, fmt.Errorf("failed to unmarshal granted_review_agents: %w", err)
		}
	}

	// Decode granted parallel agents JSON array
	var parallelAgents []string
	if parallelAgentsJSON := hash["granted_parallel_agents"]; parallelAgentsJSON != "" {
		if err := json.Unmarshal([]byte(parallelAgentsJSON), &parallelAgents); err != nil {
			return nil, fmt.Errorf("failed to unmarshal granted_parallel_agents: %w", err)
		}
	}

	// Ensure we have empty slices instead of nil for consistency
	if reviewAgents == nil {
		reviewAgents = []string{}
	}
	if parallelAgents == nil {
		parallelAgents = []string{}
	}

	claim := &Claim{
		ID:                    hash["id"],
		ArtefactID:            hash["artefact_id"],
		Status:                ClaimStatus(hash["status"]),
		GrantedReviewAgents:   reviewAgents,
		GrantedParallelAgents: parallelAgents,
		GrantedExclusiveAgent: hash["granted_exclusive_agent"],
	}

	return claim, nil
}
