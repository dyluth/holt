package pup

import (
	"fmt"
	"os/exec"
	"strings"
)

const (
	// workspaceDir is the mounted Git repository path in agent containers
	workspaceDir = "/workspace"
)

// validateCommitExists verifies that a git commit hash exists in the repository.
// This is called after tool execution when the tool returns a CodeCommit artefact.
//
// Uses `git cat-file -e <hash>` which:
//   - Returns exit code 0 if the commit exists
//   - Returns non-zero if the commit doesn't exist or is invalid
//   - Works with both full (40-char) and short commit hashes
//
// Returns nil if the commit exists, error otherwise.
func validateCommitExists(commitHash string) error {
	if commitHash == "" {
		return fmt.Errorf("commit hash is empty")
	}

	// Use git cat-file -e to check if commit exists
	// -e flag: exit with zero status if object exists
	cmd := exec.Command("git", "cat-file", "-e", commitHash)
	cmd.Dir = workspaceDir

	// Run command and check exit code
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Command failed - commit doesn't exist or git error
		stderr := strings.TrimSpace(string(output))
		if stderr == "" {
			stderr = err.Error()
		}
		return fmt.Errorf("git commit validation failed: %s", stderr)
	}

	return nil
}
