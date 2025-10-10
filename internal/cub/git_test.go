package cub

import (
	"testing"
)

// TestValidateCommitExists_EmptyHash verifies empty hash returns error
func TestValidateCommitExists_EmptyHash(t *testing.T) {
	err := validateCommitExists("")
	if err == nil {
		t.Error("Expected error for empty commit hash, got nil")
	}

	if err.Error() != "commit hash is empty" {
		t.Errorf("Expected 'commit hash is empty', got %q", err.Error())
	}
}

// TestValidateCommitExists_InvalidHash verifies invalid hash returns error
func TestValidateCommitExists_InvalidHash(t *testing.T) {
	err := validateCommitExists("invalid-hash-123")
	if err == nil {
		t.Error("Expected error for invalid commit hash, got nil")
	}

	// Error message should mention git validation failure
	if !contains(err.Error(), "git commit validation failed") {
		t.Errorf("Expected git validation error, got %q", err.Error())
	}
}

// Note: Testing with valid commit hashes requires a real git repository
// which is difficult in unit tests. Integration tests will cover this.
