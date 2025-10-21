package scaffold

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckExisting(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func() (string, func())
		wantErr   bool
		errMsg    string
	}{
		{
			name: "no existing files",
			setupFunc: func() (string, func()) {
				tmpDir, err := os.MkdirTemp("", "scaffold-test-*")
				if err != nil {
					t.Fatal(err)
				}
				return tmpDir, func() { os.RemoveAll(tmpDir) }
			},
			wantErr: false,
		},
		{
			name: "existing holt.yml only",
			setupFunc: func() (string, func()) {
				tmpDir, err := os.MkdirTemp("", "scaffold-test-*")
				if err != nil {
					t.Fatal(err)
				}
				holtYml := filepath.Join(tmpDir, "holt.yml")
				if err := os.WriteFile(holtYml, []byte("version: '1.0'"), 0644); err != nil {
					os.RemoveAll(tmpDir)
					t.Fatal(err)
				}
				return tmpDir, func() { os.RemoveAll(tmpDir) }
			},
			wantErr: true,
			errMsg:  "holt.yml",
		},
		{
			name: "existing agents/ directory only",
			setupFunc: func() (string, func()) {
				tmpDir, err := os.MkdirTemp("", "scaffold-test-*")
				if err != nil {
					t.Fatal(err)
				}
				agentsDir := filepath.Join(tmpDir, "agents")
				if err := os.MkdirAll(agentsDir, 0755); err != nil {
					os.RemoveAll(tmpDir)
					t.Fatal(err)
				}
				return tmpDir, func() { os.RemoveAll(tmpDir) }
			},
			wantErr: true,
			errMsg:  "agents/",
		},
		{
			name: "both holt.yml and agents/ exist",
			setupFunc: func() (string, func()) {
				tmpDir, err := os.MkdirTemp("", "scaffold-test-*")
				if err != nil {
					t.Fatal(err)
				}
				holtYml := filepath.Join(tmpDir, "holt.yml")
				if err := os.WriteFile(holtYml, []byte("version: '1.0'"), 0644); err != nil {
					os.RemoveAll(tmpDir)
					t.Fatal(err)
				}
				agentsDir := filepath.Join(tmpDir, "agents")
				if err := os.MkdirAll(agentsDir, 0755); err != nil {
					os.RemoveAll(tmpDir)
					t.Fatal(err)
				}
				return tmpDir, func() { os.RemoveAll(tmpDir) }
			},
			wantErr: true,
			errMsg:  "project already initialized",
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

			err = CheckExisting()

			if (err != nil) != tt.wantErr {
				t.Errorf("CheckExisting() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("CheckExisting() error = %v, should contain %v", err.Error(), tt.errMsg)
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
