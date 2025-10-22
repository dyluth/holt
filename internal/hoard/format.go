package hoard

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/dyluth/holt/pkg/blackboard"
	"github.com/olekukonko/tablewriter"
)

// FormatTable writes artefacts as a formatted table to the provided writer.
// The table includes columns: ID, TYPE, PRODUCED BY, and PAYLOAD (truncated).
// Returns the number of artefacts formatted.
func FormatTable(w io.Writer, artefacts []*blackboard.Artefact, instanceName string) int {
	if len(artefacts) == 0 {
		fmt.Fprintf(w, "No artefacts found for instance '%s'\n", instanceName)
		return 0
	}

	// Print header
	fmt.Fprintf(w, "Artefacts for instance '%s':\n\n", instanceName)

	// Create table
	table := tablewriter.NewWriter(w)
	table.Header("ID", "TYPE", "PRODUCED BY", "PAYLOAD")

	// Add rows
	for _, a := range artefacts {
		table.Append([]string{
			a.ID,
			a.Type,
			formatProducedBy(a.ProducedByRole),
			formatPayload(a.Payload),
		})
	}

	// Render table
	table.Render()

	// Print count
	countMsg := "artefact"
	if len(artefacts) != 1 {
		countMsg = "artefacts"
	}
	fmt.Fprintf(w, "\n%d %s found\n", len(artefacts), countMsg)

	return len(artefacts)
}

// FormatJSONArray writes artefacts as a JSON array to the provided writer.
// The array contains complete artefact objects with proper JSON formatting.
func FormatJSONArray(w io.Writer, artefacts []*blackboard.Artefact) error {
	// Marshal to pretty JSON
	data, err := json.MarshalIndent(artefacts, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal artefacts to JSON: %w", err)
	}

	// Write to output
	_, err = w.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write JSON output: %w", err)
	}

	// Add newline for clean output
	fmt.Fprintln(w)

	return nil
}

// FormatSingleJSON writes a single artefact as pretty-printed JSON to the provided writer.
// Used in get mode to display complete artefact details.
func FormatSingleJSON(w io.Writer, artefact *blackboard.Artefact) error {
	// Marshal to pretty JSON
	data, err := json.MarshalIndent(artefact, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal artefact to JSON: %w", err)
	}

	// Write to output
	_, err = w.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write JSON output: %w", err)
	}

	// Add newline for clean output
	fmt.Fprintln(w)

	return nil
}

// formatPayload truncates payload to first line with max 60 characters for table display.
// Multi-line payloads show only the first line. Empty payloads return "-".
func formatPayload(payload string) string {
	if payload == "" {
		return "-"
	}

	// Get first non-empty line
	lines := strings.Split(payload, "\n")
	var firstLine string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			firstLine = trimmed
			break
		}
	}

	// If all lines were empty
	if firstLine == "" {
		return "-"
	}

	// Truncate to 60 chars
	if len(firstLine) > 60 {
		return firstLine[:57] + "..."
	}

	return firstLine
}

// formatProducedBy formats the produced_by_role field for table display.
// Empty values return "-".
func formatProducedBy(role string) string {
	if role == "" {
		return "-"
	}
	return role
}
