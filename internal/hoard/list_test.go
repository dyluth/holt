package hoard

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/dyluth/holt/pkg/blackboard"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListArtefacts(t *testing.T) {
	t.Run("empty blackboard - default format", func(t *testing.T) {
		// Setup miniredis
		mr := miniredis.RunT(t)
		defer mr.Close()

		redisOpts := &redis.Options{Addr: mr.Addr()}
		bbClient, err := blackboard.NewClient(redisOpts, "test-instance")
		require.NoError(t, err)
		defer bbClient.Close()

		ctx := context.Background()

		// List artefacts
		var buf bytes.Buffer
		err = ListArtefacts(ctx, bbClient, "test-instance", OutputFormatDefault, nil, &buf)
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "No artefacts found for instance 'test-instance'")
	})

	t.Run("empty blackboard - JSON format", func(t *testing.T) {
		// Setup miniredis
		mr := miniredis.RunT(t)
		defer mr.Close()

		redisOpts := &redis.Options{Addr: mr.Addr()}
		bbClient, err := blackboard.NewClient(redisOpts, "test-instance")
		require.NoError(t, err)
		defer bbClient.Close()

		ctx := context.Background()

		// List artefacts
		var buf bytes.Buffer
		err = ListArtefacts(ctx, bbClient, "test-instance", OutputFormatJSONL, nil, &buf)
		require.NoError(t, err)

		// JSONL format should be empty for no artefacts
		assert.Empty(t, buf.String())
	})

	t.Run("single artefact - default format", func(t *testing.T) {
		// Setup miniredis
		mr := miniredis.RunT(t)
		defer mr.Close()

		redisOpts := &redis.Options{Addr: mr.Addr()}
		bbClient, err := blackboard.NewClient(redisOpts, "test-instance")
		require.NoError(t, err)
		defer bbClient.Close()

		ctx := context.Background()

		// Create artefact
		artefact := &blackboard.Artefact{
			ID:              "550e8400-e29b-41d4-a716-446655440000",
			LogicalID:       "650e8400-e29b-41d4-a716-446655440000",
			Version:         1,
			StructuralType:  blackboard.StructuralTypeStandard,
			Type:            "GoalDefined",
			ProducedByRole:  "user", // M3.7: GoalDefined created by user via CLI
			Payload:         "test-goal.txt",
		}
		err = bbClient.CreateArtefact(ctx, artefact)
		require.NoError(t, err)

		// List artefacts
		var buf bytes.Buffer
		err = ListArtefacts(ctx, bbClient, "test-instance", OutputFormatDefault, nil, &buf)
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "Artefacts for instance 'test-instance'")
		assert.Contains(t, output, "550e8400") // ID is truncated to 8 chars in table
		assert.Contains(t, output, "Goal")      // "GoalDefined" is shortened to "Goal"
		assert.Contains(t, output, "user")
		assert.Contains(t, output, "test-goal.txt")
		assert.Contains(t, output, "1 artefact found")
	})

	t.Run("multiple artefacts - default format", func(t *testing.T) {
		// Setup miniredis
		mr := miniredis.RunT(t)
		defer mr.Close()

		redisOpts := &redis.Options{Addr: mr.Addr()}
		bbClient, err := blackboard.NewClient(redisOpts, "test-instance")
		require.NoError(t, err)
		defer bbClient.Close()

		ctx := context.Background()

		// Create multiple artefacts
		artefacts := []*blackboard.Artefact{
			{
				ID:              "550e8400-e29b-41d4-a716-446655440001",
				LogicalID:       "650e8400-e29b-41d4-a716-446655440001",
				Version:         1,
				StructuralType:  blackboard.StructuralTypeStandard,
				Type:            "GoalDefined",
				ProducedByRole:  "test-agent",
				Payload:         "test-goal.txt",
				SourceArtefacts: []string{},
			},
			{
				ID:              "550e8400-e29b-41d4-a716-446655440002",
				LogicalID:       "650e8400-e29b-41d4-a716-446655440002",
				Version:         1,
				StructuralType:  blackboard.StructuralTypeStandard,
				Type:            "CodeCommit",
				ProducedByRole:  "test-agent",
				Payload:         "a3f5b8c91d2e4f7a9b1c3d5e6f8a9b0c1d2e3f4a5b6c7d8e9f0a",
				SourceArtefacts: []string{"550e8400-e29b-41d4-a716-446655440001"},
			},
		}

		for _, a := range artefacts {
			err = bbClient.CreateArtefact(ctx, a)
			require.NoError(t, err)
		}

		// List artefacts
		var buf bytes.Buffer
		err = ListArtefacts(ctx, bbClient, "test-instance", OutputFormatDefault, nil, &buf)
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "550e8400") // IDs are truncated to 8 chars
		assert.Contains(t, output, "2 artefacts found")
	})

	t.Run("multiple artefacts - JSON format", func(t *testing.T) {
		// Setup miniredis
		mr := miniredis.RunT(t)
		defer mr.Close()

		redisOpts := &redis.Options{Addr: mr.Addr()}
		bbClient, err := blackboard.NewClient(redisOpts, "test-instance")
		require.NoError(t, err)
		defer bbClient.Close()

		ctx := context.Background()

		// Create multiple artefacts
		artefacts := []*blackboard.Artefact{
			{
				ID:              "550e8400-e29b-41d4-a716-446655440001",
				LogicalID:       "650e8400-e29b-41d4-a716-446655440001",
				Version:         1,
				StructuralType:  blackboard.StructuralTypeStandard,
				Type:            "GoalDefined",
				ProducedByRole:  "test-agent",
				Payload:         "test-goal.txt",
				SourceArtefacts: []string{},
			},
			{
				ID:              "550e8400-e29b-41d4-a716-446655440002",
				LogicalID:       "650e8400-e29b-41d4-a716-446655440002",
				Version:         1,
				StructuralType:  blackboard.StructuralTypeStandard,
				Type:            "CodeCommit",
				ProducedByRole:  "test-agent",
				Payload:         "commit-hash",
				SourceArtefacts: []string{"550e8400-e29b-41d4-a716-446655440001"},
			},
		}

		for _, a := range artefacts {
			err = bbClient.CreateArtefact(ctx, a)
			require.NoError(t, err)
		}

		// List artefacts
		var buf bytes.Buffer
		err = ListArtefacts(ctx, bbClient, "test-instance", OutputFormatJSONL, nil, &buf)
		require.NoError(t, err)

		// Parse JSONL (one JSON object per line)
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		require.Len(t, lines, 2)

		// Parse first line
		var art1 blackboard.Artefact
		err = json.Unmarshal([]byte(lines[0]), &art1)
		require.NoError(t, err)
		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440001", art1.ID)
		assert.Equal(t, "GoalDefined", art1.Type)

		// Parse second line
		var art2 blackboard.Artefact
		err = json.Unmarshal([]byte(lines[1]), &art2)
		require.NoError(t, err)
		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440002", art2.ID)
		assert.Equal(t, "CodeCommit", art2.Type)
	})

	t.Run("artefacts sorted alphabetically by ID", func(t *testing.T) {
		// Setup miniredis
		mr := miniredis.RunT(t)
		defer mr.Close()

		redisOpts := &redis.Options{Addr: mr.Addr()}
		bbClient, err := blackboard.NewClient(redisOpts, "test-instance")
		require.NoError(t, err)
		defer bbClient.Close()

		ctx := context.Background()

		// Create artefacts in non-alphabetical order
		artefacts := []*blackboard.Artefact{
			{
				ID:              "ccccc400-e29b-41d4-a716-446655440000",
				LogicalID:       "650e8400-e29b-41d4-a716-446655440000",
				Version:         1,
				StructuralType:  blackboard.StructuralTypeStandard,
				Type:            "Third",
				ProducedByRole:  "test-agent",
				Payload:         "c",
				SourceArtefacts: []string{},
			},
			{
				ID:              "aaaaa400-e29b-41d4-a716-446655440000",
				LogicalID:       "650e8400-e29b-41d4-a716-446655440000",
				Version:         1,
				StructuralType:  blackboard.StructuralTypeStandard,
				Type:            "First",
				ProducedByRole:  "test-agent",
				Payload:         "a",
				SourceArtefacts: []string{},
			},
			{
				ID:              "bbbbb400-e29b-41d4-a716-446655440000",
				LogicalID:       "650e8400-e29b-41d4-a716-446655440000",
				Version:         1,
				StructuralType:  blackboard.StructuralTypeStandard,
				Type:            "Second",
				ProducedByRole:  "test-agent",
				Payload:         "b",
				SourceArtefacts: []string{},
			},
		}

		for _, a := range artefacts {
			err = bbClient.CreateArtefact(ctx, a)
			require.NoError(t, err)
		}

		// List artefacts in JSON format for easy verification
		var buf bytes.Buffer
		err = ListArtefacts(ctx, bbClient, "test-instance", OutputFormatJSONL, nil, &buf)
		require.NoError(t, err)

		// Parse JSONL
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		require.Len(t, lines, 3)

		// Verify chronological order (by CreatedAtMs, not alphabetical by ID)
		// Parse all lines to check IDs exist
		var ids []string
		for _, line := range lines {
			var art blackboard.Artefact
			err = json.Unmarshal([]byte(line), &art)
			require.NoError(t, err)
			ids = append(ids, art.ID)
		}
		assert.Contains(t, ids, "aaaaa400-e29b-41d4-a716-446655440000")
		assert.Contains(t, ids, "bbbbb400-e29b-41d4-a716-446655440000")
		assert.Contains(t, ids, "ccccc400-e29b-41d4-a716-446655440000")
	})

	t.Run("invalid output format", func(t *testing.T) {
		// Setup miniredis
		mr := miniredis.RunT(t)
		defer mr.Close()

		redisOpts := &redis.Options{Addr: mr.Addr()}
		bbClient, err := blackboard.NewClient(redisOpts, "test-instance")
		require.NoError(t, err)
		defer bbClient.Close()

		ctx := context.Background()

		// Try with invalid format
		var buf bytes.Buffer
		err = ListArtefacts(ctx, bbClient, "test-instance", OutputFormat("invalid"), nil, &buf)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown output format")
	})

	t.Run("skips malformed artefacts", func(t *testing.T) {
		// Setup miniredis
		mr := miniredis.RunT(t)
		defer mr.Close()

		redisOpts := &redis.Options{Addr: mr.Addr()}
		bbClient, err := blackboard.NewClient(redisOpts, "test-instance")
		require.NoError(t, err)
		defer bbClient.Close()

		ctx := context.Background()

		// Create a valid artefact
		validArtefact := &blackboard.Artefact{
			ID:              "550e8400-e29b-41d4-a716-446655440000",
			LogicalID:       "650e8400-e29b-41d4-a716-446655440000",
			Version:         1,
			StructuralType:  blackboard.StructuralTypeStandard,
			Type:            "ValidType",
			ProducedByRole:  "test-agent",
			Payload:         "valid",
			SourceArtefacts: []string{},
		}
		err = bbClient.CreateArtefact(ctx, validArtefact)
		require.NoError(t, err)

		// Manually create a malformed artefact in Redis (missing required fields)
		malformedKey := "holt:test-instance:artefact:malformed-id"
		bbClient.RedisClient().HSet(ctx, malformedKey, "id", "malformed-id")

		// List artefacts - should skip malformed one
		var buf bytes.Buffer
		err = ListArtefacts(ctx, bbClient, "test-instance", OutputFormatJSONL, nil, &buf)
		require.NoError(t, err)

		// Parse JSONL - should only have the valid artefact
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		assert.Len(t, lines, 1)

		var result blackboard.Artefact
		err = json.Unmarshal([]byte(lines[0]), &result)
		require.NoError(t, err)
		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", result.ID)
	})

	t.Run("artefact with long multi-line payload", func(t *testing.T) {
		// Setup miniredis
		mr := miniredis.RunT(t)
		defer mr.Close()

		redisOpts := &redis.Options{Addr: mr.Addr()}
		bbClient, err := blackboard.NewClient(redisOpts, "test-instance")
		require.NoError(t, err)
		defer bbClient.Close()

		ctx := context.Background()

		// Create artefact with long multi-line payload
		longPayload := strings.Repeat("x", 100) + "\nSecond line\nThird line"
		artefact := &blackboard.Artefact{
			ID:              "550e8400-e29b-41d4-a716-446655440000",
			LogicalID:       "650e8400-e29b-41d4-a716-446655440000",
			Version:         1,
			StructuralType:  blackboard.StructuralTypeStandard,
			Type:            "LongPayload",
			ProducedByRole:  "test-agent",
			Payload:         longPayload,
			SourceArtefacts: []string{},
		}
		err = bbClient.CreateArtefact(ctx, artefact)
		require.NoError(t, err)

		// List in default format - payload should be truncated
		var buf bytes.Buffer
		err = ListArtefacts(ctx, bbClient, "test-instance", OutputFormatDefault, nil, &buf)
		require.NoError(t, err)

		output := buf.String()
		// Should contain truncation indicator
		assert.Contains(t, output, "...")
		// Should not contain "Second line"
		assert.NotContains(t, output, "Second line")

		// List in JSON format - payload should be preserved
		buf.Reset()
		err = ListArtefacts(ctx, bbClient, "test-instance", OutputFormatJSONL, nil, &buf)
		require.NoError(t, err)

		// Parse JSONL
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		require.Len(t, lines, 1)

		var result blackboard.Artefact
		err = json.Unmarshal([]byte(lines[0]), &result)
		require.NoError(t, err)
		// Full payload should be preserved in JSONL
		assert.Equal(t, longPayload, result.Payload)
	})
}
