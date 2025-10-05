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
