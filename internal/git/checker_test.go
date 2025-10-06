package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestIsWorkspaceClean(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func() (string, func())
		wantClean bool
		wantErr   bool
	}{
		{
			name: "clean workspace with committed files",
			setupFunc: func() (string, func()) {
				tmpDir, err := os.MkdirTemp("", "git-clean-test-*")
				if err != nil {
					t.Fatal(err)
				}

				// Initialize git repo
				cmd := exec.Command("git", "init")
				cmd.Dir = tmpDir
				if err := cmd.Run(); err != nil {
					os.RemoveAll(tmpDir)
					t.Fatal(err)
				}

				// Configure git user
				exec.Command("git", "-C", tmpDir, "config", "user.email", "test@example.com").Run()
				exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

				// Create and commit a file
				testFile := filepath.Join(tmpDir, "test.txt")
				if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
					os.RemoveAll(tmpDir)
					t.Fatal(err)
				}

				exec.Command("git", "-C", tmpDir, "add", ".").Run()
				exec.Command("git", "-C", tmpDir, "commit", "-m", "initial commit").Run()

				return tmpDir, func() { os.RemoveAll(tmpDir) }
			},
			wantClean: true,
			wantErr:   false,
		},
		{
			name: "dirty workspace with uncommitted file",
			setupFunc: func() (string, func()) {
				tmpDir, err := os.MkdirTemp("", "git-dirty-test-*")
				if err != nil {
					t.Fatal(err)
				}

				// Initialize git repo
				cmd := exec.Command("git", "init")
				cmd.Dir = tmpDir
				if err := cmd.Run(); err != nil {
					os.RemoveAll(tmpDir)
					t.Fatal(err)
				}

				// Create untracked file
				testFile := filepath.Join(tmpDir, "untracked.txt")
				if err := os.WriteFile(testFile, []byte("untracked"), 0644); err != nil {
					os.RemoveAll(tmpDir)
					t.Fatal(err)
				}

				return tmpDir, func() { os.RemoveAll(tmpDir) }
			},
			wantClean: false,
			wantErr:   false,
		},
		{
			name: "dirty workspace with modified file",
			setupFunc: func() (string, func()) {
				tmpDir, err := os.MkdirTemp("", "git-modified-test-*")
				if err != nil {
					t.Fatal(err)
				}

				// Initialize git repo
				cmd := exec.Command("git", "init")
				cmd.Dir = tmpDir
				if err := cmd.Run(); err != nil {
					os.RemoveAll(tmpDir)
					t.Fatal(err)
				}

				// Configure git user
				exec.Command("git", "-C", tmpDir, "config", "user.email", "test@example.com").Run()
				exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

				// Create and commit a file
				testFile := filepath.Join(tmpDir, "test.txt")
				if err := os.WriteFile(testFile, []byte("original"), 0644); err != nil {
					os.RemoveAll(tmpDir)
					t.Fatal(err)
				}

				exec.Command("git", "-C", tmpDir, "add", ".").Run()
				exec.Command("git", "-C", tmpDir, "commit", "-m", "initial commit").Run()

				// Modify the file
				if err := os.WriteFile(testFile, []byte("modified"), 0644); err != nil {
					os.RemoveAll(tmpDir)
					t.Fatal(err)
				}

				return tmpDir, func() { os.RemoveAll(tmpDir) }
			},
			wantClean: false,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir, cleanup := tt.setupFunc()
			defer cleanup()

			// Change to test directory
			originalDir, _ := os.Getwd()
			defer os.Chdir(originalDir)
			os.Chdir(testDir)

			checker := NewChecker()
			gotClean, err := checker.IsWorkspaceClean()

			if (err != nil) != tt.wantErr {
				t.Errorf("IsWorkspaceClean() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if gotClean != tt.wantClean {
				t.Errorf("IsWorkspaceClean() = %v, want %v", gotClean, tt.wantClean)
			}
		})
	}
}

func TestGetDirtyFiles(t *testing.T) {
	tests := []struct {
		name            string
		setupFunc       func() (string, func())
		wantContains    []string
		wantNotContains []string
		wantErr         bool
	}{
		{
			name: "clean workspace returns empty string",
			setupFunc: func() (string, func()) {
				tmpDir, err := os.MkdirTemp("", "git-getdirty-clean-*")
				if err != nil {
					t.Fatal(err)
				}

				cmd := exec.Command("git", "init")
				cmd.Dir = tmpDir
				if err := cmd.Run(); err != nil {
					os.RemoveAll(tmpDir)
					t.Fatal(err)
				}

				// Configure git and make initial commit
				exec.Command("git", "-C", tmpDir, "config", "user.email", "test@example.com").Run()
				exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

				testFile := filepath.Join(tmpDir, "test.txt")
				os.WriteFile(testFile, []byte("test"), 0644)
				exec.Command("git", "-C", tmpDir, "add", ".").Run()
				exec.Command("git", "-C", tmpDir, "commit", "-m", "init").Run()

				return tmpDir, func() { os.RemoveAll(tmpDir) }
			},
			wantContains:    []string{},
			wantNotContains: []string{"Uncommitted changes", "Untracked files"},
			wantErr:         false,
		},
		{
			name: "dirty workspace shows modified files",
			setupFunc: func() (string, func()) {
				tmpDir, err := os.MkdirTemp("", "git-getdirty-modified-*")
				if err != nil {
					t.Fatal(err)
				}

				cmd := exec.Command("git", "init")
				cmd.Dir = tmpDir
				if err := cmd.Run(); err != nil {
					os.RemoveAll(tmpDir)
					t.Fatal(err)
				}

				// Configure git and make initial commit
				exec.Command("git", "-C", tmpDir, "config", "user.email", "test@example.com").Run()
				exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

				testFile := filepath.Join(tmpDir, "modified.txt")
				os.WriteFile(testFile, []byte("original"), 0644)
				exec.Command("git", "-C", tmpDir, "add", ".").Run()
				exec.Command("git", "-C", tmpDir, "commit", "-m", "init").Run()

				// Modify file
				os.WriteFile(testFile, []byte("changed"), 0644)

				return tmpDir, func() { os.RemoveAll(tmpDir) }
			},
			wantContains:    []string{"Uncommitted changes", "modified.txt"},
			wantNotContains: []string{},
			wantErr:         false,
		},
		{
			name: "dirty workspace shows untracked files",
			setupFunc: func() (string, func()) {
				tmpDir, err := os.MkdirTemp("", "git-getdirty-untracked-*")
				if err != nil {
					t.Fatal(err)
				}

				cmd := exec.Command("git", "init")
				cmd.Dir = tmpDir
				if err := cmd.Run(); err != nil {
					os.RemoveAll(tmpDir)
					t.Fatal(err)
				}

				// Create untracked file
				testFile := filepath.Join(tmpDir, "untracked.txt")
				os.WriteFile(testFile, []byte("new file"), 0644)

				return tmpDir, func() { os.RemoveAll(tmpDir) }
			},
			wantContains:    []string{"Untracked files", "untracked.txt"},
			wantNotContains: []string{},
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir, cleanup := tt.setupFunc()
			defer cleanup()

			// Change to test directory
			originalDir, _ := os.Getwd()
			defer os.Chdir(originalDir)
			os.Chdir(testDir)

			checker := NewChecker()
			got, err := checker.GetDirtyFiles()

			if (err != nil) != tt.wantErr {
				t.Errorf("GetDirtyFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("GetDirtyFiles() = %q, should contain %q", got, want)
				}
			}

			for _, notWant := range tt.wantNotContains {
				if strings.Contains(got, notWant) {
					t.Errorf("GetDirtyFiles() = %q, should not contain %q", got, notWant)
				}
			}
		})
	}
}
