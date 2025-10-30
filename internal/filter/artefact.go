package filter

import (
	"path/filepath"

	"github.com/dyluth/holt/pkg/blackboard"
)

// Criteria defines filtering criteria for artefacts.
// All filters are ANDed together - an artefact must match ALL criteria to pass.
type Criteria struct {
	SinceTimestampMs int64  // Unix timestamp in milliseconds, 0 = no filter
	UntilTimestampMs int64  // Unix timestamp in milliseconds, 0 = no filter
	TypeGlob         string // Glob pattern for artefact type, empty = no filter
	AgentRole        string // Exact match for produced_by_role, empty = no filter
}

// Matches returns true if the artefact matches all filter criteria.
// Empty/zero criteria values are treated as "match all" for that criterion.
func (c *Criteria) Matches(art *blackboard.Artefact) bool {
	// Time filtering - check CreatedAtMs field
	if c.SinceTimestampMs > 0 && art.CreatedAtMs < c.SinceTimestampMs {
		return false
	}
	if c.UntilTimestampMs > 0 && art.CreatedAtMs > c.UntilTimestampMs {
		return false
	}

	// Type filtering - glob pattern matching
	if c.TypeGlob != "" {
		matched, err := filepath.Match(c.TypeGlob, art.Type)
		if err != nil || !matched {
			return false
		}
	}

	// Agent filtering - exact match on produced_by_role
	if c.AgentRole != "" && art.ProducedByRole != c.AgentRole {
		return false
	}

	return true
}

// HasFilters returns true if any filters are active.
// Used to determine if historical query is needed.
func (c *Criteria) HasFilters() bool {
	return c.SinceTimestampMs > 0 ||
		c.UntilTimestampMs > 0 ||
		c.TypeGlob != "" ||
		c.AgentRole != ""
}
