package commands

import (
	"testing"
	"time"

	"github.com/dyluth/holt/internal/instance"
	"github.com/stretchr/testify/assert"
)

func TestFormatDuration(t *testing.T) {
	testCases := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "seconds only",
			duration: 45 * time.Second,
			expected: "45s",
		},
		{
			name:     "minutes and seconds",
			duration: 2*time.Minute + 30*time.Second,
			expected: "2m 30s",
		},
		{
			name:     "hours and minutes",
			duration: 3*time.Hour + 15*time.Minute,
			expected: "3h 15m",
		},
		{
			name:     "large duration",
			duration: 25*time.Hour + 45*time.Minute,
			expected: "25h 45m",
		},
		{
			name:     "zero",
			duration: 0,
			expected: "0s",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatDuration(tc.duration)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestOutputJSON(t *testing.T) {
	infos := []instance.InstanceInfo{
		{
			Name:      "test1",
			Status:    instance.StatusRunning,
			Workspace: "/home/user/project",
			Uptime:    "2h 15m",
		},
		{
			Name:      "test2",
			Status:    instance.StatusStopped,
			Workspace: "/home/user/other",
			Uptime:    "-",
		},
	}

	// This function prints to stdout, so we just verify it doesn't panic
	assert.NotPanics(t, func() {
		outputJSON(infos)
	})
}

func TestOutputTable(t *testing.T) {
	infos := []instance.InstanceInfo{
		{
			Name:      "test1",
			Status:    instance.StatusRunning,
			Workspace: "/home/user/project",
			Uptime:    "2h 15m",
		},
		{
			Name:      "test2",
			Status:    instance.StatusDegraded,
			Workspace: "/very/long/workspace/path/that/exceeds/thirty/characters/for/testing",
			Uptime:    "-",
		},
	}

	// This function prints to stdout, so we just verify it doesn't panic
	assert.NotPanics(t, func() {
		outputTable(infos)
	})
}
