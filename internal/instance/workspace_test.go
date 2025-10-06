package instance

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCanonicalWorkspacePath(t *testing.T) {
	// This test requires being run in a git repository
	// Create a temporary git repo for testing
	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	err := cmd.Run()
	require.NoError(t, err)

	// Change to the temp directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(originalDir)

	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	// Get canonical path
	path, err := GetCanonicalWorkspacePath()
	require.NoError(t, err)

	// Should be absolute
	assert.True(t, filepath.IsAbs(path))

	// Should contain the tmpDir path (accounting for symlink resolution)
	// On some systems tmpDir might be symlinked
	realTmpDir, err := filepath.EvalSymlinks(tmpDir)
	require.NoError(t, err)
	absRealTmpDir, err := filepath.Abs(realTmpDir)
	require.NoError(t, err)

	assert.Equal(t, absRealTmpDir, path)
}

func TestGetCanonicalWorkspacePath_NotGitRepo(t *testing.T) {
	// Create a non-git directory
	tmpDir := t.TempDir()

	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(originalDir)

	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	// Should fail
	_, err = GetCanonicalWorkspacePath()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get git root")
}
