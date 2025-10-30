package hoard

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dyluth/holt/pkg/blackboard"
)

// FormatTable writes artefacts as a formatted table to the provided writer.
// The table includes columns: ID, VERSION, TYPE, PRODUCED BY, TIMESTAMP, and PAYLOAD (truncated).
// Returns the number of artefacts formatted.
func FormatTable(w io.Writer, artefacts []*blackboard.Artefact, instanceName string) int {
	if len(artefacts) == 0 {
		fmt.Fprintf(w, "No artefacts found for instance '%s'\n", instanceName)
		return 0
	}

	// Print header
	fmt.Fprintf(w, "Artefacts for instance '%s':\n\n", instanceName)

	// Print header row
	fmt.Fprintf(w, "%-10s %-5s %-10s %-18s %-8s %s\n",
		"ID", "VER", "TYPE", "BY", "AGE", "PAYLOAD")
	fmt.Fprintf(w, "%-10s %-5s %-10s %-18s %-8s %s\n",
		"----------", "-----", "----------", "------------------", "--------", "----------------------------------------")

	// Print data rows
	for _, a := range artefacts {
		fmt.Fprintf(w, "%-10s %-5s %-10s %-18s %-8s %s\n",
			formatID(a.ID),
			formatVersion(a.Version),
			formatType(a.Type),
			formatProducedBy(a.ProducedByRole),
			formatTimestamp(a.CreatedAtMs),
			formatPayload(a.Payload),
		)
	}

	// Print count
	countMsg := "artefact"
	if len(artefacts) != 1 {
		countMsg = "artefacts"
	}
	fmt.Fprintf(w, "\n%d %s found\n", len(artefacts), countMsg)

	return len(artefacts)
}

// FormatJSONL writes artefacts as line-delimited JSON (JSONL) to the provided writer.
// Each artefact is written as a single JSON object on its own line.
// This format is ideal for streaming and processing with tools like jq.
func FormatJSONL(w io.Writer, artefacts []*blackboard.Artefact) error {
	for _, artefact := range artefacts {
		// Marshal artefact to JSON (compact, no indentation)
		data, err := json.Marshal(artefact)
		if err != nil {
			return fmt.Errorf("failed to marshal artefact to JSON: %w", err)
		}

		// Write as single line
		_, err = fmt.Fprintf(w, "%s\n", string(data))
		if err != nil {
			return fmt.Errorf("failed to write JSONL output: %w", err)
		}
	}

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

// formatID truncates artefact ID to first 8 characters for compact display.
func formatID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// formatType truncates type names for compact display.
// Shortens common types to save space.
func formatType(typeName string) string {
	// Shorten common type names
	switch typeName {
	case "TerraformCode":
		return "TfCode"
	case "TerraformDocumentation":
		return "TfDocs"
	case "FormattedDocumentation":
		return "FmtDocs"
	case "PackagedModule":
		return "Package"
	case "GoalDefined":
		return "Goal"
	case "ToolExecutionFailure":
		return "Failure"
	}

	// Truncate long type names
	if len(typeName) > 20 {
		return typeName[:17] + "..."
	}
	return typeName
}

// formatPayload truncates payload to first line with max 40 characters for table display.
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

	// Truncate to 40 chars (shorter for compact display)
	if len(firstLine) > 40 {
		return firstLine[:37] + "..."
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

// formatVersion formats the version number for table display.
// Shows "v1", "v2", etc. for versions > 1, or "-" for version 1 (initial artefact).
func formatVersion(version int) string {
	if version <= 1 {
		return "-"
	}
	return fmt.Sprintf("v%d", version)
}

// formatTimestamp formats Unix timestamp in milliseconds to human-readable time.
// Shows relative time like "2m ago", "1h ago", etc.
func formatTimestamp(timestampMs int64) string {
	if timestampMs == 0 {
		return "-"
	}

	// Convert ms to time
	t := time.Unix(timestampMs/1000, (timestampMs%1000)*1000000)

	// Calculate time difference from now
	diff := time.Since(t)

	// Format as relative time
	if diff < time.Minute {
		return fmt.Sprintf("%ds ago", int(diff.Seconds()))
	} else if diff < time.Hour {
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	} else if diff < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	} else {
		return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
	}
}
