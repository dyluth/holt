package hoard

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/dyluth/holt/pkg/blackboard"
)

// OutputFormat specifies how to format the artefact list output.
type OutputFormat string

const (
	// OutputFormatDefault uses a table format with truncated payloads
	OutputFormatDefault OutputFormat = "default"

	// OutputFormatJSONL outputs complete artefacts as line-delimited JSON
	OutputFormatJSONL OutputFormat = "jsonl"
)

// FilterCriteria defines filtering options for hoard list command.
// All filters are ANDed together.
type FilterCriteria struct {
	SinceTimestampMs int64  // Unix timestamp in milliseconds, 0 = no filter
	UntilTimestampMs int64  // Unix timestamp in milliseconds, 0 = no filter
	TypeGlob         string // Glob pattern for artefact type, empty = no filter
	AgentRole        string // Exact match for produced_by_role, empty = no filter
}

// matchesFilter returns true if the artefact matches all filter criteria.
func (fc *FilterCriteria) matchesFilter(art *blackboard.Artefact) bool {
	// Time filtering
	if fc.SinceTimestampMs > 0 && art.CreatedAtMs < fc.SinceTimestampMs {
		return false
	}
	if fc.UntilTimestampMs > 0 && art.CreatedAtMs > fc.UntilTimestampMs {
		return false
	}

	// Type filtering - glob pattern matching
	if fc.TypeGlob != "" {
		matched, err := filepath.Match(fc.TypeGlob, art.Type)
		if err != nil || !matched {
			return false
		}
	}

	// Agent filtering - exact match on produced_by_role
	if fc.AgentRole != "" && art.ProducedByRole != fc.AgentRole {
		return false
	}

	return true
}

// ListArtefacts retrieves all artefacts for an instance and writes them to the provided writer.
// Uses Redis SCAN to iterate over artefact keys without blocking the server.
// Applies filter criteria if provided. Sorts artefacts by creation time for stable output.
// Skips malformed artefacts with a warning to stderr but continues processing.
func ListArtefacts(ctx context.Context, bbClient *blackboard.Client, instanceName string, format OutputFormat, filters *FilterCriteria, w io.Writer) error {
	// Scan for all artefact keys using Redis SCAN
	pattern := fmt.Sprintf("holt:%s:artefact:*", instanceName)
	iter := bbClient.RedisClient().Scan(ctx, 0, pattern, 0).Iterator()

	var artefacts []*blackboard.Artefact

	// Iterate over all matching keys
	for iter.Next(ctx) {
		key := iter.Val()

		// Extract artefact ID from key (format: holt:{instance}:artefact:{id})
		artefactID := key[len(fmt.Sprintf("holt:%s:artefact:", instanceName)):]

		// Fetch artefact
		artefact, err := bbClient.GetArtefact(ctx, artefactID)
		if err != nil {
			// Skip malformed artefacts with warning to stderr
			fmt.Fprintf(os.Stderr, "⚠️  Skipping malformed artefact: key=%s (error: %v)\n", key, err)
			continue
		}

		// Apply filters if provided
		if filters != nil && !filters.matchesFilter(artefact) {
			continue
		}

		artefacts = append(artefacts, artefact)
	}

	// Check for iteration errors
	if err := iter.Err(); err != nil {
		return fmt.Errorf("failed to scan artefacts: %w", err)
	}

	// Sort by creation time (oldest first) for chronological output
	sort.Slice(artefacts, func(i, j int) bool {
		return artefacts[i].CreatedAtMs < artefacts[j].CreatedAtMs
	})

	// Format output based on requested format
	switch format {
	case OutputFormatDefault:
		FormatTable(w, artefacts, instanceName)
	case OutputFormatJSONL:
		if err := FormatJSONL(w, artefacts); err != nil {
			return fmt.Errorf("failed to format JSONL output: %w", err)
		}
	default:
		return fmt.Errorf("unknown output format: %s", format)
	}

	return nil
}
