package docker

import (
	"fmt"

	"github.com/google/uuid"
)

// Label keys used for Sett resources
const (
	LabelProject      = "sett.project"
	LabelInstanceName = "sett.instance.name"
	LabelInstanceRunID = "sett.instance.run_id"
	LabelWorkspacePath = "sett.workspace.path"
	LabelComponent    = "sett.component"
)

// BuildLabels creates the standard label set for all Sett resources.
// All parameters are required except component (which is resource-specific).
func BuildLabels(instanceName, runID, workspacePath, component string) map[string]string {
	labels := map[string]string{
		LabelProject:       "true",
		LabelInstanceName:  instanceName,
		LabelInstanceRunID: runID,
		LabelWorkspacePath: workspacePath,
	}

	if component != "" {
		labels[LabelComponent] = component
	}

	return labels
}

// GenerateRunID creates a new UUID for an instance run.
// Each invocation of `sett up` gets a unique run ID.
func GenerateRunID() string {
	return uuid.New().String()
}

// Resource naming conventions for Sett components

// NetworkName returns the Docker network name for an instance
func NetworkName(instanceName string) string {
	return fmt.Sprintf("sett-network-%s", instanceName)
}

// RedisContainerName returns the Redis container name for an instance
func RedisContainerName(instanceName string) string {
	return fmt.Sprintf("sett-redis-%s", instanceName)
}

// OrchestratorContainerName returns the orchestrator container name for an instance
func OrchestratorContainerName(instanceName string) string {
	return fmt.Sprintf("sett-orchestrator-%s", instanceName)
}
