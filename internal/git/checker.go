package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Checker provides Git repository validation functionality
type Checker struct{}

// NewChecker creates a new Git checker
func NewChecker() *Checker {
	return &Checker{}
}

// IsGitRepository checks if the current directory is within a Git repository
func (c *Checker) IsGitRepository() (bool, error) {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	err := cmd.Run()
	if err != nil {
		// Check if error is because git command not found
		if _, ok := err.(*exec.Error); ok {
			return false, fmt.Errorf("git not found in PATH\nSett requires Git to be installed.\nInstall Git: https://git-scm.com/downloads")
		}
		// Not in a Git repository
		return false, nil
	}
	return true, nil
}

// GetGitRoot returns the absolute path to the Git repository root
func (c *Checker) GetGitRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get Git root: %w", err)
	}

	gitRoot := strings.TrimSpace(string(output))
	return gitRoot, nil
}

// IsGitRoot checks if the current directory is the Git repository root
func (c *Checker) IsGitRoot() (bool, string, error) {
	// Get current directory
	currentDir, err := os.Getwd()
	if err != nil {
		return false, "", fmt.Errorf("failed to get current directory: %w", err)
	}

	// Get Git root
	gitRoot, err := c.GetGitRoot()
	if err != nil {
		return false, "", err
	}

	// Clean both paths and compare
	currentDirClean := filepath.Clean(currentDir)
	gitRootClean := filepath.Clean(gitRoot)

	isRoot := currentDirClean == gitRootClean

	return isRoot, gitRoot, nil
}

// ValidateGitContext validates that we're in a Git repository at its root
// Returns a user-friendly error if validation fails
func (c *Checker) ValidateGitContext() error {
	// First check if we're in a Git repository
	isRepo, err := c.IsGitRepository()
	if err != nil {
		return err
	}

	if !isRepo {
		return fmt.Errorf("not a Git repository\n\nSett requires initialization from within a Git repository.\n\nRun 'git init' first, then 'sett init'")
	}

	// Check if we're at the Git root
	isRoot, gitRoot, err := c.IsGitRoot()
	if err != nil {
		return err
	}

	if !isRoot {
		currentDir, _ := os.Getwd()
		return fmt.Errorf("must run from Git repository root\n\nGit root: %s\nCurrent directory: %s\n\nPlease cd to the Git root and run 'sett init'", gitRoot, currentDir)
	}

	return nil
}

// IsWorkspaceClean returns true if the Git working directory has no uncommitted changes.
// This includes staged, unstaged, and untracked files.
func (c *Checker) IsWorkspaceClean() (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check Git status: %w", err)
	}
	return len(strings.TrimSpace(string(output))) == 0, nil
}

// GetDirtyFiles returns a formatted list of uncommitted changes for error messages.
// Returns empty string if workspace is clean.
func (c *Checker) GetDirtyFiles() (string, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to check Git status: %w", err)
	}

	porcelain := strings.TrimSpace(string(output))
	if porcelain == "" {
		return "", nil
	}

	// Parse porcelain output into categorized lists
	var modified, untracked []string
	for _, line := range strings.Split(porcelain, "\n") {
		if len(line) < 3 {
			continue
		}
		status := line[:2]
		file := strings.TrimSpace(line[2:])

		if strings.HasPrefix(status, "??") {
			untracked = append(untracked, file)
		} else {
			modified = append(modified, file)
		}
	}

	// Format output
	var parts []string
	if len(modified) > 0 {
		parts = append(parts, "Uncommitted changes:")
		for _, file := range modified {
			parts = append(parts, fmt.Sprintf(" M %s", file))
		}
	}
	if len(untracked) > 0 {
		if len(parts) > 0 {
			parts = append(parts, "")
		}
		parts = append(parts, "Untracked files:")
		for _, file := range untracked {
			parts = append(parts, fmt.Sprintf("?? %s", file))
		}
	}

	return strings.Join(parts, "\n"), nil
}
