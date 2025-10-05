package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestIsGitRepository(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func() (string, func())
		wantIsGit bool
		wantErr   bool
	}{
		{
			name: "valid git repository",
			setupFunc: func() (string, func()) {
				tmpDir, err := os.MkdirTemp("", "git-test-*")
				if err != nil {
					t.Fatal(err)
				}
				cmd := exec.Command("git", "init")
				cmd.Dir = tmpDir
				if err := cmd.Run(); err != nil {
					os.RemoveAll(tmpDir)
					t.Fatal(err)
				}
				return tmpDir, func() { os.RemoveAll(tmpDir) }
			},
			wantIsGit: true,
			wantErr:   false,
		},
		{
			name: "not a git repository",
			setupFunc: func() (string, func()) {
				tmpDir, err := os.MkdirTemp("", "not-git-test-*")
				if err != nil {
					t.Fatal(err)
				}
				return tmpDir, func() { os.RemoveAll(tmpDir) }
			},
			wantIsGit: false,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, cleanup := tt.setupFunc()
			defer cleanup()

			// Change to test directory
			originalDir, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			defer os.Chdir(originalDir)

			if err := os.Chdir(dir); err != nil {
				t.Fatal(err)
			}

			checker := NewChecker()
			isGit, err := checker.IsGitRepository()

			if (err != nil) != tt.wantErr {
				t.Errorf("IsGitRepository() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if isGit != tt.wantIsGit {
				t.Errorf("IsGitRepository() = %v, want %v", isGit, tt.wantIsGit)
			}
		})
	}
}

func TestGetGitRoot(t *testing.T) {
	// Create a git repository with subdirectories
	tmpDir, err := os.MkdirTemp("", "git-root-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	// Create subdirectory
	subDir := filepath.Join(tmpDir, "subdir", "nested")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalDir)

	tests := []struct {
		name    string
		dir     string
		wantErr bool
	}{
		{
			name:    "from git root",
			dir:     tmpDir,
			wantErr: false,
		},
		{
			name:    "from subdirectory",
			dir:     subDir,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.Chdir(tt.dir); err != nil {
				t.Fatal(err)
			}

			checker := NewChecker()
			gitRoot, err := checker.GetGitRoot()

			if (err != nil) != tt.wantErr {
				t.Errorf("GetGitRoot() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Resolve symlinks for comparison (handles macOS /var -> /private/var)
				expectedRoot, err := filepath.EvalSymlinks(tmpDir)
				if err != nil {
					expectedRoot = filepath.Clean(tmpDir)
				}
				actualRoot, err := filepath.EvalSymlinks(gitRoot)
				if err != nil {
					actualRoot = filepath.Clean(gitRoot)
				}
				if actualRoot != expectedRoot {
					t.Errorf("GetGitRoot() = %v, want %v", actualRoot, expectedRoot)
				}
			}
		})
	}
}

func TestIsGitRoot(t *testing.T) {
	// Create a git repository with subdirectories
	tmpDir, err := os.MkdirTemp("", "git-is-root-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	// Create subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalDir)

	tests := []struct {
		name       string
		dir        string
		wantIsRoot bool
		wantErr    bool
	}{
		{
			name:       "at git root",
			dir:        tmpDir,
			wantIsRoot: true,
			wantErr:    false,
		},
		{
			name:       "in subdirectory",
			dir:        subDir,
			wantIsRoot: false,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.Chdir(tt.dir); err != nil {
				t.Fatal(err)
			}

			checker := NewChecker()
			isRoot, gitRoot, err := checker.IsGitRoot()

			if (err != nil) != tt.wantErr {
				t.Errorf("IsGitRoot() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if isRoot != tt.wantIsRoot {
				t.Errorf("IsGitRoot() isRoot = %v, want %v", isRoot, tt.wantIsRoot)
			}

			if !tt.wantErr {
				// Resolve symlinks for comparison (handles macOS /var -> /private/var)
				expectedRoot, err := filepath.EvalSymlinks(tmpDir)
				if err != nil {
					expectedRoot = filepath.Clean(tmpDir)
				}
				actualRoot, err := filepath.EvalSymlinks(gitRoot)
				if err != nil {
					actualRoot = filepath.Clean(gitRoot)
				}
				if actualRoot != expectedRoot {
					t.Errorf("IsGitRoot() gitRoot = %v, want %v", actualRoot, expectedRoot)
				}
			}
		})
	}
}

func TestValidateGitContext(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func() (string, func())
		wantErr   bool
		errMsg    string
	}{
		{
			name: "valid: at git root",
			setupFunc: func() (string, func()) {
				tmpDir, err := os.MkdirTemp("", "git-validate-test-*")
				if err != nil {
					t.Fatal(err)
				}
				cmd := exec.Command("git", "init")
				cmd.Dir = tmpDir
				if err := cmd.Run(); err != nil {
					os.RemoveAll(tmpDir)
					t.Fatal(err)
				}
				return tmpDir, func() { os.RemoveAll(tmpDir) }
			},
			wantErr: false,
		},
		{
			name: "invalid: not a git repository",
			setupFunc: func() (string, func()) {
				tmpDir, err := os.MkdirTemp("", "not-git-validate-test-*")
				if err != nil {
					t.Fatal(err)
				}
				return tmpDir, func() { os.RemoveAll(tmpDir) }
			},
			wantErr: true,
			errMsg:  "not a Git repository",
		},
		{
			name: "invalid: in subdirectory",
			setupFunc: func() (string, func()) {
				tmpDir, err := os.MkdirTemp("", "git-subdir-validate-test-*")
				if err != nil {
					t.Fatal(err)
				}
				cmd := exec.Command("git", "init")
				cmd.Dir = tmpDir
				if err := cmd.Run(); err != nil {
					os.RemoveAll(tmpDir)
					t.Fatal(err)
				}
				subDir := filepath.Join(tmpDir, "subdir")
				if err := os.MkdirAll(subDir, 0755); err != nil {
					os.RemoveAll(tmpDir)
					t.Fatal(err)
				}
				return subDir, func() { os.RemoveAll(tmpDir) }
			},
			wantErr: true,
			errMsg:  "must run from Git repository root",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, cleanup := tt.setupFunc()
			defer cleanup()

			originalDir, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			defer os.Chdir(originalDir)

			if err := os.Chdir(dir); err != nil {
				t.Fatal(err)
			}

			checker := NewChecker()
			err = checker.ValidateGitContext()

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateGitContext() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateGitContext() error = %v, should contain %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
