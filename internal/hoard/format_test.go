package hoard

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dyluth/sett/pkg/blackboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatPayload(t *testing.T) {
	tests := []struct {
		name     string
		payload  string
		expected string
	}{
		{
			name:     "empty payload",
			payload:  "",
			expected: "-",
		},
		{
			name:     "short single line",
			payload:  "hello.txt",
			expected: "hello.txt",
		},
		{
			name:     "exactly 60 chars",
			payload:  strings.Repeat("a", 60),
			expected: strings.Repeat("a", 60),
		},
		{
			name:     "61 chars - should truncate",
			payload:  strings.Repeat("a", 61),
			expected: strings.Repeat("a", 57) + "...",
		},
		{
			name:     "long payload - should truncate",
			payload:  "a3f5b8c91d2e4f7a9b1c3d5e6f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6",
			expected: "a3f5b8c91d2e4f7a9b1c3d5e6f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3...",
		},
		{
			name:     "multi-line payload - first line only",
			payload:  "First line\nSecond line\nThird line",
			expected: "First line",
		},
		{
			name:     "multi-line with long first line",
			payload:  strings.Repeat("x", 70) + "\nSecond line",
			expected: strings.Repeat("x", 57) + "...",
		},
		{
			name:     "payload with leading/trailing whitespace",
			payload:  "  \n  hello world  \n  ",
			expected: "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatPayload(tt.payload)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatProducedBy(t *testing.T) {
	tests := []struct {
		name     string
		role     string
		expected string
	}{
		{
			name:     "empty role",
			role:     "",
			expected: "-",
		},
		{
			name:     "user role",
			role:     "user",
			expected: "user",
		},
		{
			name:     "agent role",
			role:     "git-agent",
			expected: "git-agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatProducedBy(tt.role)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatTable(t *testing.T) {
	t.Run("empty artefacts", func(t *testing.T) {
		var buf bytes.Buffer
		count := FormatTable(&buf, []*blackboard.Artefact{}, "test-instance")

		output := buf.String()
		assert.Contains(t, output, "No artefacts found for instance 'test-instance'")
		assert.Equal(t, 0, count)
	})

	t.Run("single artefact", func(t *testing.T) {
		artefacts := []*blackboard.Artefact{
			{
				ID:              "abc-123",
				Type:            "GoalDefined",
				Payload:         "hello.txt",
				ProducedByRole:  "user",
			},
		}

		var buf bytes.Buffer
		count := FormatTable(&buf, artefacts, "test-instance")

		output := buf.String()
		assert.Contains(t, output, "Artefacts for instance 'test-instance'")
		assert.Contains(t, output, "abc-123")
		assert.Contains(t, output, "GoalDefined")
		assert.Contains(t, output, "user")
		assert.Contains(t, output, "hello.txt")
		assert.Contains(t, output, "1 artefact found")
		assert.Equal(t, 1, count)
	})

	t.Run("multiple artefacts", func(t *testing.T) {
		artefacts := []*blackboard.Artefact{
			{
				ID:              "abc-123",
				Type:            "GoalDefined",
				Payload:         "hello.txt",
				ProducedByRole:  "user",
			},
			{
				ID:              "def-456",
				Type:            "CodeCommit",
				Payload:         "a3f5b8c91d2e4f7a9b1c3d5e6f8a9b0c1d2e3f4a5b6c7d8e9f0a",
				ProducedByRole:  "git-agent",
			},
		}

		var buf bytes.Buffer
		count := FormatTable(&buf, artefacts, "test-instance")

		output := buf.String()
		assert.Contains(t, output, "abc-123")
		assert.Contains(t, output, "def-456")
		assert.Contains(t, output, "2 artefacts found")
		assert.Equal(t, 2, count)
	})

	t.Run("artefact with empty fields", func(t *testing.T) {
		artefacts := []*blackboard.Artefact{
			{
				ID:              "abc-123",
				Type:            "Unknown",
				Payload:         "",
				ProducedByRole:  "",
			},
		}

		var buf bytes.Buffer
		FormatTable(&buf, artefacts, "test-instance")

		output := buf.String()
		// Should contain "-" for empty fields
		assert.Contains(t, output, "-")
	})

	t.Run("artefact with long payload", func(t *testing.T) {
		artefacts := []*blackboard.Artefact{
			{
				ID:              "abc-123",
				Type:            "CodeCommit",
				Payload:         strings.Repeat("x", 100),
				ProducedByRole:  "agent",
			},
		}

		var buf bytes.Buffer
		FormatTable(&buf, artefacts, "test-instance")

		output := buf.String()
		// Payload should be truncated with "..."
		assert.Contains(t, output, "...")
		// Should not contain the full 100 char payload
		assert.NotContains(t, output, strings.Repeat("x", 100))
	})
}

func TestFormatJSONArray(t *testing.T) {
	t.Run("empty artefacts", func(t *testing.T) {
		var buf bytes.Buffer
		err := FormatJSONArray(&buf, []*blackboard.Artefact{})

		require.NoError(t, err)

		// Should be valid JSON array
		var result []blackboard.Artefact
		err = json.Unmarshal(buf.Bytes(), &result)
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("single artefact", func(t *testing.T) {
		artefacts := []*blackboard.Artefact{
			{
				ID:              "abc-123",
				LogicalID:       "logical-1",
				Version:         1,
				StructuralType:  blackboard.StructuralTypeStandard,
				Type:            "GoalDefined",
				Payload:         "hello.txt",
				SourceArtefacts: []string{},
				ProducedByRole:  "user",
			},
		}

		var buf bytes.Buffer
		err := FormatJSONArray(&buf, artefacts)

		require.NoError(t, err)

		// Should be valid JSON array
		var result []*blackboard.Artefact
		err = json.Unmarshal(buf.Bytes(), &result)
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "abc-123", result[0].ID)
		assert.Equal(t, "GoalDefined", result[0].Type)
		assert.Equal(t, "hello.txt", result[0].Payload)
	})

	t.Run("multiple artefacts with full data", func(t *testing.T) {
		artefacts := []*blackboard.Artefact{
			{
				ID:              "abc-123",
				LogicalID:       "logical-1",
				Version:         1,
				StructuralType:  blackboard.StructuralTypeStandard,
				Type:            "GoalDefined",
				Payload:         "hello.txt",
				SourceArtefacts: []string{},
				ProducedByRole:  "user",
			},
			{
				ID:              "def-456",
				LogicalID:       "logical-2",
				Version:         1,
				StructuralType:  blackboard.StructuralTypeStandard,
				Type:            "CodeCommit",
				Payload:         "a3f5b8c91d2e4f7a9b1c3d5e6f8a9b0c1d2e3f4a5b6c7d8e9f0a",
				SourceArtefacts: []string{"abc-123"},
				ProducedByRole:  "git-agent",
			},
		}

		var buf bytes.Buffer
		err := FormatJSONArray(&buf, artefacts)

		require.NoError(t, err)

		// Should be valid JSON array
		var result []*blackboard.Artefact
		err = json.Unmarshal(buf.Bytes(), &result)
		require.NoError(t, err)
		assert.Len(t, result, 2)

		// Verify all fields are preserved
		assert.Equal(t, "abc-123", result[0].ID)
		assert.Equal(t, "logical-1", result[0].LogicalID)
		assert.Equal(t, 1, result[0].Version)
		assert.Equal(t, blackboard.StructuralTypeStandard, result[0].StructuralType)

		assert.Equal(t, "def-456", result[1].ID)
		assert.Equal(t, []string{"abc-123"}, result[1].SourceArtefacts)
	})

	t.Run("preserves multi-line payloads", func(t *testing.T) {
		artefacts := []*blackboard.Artefact{
			{
				ID:              "abc-123",
				LogicalID:       "logical-1",
				Version:         1,
				StructuralType:  blackboard.StructuralTypeStandard,
				Type:            "Config",
				Payload:         "line1\nline2\nline3",
				SourceArtefacts: []string{},
				ProducedByRole:  "user",
			},
		}

		var buf bytes.Buffer
		err := FormatJSONArray(&buf, artefacts)

		require.NoError(t, err)

		var result []*blackboard.Artefact
		err = json.Unmarshal(buf.Bytes(), &result)
		require.NoError(t, err)

		// Multi-line payload should be preserved
		assert.Equal(t, "line1\nline2\nline3", result[0].Payload)
	})
}

func TestFormatSingleJSON(t *testing.T) {
	t.Run("single artefact", func(t *testing.T) {
		artefact := &blackboard.Artefact{
			ID:              "abc-123",
			LogicalID:       "logical-1",
			Version:         1,
			StructuralType:  blackboard.StructuralTypeStandard,
			Type:            "GoalDefined",
			Payload:         "hello.txt",
			SourceArtefacts: []string{},
			ProducedByRole:  "user",
		}

		var buf bytes.Buffer
		err := FormatSingleJSON(&buf, artefact)

		require.NoError(t, err)

		// Should be valid JSON object
		var result blackboard.Artefact
		err = json.Unmarshal(buf.Bytes(), &result)
		require.NoError(t, err)
		assert.Equal(t, "abc-123", result.ID)
		assert.Equal(t, "GoalDefined", result.Type)
	})

	t.Run("preserves all fields", func(t *testing.T) {
		artefact := &blackboard.Artefact{
			ID:              "def-456",
			LogicalID:       "logical-2",
			Version:         2,
			StructuralType:  blackboard.StructuralTypeReview,
			Type:            "ReviewFeedback",
			Payload:         "Some feedback\nwith multiple lines",
			SourceArtefacts: []string{"abc-123", "xyz-789"},
			ProducedByRole:  "reviewer-agent",
		}

		var buf bytes.Buffer
		err := FormatSingleJSON(&buf, artefact)

		require.NoError(t, err)

		var result blackboard.Artefact
		err = json.Unmarshal(buf.Bytes(), &result)
		require.NoError(t, err)

		assert.Equal(t, "def-456", result.ID)
		assert.Equal(t, "logical-2", result.LogicalID)
		assert.Equal(t, 2, result.Version)
		assert.Equal(t, blackboard.StructuralTypeReview, result.StructuralType)
		assert.Equal(t, "ReviewFeedback", result.Type)
		assert.Equal(t, "Some feedback\nwith multiple lines", result.Payload)
		assert.Equal(t, []string{"abc-123", "xyz-789"}, result.SourceArtefacts)
		assert.Equal(t, "reviewer-agent", result.ProducedByRole)
	})

	t.Run("pretty printed with indentation", func(t *testing.T) {
		artefact := &blackboard.Artefact{
			ID:              "abc-123",
			LogicalID:       "logical-1",
			Version:         1,
			StructuralType:  blackboard.StructuralTypeStandard,
			Type:            "Test",
			Payload:         "test",
			SourceArtefacts: []string{},
			ProducedByRole:  "user",
		}

		var buf bytes.Buffer
		err := FormatSingleJSON(&buf, artefact)

		require.NoError(t, err)

		output := buf.String()
		// Check for pretty-printed format (should have newlines and indentation)
		assert.Contains(t, output, "\n")
		assert.Contains(t, output, "  ") // indentation
	})
}
