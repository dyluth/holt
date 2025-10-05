package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInitCommand(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		setupFunc func() (string, func())
		wantErr   bool
		errMsg    string
	}{
		{
			name: "successful init in git repo",
			args: []string{"init"},
			setupFunc: func() (string, func()) {
				tmpDir, err := os.MkdirTemp("", "init-cmd-test-*")
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
			name: "fails when not in git repo",
			args: []string{"init"},
			setupFunc: func() (string, func()) {
				tmpDir, err := os.MkdirTemp("", "init-nogit-test-*")
				if err != nil {
					t.Fatal(err)
				}
				return tmpDir, func() { os.RemoveAll(tmpDir) }
			},
			wantErr: true,
			errMsg:  "not a Git repository",
		},
		{
			name: "fails when not at git root",
			args: []string{"init"},
			setupFunc: func() (string, func()) {
				tmpDir, err := os.MkdirTemp("", "init-subdir-test-*")
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
		{
			name: "fails when already initialized",
			args: []string{"init"},
			setupFunc: func() (string, func()) {
				tmpDir, err := os.MkdirTemp("", "init-existing-test-*")
				if err != nil {
					t.Fatal(err)
				}
				cmd := exec.Command("git", "init")
				cmd.Dir = tmpDir
				if err := cmd.Run(); err != nil {
					os.RemoveAll(tmpDir)
					t.Fatal(err)
				}
				// Create existing sett.yml
				settYml := filepath.Join(tmpDir, "sett.yml")
				if err := os.WriteFile(settYml, []byte("version: '1.0'"), 0644); err != nil {
					os.RemoveAll(tmpDir)
					t.Fatal(err)
				}
				return tmpDir, func() { os.RemoveAll(tmpDir) }
			},
			wantErr: true,
			errMsg:  "project already initialized",
		},
		{
			name: "force flag allows reinitialization",
			args: []string{"init", "--force"},
			setupFunc: func() (string, func()) {
				tmpDir, err := os.MkdirTemp("", "init-force-test-*")
				if err != nil {
					t.Fatal(err)
				}
				cmd := exec.Command("git", "init")
				cmd.Dir = tmpDir
				if err := cmd.Run(); err != nil {
					os.RemoveAll(tmpDir)
					t.Fatal(err)
				}
				// Create existing files
				settYml := filepath.Join(tmpDir, "sett.yml")
				if err := os.WriteFile(settYml, []byte("old content"), 0644); err != nil {
					os.RemoveAll(tmpDir)
					t.Fatal(err)
				}
				agentsDir := filepath.Join(tmpDir, "agents", "old-agent")
				if err := os.MkdirAll(agentsDir, 0755); err != nil {
					os.RemoveAll(tmpDir)
					t.Fatal(err)
				}
				return tmpDir, func() { os.RemoveAll(tmpDir) }
			},
			wantErr: false,
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

			// Reset root command for clean test
			rootCmd.SetArgs(tt.args)

			// Capture output
			err = rootCmd.Execute()

			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("Execute() error = %v, should contain %v", err.Error(), tt.errMsg)
				}
			}

			if !tt.wantErr {
				// Verify files were created
				expectedFiles := []string{
					"sett.yml",
					"agents/example-agent/Dockerfile",
					"agents/example-agent/run.sh",
					"agents/example-agent/README.md",
				}

				for _, file := range expectedFiles {
					fullPath := filepath.Join(dir, file)
					if _, err := os.Stat(fullPath); err != nil {
						t.Errorf("Expected file %s to exist, but got error: %v", file, err)
					}
				}

				// Verify run.sh is executable
				runShPath := filepath.Join(dir, "agents/example-agent/run.sh")
				info, err := os.Stat(runShPath)
				if err != nil {
					t.Errorf("Failed to stat run.sh: %v", err)
				} else {
					if info.Mode()&0111 == 0 {
						t.Errorf("run.sh should be executable, but mode is %v", info.Mode())
					}
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
