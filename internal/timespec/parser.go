package timespec

import (
	"fmt"
	"time"
)

// Parse parses a time specification into a Unix timestamp (milliseconds).
// Supports two formats:
//   - Go duration format: "1h", "30m", "1h30m", "2h45m30s"
//   - RFC3339 timestamps: "2025-10-29T13:00:00Z"
//
// Duration specifications are relative to the current time (subtracted from now).
// For example, "1h" means "1 hour ago".
//
// Returns Unix timestamp in milliseconds.
func Parse(spec string) (int64, error) {
	if spec == "" {
		return 0, fmt.Errorf("empty time specification")
	}

	// Try parsing as RFC3339 first
	if t, err := time.Parse(time.RFC3339, spec); err == nil {
		return t.UnixMilli(), nil
	}

	// Try parsing as Go duration
	if d, err := time.ParseDuration(spec); err == nil {
		// Duration is relative to now (subtract from current time)
		return time.Now().Add(-d).UnixMilli(), nil
	}

	return 0, fmt.Errorf("invalid time specification: %s (use duration like '1h30m' or RFC3339 like '2025-10-29T13:00:00Z')", spec)
}

// ParseRange parses both --since and --until flags into a time range.
// Returns (sinceTimestampMs, untilTimestampMs, error).
// Zero values indicate "no bound" for that end of the range.
//
// Validates that since < until if both are specified.
func ParseRange(since, until string) (int64, int64, error) {
	var sinceMS, untilMS int64
	var err error

	if since != "" {
		sinceMS, err = Parse(since)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid --since: %w", err)
		}
	}

	if until != "" {
		untilMS, err = Parse(until)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid --until: %w", err)
		}
	}

	// Validate range
	if sinceMS > 0 && untilMS > 0 && sinceMS >= untilMS {
		return 0, 0, fmt.Errorf("--since must be before --until")
	}

	return sinceMS, untilMS, nil
}
