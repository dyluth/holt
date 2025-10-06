package instance

import (
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
)

func TestDetermineStatus_AllRunning(t *testing.T) {
	containers := []types.Container{
		{State: "running"},
		{State: "running"},
		{State: "running"},
	}

	status := DetermineStatus(containers)
	assert.Equal(t, StatusRunning, status)
}

func TestDetermineStatus_AllStopped(t *testing.T) {
	containers := []types.Container{
		{State: "exited"},
		{State: "exited"},
	}

	status := DetermineStatus(containers)
	assert.Equal(t, StatusStopped, status)
}

func TestDetermineStatus_Degraded(t *testing.T) {
	containers := []types.Container{
		{State: "running"},
		{State: "exited"},
	}

	status := DetermineStatus(containers)
	assert.Equal(t, StatusDegraded, status)
}

func TestDetermineStatus_Empty(t *testing.T) {
	containers := []types.Container{}

	status := DetermineStatus(containers)
	assert.Equal(t, StatusStopped, status)
}

func TestDetermineStatus_SingleRunning(t *testing.T) {
	containers := []types.Container{
		{State: "running"},
	}

	status := DetermineStatus(containers)
	assert.Equal(t, StatusRunning, status)
}

func TestDetermineStatus_SingleStopped(t *testing.T) {
	containers := []types.Container{
		{State: "exited"},
	}

	status := DetermineStatus(containers)
	assert.Equal(t, StatusStopped, status)
}

func TestDetermineStatus_MostlyRunning(t *testing.T) {
	containers := []types.Container{
		{State: "running"},
		{State: "running"},
		{State: "running"},
		{State: "exited"},
	}

	status := DetermineStatus(containers)
	assert.Equal(t, StatusDegraded, status)
}

func TestDetermineStatus_MostlyStopped(t *testing.T) {
	containers := []types.Container{
		{State: "running"},
		{State: "exited"},
		{State: "exited"},
		{State: "exited"},
	}

	status := DetermineStatus(containers)
	assert.Equal(t, StatusDegraded, status)
}
