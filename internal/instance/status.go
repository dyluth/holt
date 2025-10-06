package instance

import (
	"github.com/docker/docker/api/types"
)

// Status represents the health status of a Sett instance
type Status string

const (
	// StatusRunning indicates all containers are running
	StatusRunning Status = "Running"

	// StatusDegraded indicates some containers are stopped or missing
	StatusDegraded Status = "Degraded"

	// StatusStopped indicates all containers exist but are stopped
	StatusStopped Status = "Stopped"
)

// DetermineStatus analyzes a set of containers and determines the overall instance status.
func DetermineStatus(containers []types.Container) Status {
	if len(containers) == 0 {
		return StatusStopped
	}

	runningCount := 0
	for _, c := range containers {
		if c.State == "running" {
			runningCount++
		}
	}

	if runningCount == len(containers) {
		return StatusRunning
	} else if runningCount > 0 {
		return StatusDegraded
	} else {
		return StatusStopped
	}
}

// InstanceInfo holds information about a Sett instance
type InstanceInfo struct {
	Name      string `json:"name"`
	Status    Status `json:"status"`
	Workspace string `json:"workspace"`
	Uptime    string `json:"uptime"`
}
