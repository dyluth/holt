package hoard

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/dyluth/sett/pkg/blackboard"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetArtefact(t *testing.T) {
	t.Run("valid artefact ID", func(t *testing.T) {
		// Setup miniredis
		mr := miniredis.RunT(t)
		defer mr.Close()

		redisOpts := &redis.Options{Addr: mr.Addr()}
		bbClient, err := blackboard.NewClient(redisOpts, "test-instance")
		require.NoError(t, err)
		defer bbClient.Close()

		ctx := context.Background()

		// Create test artefact
		artefact := &blackboard.Artefact{
			ID:              "550e8400-e29b-41d4-a716-446655440000",
			LogicalID:       "650e8400-e29b-41d4-a716-446655440000",
			Version:         1,
			StructuralType:  blackboard.StructuralTypeStandard,
			Type:            "GoalDefined",
			Payload:         "test-goal.txt",
			SourceArtefacts: []string{},
			ProducedByRole:  "user",
		}

		err = bbClient.CreateArtefact(ctx, artefact)
		require.NoError(t, err)

		// Get artefact
		var buf bytes.Buffer
		err = GetArtefact(ctx, bbClient, artefact.ID, &buf)
		require.NoError(t, err)

		// Verify JSON output
		var result blackboard.Artefact
		err = json.Unmarshal(buf.Bytes(), &result)
		require.NoError(t, err)
		assert.Equal(t, artefact.ID, result.ID)
		assert.Equal(t, artefact.Type, result.Type)
		assert.Equal(t, artefact.Payload, result.Payload)
	})

	t.Run("artefact not found", func(t *testing.T) {
		// Setup miniredis
		mr := miniredis.RunT(t)
		defer mr.Close()

		redisOpts := &redis.Options{Addr: mr.Addr()}
		bbClient, err := blackboard.NewClient(redisOpts, "test-instance")
		require.NoError(t, err)
		defer bbClient.Close()

		ctx := context.Background()

		// Try to get non-existent artefact
		var buf bytes.Buffer
		err = GetArtefact(ctx, bbClient, "550e8400-e29b-41d4-a716-446655440000", &buf)

		require.Error(t, err)
		assert.True(t, IsNotFound(err), "error should be ArtefactNotFoundError")

		// Verify error message
		notFoundErr, ok := err.(*ArtefactNotFoundError)
		require.True(t, ok)
		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", notFoundErr.ArtefactID)
		assert.Contains(t, err.Error(), "artefact with ID")
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("invalid artefact ID format", func(t *testing.T) {
		// Setup miniredis
		mr := miniredis.RunT(t)
		defer mr.Close()

		redisOpts := &redis.Options{Addr: mr.Addr()}
		bbClient, err := blackboard.NewClient(redisOpts, "test-instance")
		require.NoError(t, err)
		defer bbClient.Close()

		ctx := context.Background()

		// Try with invalid UUID
		var buf bytes.Buffer
		err = GetArtefact(ctx, bbClient, "not-a-uuid", &buf)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid artefact ID format")
		assert.Contains(t, err.Error(), "must be a valid UUID")

		// IsNotFound should return false for validation errors
		assert.False(t, IsNotFound(err))
	})

	t.Run("empty artefact ID", func(t *testing.T) {
		// Setup miniredis
		mr := miniredis.RunT(t)
		defer mr.Close()

		redisOpts := &redis.Options{Addr: mr.Addr()}
		bbClient, err := blackboard.NewClient(redisOpts, "test-instance")
		require.NoError(t, err)
		defer bbClient.Close()

		ctx := context.Background()

		// Try with empty UUID
		var buf bytes.Buffer
		err = GetArtefact(ctx, bbClient, "", &buf)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid artefact ID format")
	})
}

func TestArtefactNotFoundError(t *testing.T) {
	t.Run("error message", func(t *testing.T) {
		err := &ArtefactNotFoundError{ArtefactID: "test-id-123"}
		assert.Equal(t, "artefact with ID 'test-id-123' not found", err.Error())
	})

	t.Run("IsNotFound with ArtefactNotFoundError", func(t *testing.T) {
		err := &ArtefactNotFoundError{ArtefactID: "test-id"}
		assert.True(t, IsNotFound(err))
	})

	t.Run("IsNotFound with other error", func(t *testing.T) {
		err := assert.AnError
		assert.False(t, IsNotFound(err))
	})

	t.Run("IsNotFound with nil", func(t *testing.T) {
		assert.False(t, IsNotFound(nil))
	})
}
