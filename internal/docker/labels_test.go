package docker

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestBuildLabels(t *testing.T) {
	runID := "test-run-123"
	instanceName := "prod"
	workspacePath := "/home/user/project"

	labels := BuildLabels(instanceName, runID, workspacePath, "redis")

	assert.Equal(t, "true", labels[LabelProject])
	assert.Equal(t, instanceName, labels[LabelInstanceName])
	assert.Equal(t, runID, labels[LabelInstanceRunID])
	assert.Equal(t, workspacePath, labels[LabelWorkspacePath])
	assert.Equal(t, "redis", labels[LabelComponent])
	assert.Len(t, labels, 5)
}

func TestBuildLabels_NoComponent(t *testing.T) {
	runID := "test-run-456"
	instanceName := "dev"
	workspacePath := "/workspace"

	labels := BuildLabels(instanceName, runID, workspacePath, "")

	assert.Equal(t, "true", labels[LabelProject])
	assert.Equal(t, instanceName, labels[LabelInstanceName])
	assert.Equal(t, runID, labels[LabelInstanceRunID])
	assert.Equal(t, workspacePath, labels[LabelWorkspacePath])
	assert.NotContains(t, labels, LabelComponent)
	assert.Len(t, labels, 4)
}

func TestGenerateRunID(t *testing.T) {
	runID1 := GenerateRunID()
	runID2 := GenerateRunID()

	// Verify they are valid UUIDs
	_, err1 := uuid.Parse(runID1)
	assert.NoError(t, err1)

	_, err2 := uuid.Parse(runID2)
	assert.NoError(t, err2)

	// Verify they are different
	assert.NotEqual(t, runID1, runID2)
}

func TestNetworkName(t *testing.T) {
	testCases := []struct {
		instanceName string
		expected     string
	}{
		{"prod", "holt-network-prod"},
		{"dev", "holt-network-dev"},
		{"staging-1", "holt-network-staging-1"},
	}

	for _, tc := range testCases {
		result := NetworkName(tc.instanceName)
		assert.Equal(t, tc.expected, result)
	}
}

func TestRedisContainerName(t *testing.T) {
	testCases := []struct {
		instanceName string
		expected     string
	}{
		{"prod", "holt-redis-prod"},
		{"dev", "holt-redis-dev"},
		{"default-1", "holt-redis-default-1"},
	}

	for _, tc := range testCases {
		result := RedisContainerName(tc.instanceName)
		assert.Equal(t, tc.expected, result)
	}
}

func TestOrchestratorContainerName(t *testing.T) {
	testCases := []struct {
		instanceName string
		expected     string
	}{
		{"prod", "holt-orchestrator-prod"},
		{"dev", "holt-orchestrator-dev"},
		{"test-123", "holt-orchestrator-test-123"},
	}

	for _, tc := range testCases {
		result := OrchestratorContainerName(tc.instanceName)
		assert.Equal(t, tc.expected, result)
	}
}
