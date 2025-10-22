package blackboard

import (
	"testing"

	"github.com/google/uuid"
)

// TestThreadScore tests conversion of version to ZSET score
func TestThreadScore(t *testing.T) {
	testCases := []struct {
		name     string
		version  int
		expected float64
	}{
		{"version 1", 1, 1.0},
		{"version 2", 2, 2.0},
		{"version 100", 100, 100.0},
		{"version 1000", 1000, 1000.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			score := ThreadScore(tc.version)
			if score != tc.expected {
				t.Errorf("ThreadScore(%d) = %f, expected %f", tc.version, score, tc.expected)
			}
		})
	}
}

// TestVersionFromScore tests conversion of ZSET score back to version
func TestVersionFromScore(t *testing.T) {
	testCases := []struct {
		name     string
		score    float64
		expected int
	}{
		{"score 1.0", 1.0, 1},
		{"score 2.0", 2.0, 2},
		{"score 100.0", 100.0, 100},
		{"score 1000.0", 1000.0, 1000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			version := VersionFromScore(tc.score)
			if version != tc.expected {
				t.Errorf("VersionFromScore(%f) = %d, expected %d", tc.score, version, tc.expected)
			}
		})
	}
}

// TestThreadScoreRoundTrip tests that version -> score -> version maintains fidelity
func TestThreadScoreRoundTrip(t *testing.T) {
	versions := []int{1, 2, 5, 10, 50, 100, 500, 1000}

	for _, original := range versions {
		score := ThreadScore(original)
		result := VersionFromScore(score)

		if result != original {
			t.Errorf("round-trip failed for version %d: got %d", original, result)
		}
	}
}

// TestThreadVersion_Struct tests the ThreadVersion struct
func TestThreadVersion_Struct(t *testing.T) {
	artefactID := uuid.New().String()
	version := 5

	tv := ThreadVersion{
		ArtefactID: artefactID,
		Version:    version,
	}

	if tv.ArtefactID != artefactID {
		t.Errorf("ThreadVersion.ArtefactID = %q, expected %q", tv.ArtefactID, artefactID)
	}
	if tv.Version != version {
		t.Errorf("ThreadVersion.Version = %d, expected %d", tv.Version, version)
	}
}

// TestThreadKey_Integration tests thread key generation with thread utilities
func TestThreadKey_Integration(t *testing.T) {
	instanceName := "default-1"
	logicalID := uuid.New().String()

	// Generate thread key
	key := ThreadKey(instanceName, logicalID)

	// Verify format
	expectedPrefix := "holt:default-1:thread:"
	if len(key) < len(expectedPrefix) || key[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("thread key should start with %q, got %q", expectedPrefix, key)
	}

	// Create thread version for this thread
	artefactID := uuid.New().String()
	version := 1
	score := ThreadScore(version)

	tv := ThreadVersion{
		ArtefactID: artefactID,
		Version:    version,
	}

	// Verify that we can reconstruct the version from score
	reconstructedVersion := VersionFromScore(score)
	if reconstructedVersion != tv.Version {
		t.Errorf("reconstructed version = %d, expected %d", reconstructedVersion, tv.Version)
	}
}
