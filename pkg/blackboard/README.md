# Blackboard Package

Type-safe Go definitions and Redis schema patterns for the Holt blackboard architecture.

## Purpose

The blackboard is the central shared state system where all Holt components (orchestrator, pups, CLI) interact via well-defined data structures stored in Redis.

## Installation

```bash
go get github.com/dyluth/holt/pkg/blackboard
```

## Quick Start

```go
import (
	"github.com/dyluth/holt/pkg/blackboard"
	"github.com/google/uuid"
)

// Create and validate an artefact
artefact := &blackboard.Artefact{
	ID:              uuid.New().String(),
	LogicalID:       uuid.New().String(),
	Version:         1,
	StructuralType:  blackboard.StructuralTypeStandard,
	Type:            "CodeCommit",
	Payload:         "abc123def",
	SourceArtefacts: []string{},
	ProducedByRole:  "go-coder",
}

if err := artefact.Validate(); err != nil {
	log.Fatal(err)
}

// Generate Redis key
key := blackboard.ArtefactKey("default-1", artefact.ID)
// Returns: "holt:default-1:artefact:<uuid>"

// Convert to Redis hash format
hash, err := blackboard.ArtefactToHash(artefact)
```

## Core Types

### Artefact
Immutable work product on the blackboard. Every piece of work, decision, and result is represented as an artefact with complete provenance.

### Claim
Represents the orchestrator's decision about an artefact. Tracks which agents have been granted access and coordinates phased execution (review → parallel → exclusive).

### Bid
Represents an agent's interest in a claim. Values: `review`, `claim` (parallel), `exclusive`, `ignore`.

### Structural Types
- `Standard` - Normal work artefacts
- `Review` - Review feedback artefacts
- `Question` - Questions escalated to humans
- `Answer` - Human answers to questions
- `Failure` - Agent failures
- `Terminal` - Workflow completion

## Redis Schema

All keys are namespaced by instance name:

```
# Instance-specific keys
holt:{instance_name}:artefact:{uuid}       # Artefact data
holt:{instance_name}:claim:{uuid}          # Claim data
holt:{instance_name}:claim:{uuid}:bids     # Bid data
holt:{instance_name}:thread:{logical_id}   # Version tracking (ZSET)

# Pub/Sub channels
holt:{instance_name}:artefact_events       # Artefact creation events
holt:{instance_name}:claim_events          # Claim creation events
```

## Helper Functions

### Key Generation
- `ArtefactKey(instanceName, artefactID string) string`
- `ClaimKey(instanceName, claimID string) string`
- `ClaimBidsKey(instanceName, claimID string) string`
- `ThreadKey(instanceName, logicalID string) string`

### Channel Names
- `ArtefactEventsChannel(instanceName string) string`
- `ClaimEventsChannel(instanceName string) string`

### Serialization
- `ArtefactToHash(a *Artefact) (map[string]interface{}, error)`
- `HashToArtefact(hash map[string]string) (*Artefact, error)`
- `ClaimToHash(c *Claim) (map[string]interface{}, error)`
- `HashToClaim(hash map[string]string) (*Claim, error)`

### Thread Utilities
- `ThreadScore(version int) float64`
- `VersionFromScore(score float64) int`

## Validation

All core types have `Validate() error` methods that check:
- UUID format correctness
- Required fields are non-empty
- Enum values are valid
- Version numbers are >= 1

## Testing

```bash
# Run tests
go test ./pkg/blackboard/

# With coverage
go test -cover ./pkg/blackboard/
# Target: >= 90% coverage
```

## Design Principles

- **Type Safety**: Strong typing with validation methods
- **Immutability**: Artefacts are immutable once created
- **Auditability**: Complete provenance tracking
- **Isolation**: Instance namespacing prevents cross-instance interference
- **Simplicity**: Minimal dependencies (only google/uuid)

## Dependencies

- `github.com/google/uuid` - UUID validation and generation

## Documentation

Full API documentation available via godoc:

```bash
godoc -http=:6060
# Navigate to localhost:6060/pkg/github.com/dyluth/holt/pkg/blackboard/
```

## License

See project root LICENSE file.
