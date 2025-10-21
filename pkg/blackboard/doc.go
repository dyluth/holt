// Package blackboard provides type-safe Go definitions and Redis schema patterns
// for the Holt blackboard architecture.
//
// # Overview
//
// The blackboard is the central shared state system where all Holt components
// (orchestrator, pups, CLI) interact via well-defined data structures stored in Redis.
// It implements the Blackboard architectural pattern - a shared workspace where
// independent agents collaborate by reading and writing structured data.
//
// # Core Concepts
//
// Artefacts are immutable work products that represent the fundamental unit of state
// in Holt. Every piece of work, decision, and result is represented as an artefact
// with complete provenance tracking via source_artefacts and produced_by_role fields.
//
// Claims represent the orchestrator's decision about how an artefact should be processed.
// They coordinate the phased execution model: review → parallel → exclusive.
//
// Bids represent an agent's interest in working on a claim. Agents can bid to review,
// work in parallel, request exclusive access, or ignore an artefact.
//
// # Multi-Instance Support
//
// All Redis keys and Pub/Sub channels are namespaced by instance name to enable
// multiple Holt instances to safely coexist on a single Redis server without
// interference. Each instance has complete isolation of its data and events.
//
// # Usage Example
//
//	import "github.com/dyluth/holt/pkg/blackboard"
//
//	// Create an artefact
//	artefact := &blackboard.Artefact{
//		ID:              uuid.New().String(),
//		LogicalID:       uuid.New().String(),
//		Version:         1,
//		StructuralType:  blackboard.StructuralTypeStandard,
//		Type:            "CodeCommit",
//		Payload:         "abc123def",
//		SourceArtefacts: []string{},
//		ProducedByRole:  "go-coder",
//	}
//
//	// Validate before storing
//	if err := artefact.Validate(); err != nil {
//		log.Fatal(err)
//	}
//
//	// Generate Redis key for this artefact
//	key := blackboard.ArtefactKey("default-1", artefact.ID)
//	// key = "holt:default-1:artefact:<uuid>"
//
//	// Convert to Redis hash format for storage (M1.2 will do this)
//	hash, err := blackboard.ArtefactToHash(artefact)
//	if err != nil {
//		log.Fatal(err)
//	}
//
// # Redis Schema
//
// All Redis keys follow the pattern: holt:{instance_name}:{entity}:{uuid}
//
// Artefacts: holt:{instance_name}:artefact:{artefact_id}
// Claims: holt:{instance_name}:claim:{claim_id}
// Claim Bids: holt:{instance_name}:claim:{claim_id}:bids
// Threads: holt:{instance_name}:thread:{logical_id}
//
// Pub/Sub channels: holt:{instance_name}:{event_type}_events
//
// Artefact Events: holt:{instance_name}:artefact_events
// Claim Events: holt:{instance_name}:claim_events
//
// # Design Principles
//
// - Type Safety: All data structures have strong typing with validation methods
// - Immutability: Artefacts are immutable once created
// - Auditability: Complete provenance via source artefacts and producer tracking
// - Isolation: Instance namespacing prevents cross-instance interference
// - Simplicity: Minimal dependencies (only google/uuid for validation)
package blackboard
