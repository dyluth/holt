package hoard

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/dyluth/sett/pkg/blackboard"
)

// OutputFormat specifies how to format the artefact list output.
type OutputFormat string

const (
	// OutputFormatDefault uses a table format with truncated payloads
	OutputFormatDefault OutputFormat = "default"

	// OutputFormatJSON outputs complete artefacts as a JSON array
	OutputFormatJSON OutputFormat = "json"
)

// ListArtefacts retrieves all artefacts for an instance and writes them to the provided writer.
// Uses Redis SCAN to iterate over artefact keys without blocking the server.
// Sorts artefacts alphabetically by ID for stable, predictable output.
// Skips malformed artefacts with a warning to stderr but continues processing.
func ListArtefacts(ctx context.Context, bbClient *blackboard.Client, instanceName string, format OutputFormat, w io.Writer) error {
	// Scan for all artefact keys using Redis SCAN
	pattern := fmt.Sprintf("sett:%s:artefact:*", instanceName)
	iter := bbClient.RedisClient().Scan(ctx, 0, pattern, 0).Iterator()

	var artefacts []*blackboard.Artefact

	// Iterate over all matching keys
	for iter.Next(ctx) {
		key := iter.Val()

		// Extract artefact ID from key (format: sett:{instance}:artefact:{id})
		artefactID := key[len(fmt.Sprintf("sett:%s:artefact:", instanceName)):]

		// Fetch artefact
		artefact, err := bbClient.GetArtefact(ctx, artefactID)
		if err != nil {
			// Skip malformed artefacts with warning to stderr
			fmt.Fprintf(os.Stderr, "⚠️  Skipping malformed artefact: key=%s (error: %v)\n", key, err)
			continue
		}

		artefacts = append(artefacts, artefact)
	}

	// Check for iteration errors
	if err := iter.Err(); err != nil {
		return fmt.Errorf("failed to scan artefacts: %w", err)
	}

	// Sort alphabetically by ID for stable output
	sort.Slice(artefacts, func(i, j int) bool {
		return artefacts[i].ID < artefacts[j].ID
	})

	// Format output based on requested format
	switch format {
	case OutputFormatDefault:
		FormatTable(w, artefacts, instanceName)
	case OutputFormatJSON:
		if err := FormatJSONArray(w, artefacts); err != nil {
			return fmt.Errorf("failed to format JSON output: %w", err)
		}
	default:
		return fmt.Errorf("unknown output format: %s", format)
	}

	return nil
}
