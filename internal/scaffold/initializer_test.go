package scaffold

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestInitialize(t *testing.T) {
	tests := []struct {
		name      string
		force     bool
		setupFunc func(string)
		wantErr   bool
	}{
		{
			name:  "fresh initialization",
			force: false,
			setupFunc: func(dir string) {
				// No setup needed - clean directory
			},
			wantErr: false,
		},
		{
			name:  "force initialization removes existing files",
			force: true,
			setupFunc: func(dir string) {
				// Create existing files
				os.WriteFile(filepath.Join(dir, "holt.yml"), []byte("old content"), 0644)
				os.MkdirAll(filepath.Join(dir, "agents", "old-agent"), 0755)
				os.WriteFile(filepath.Join(dir, "agents", "old-agent", "old.txt"), []byte("old"), 0644)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "init-test-*")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(tmpDir)

			// Change to test directory
			originalDir, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			defer os.Chdir(originalDir)

			if err := os.Chdir(tmpDir); err != nil {
				t.Fatal(err)
			}

			// Run setup
			tt.setupFunc(tmpDir)

			// Run initialization
			err = Initialize(tt.force)

			if (err != nil) != tt.wantErr {
				t.Errorf("Initialize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify all expected files were created
				expectedFiles := []struct {
					path        string
					shouldExist bool
					executable  bool
				}{
					{"holt.yml", true, false},
					{"agents/example-agent/Dockerfile", true, false},
					{"agents/example-agent/run.sh", true, true},
					{"agents/example-agent/README.md", true, false},
				}

				for _, ef := range expectedFiles {
					fullPath := filepath.Join(tmpDir, ef.path)
					info, err := os.Stat(fullPath)

					if ef.shouldExist {
						if err != nil {
							t.Errorf("Expected file %s to exist, but got error: %v", ef.path, err)
							continue
						}

						// Check if file should be executable
						if ef.executable {
							mode := info.Mode()
							if mode&0111 == 0 {
								t.Errorf("File %s should be executable, but mode is %v", ef.path, mode)
							}
						}
					} else {
						if err == nil {
							t.Errorf("Expected file %s to not exist, but it does", ef.path)
						}
					}
				}

				// Verify holt.yml is valid YAML
				content, err := os.ReadFile(filepath.Join(tmpDir, "holt.yml"))
				if err != nil {
					t.Errorf("Failed to read holt.yml: %v", err)
				}

				var yamlData interface{}
				if err := yaml.Unmarshal(content, &yamlData); err != nil {
					t.Errorf("holt.yml is not valid YAML: %v", err)
				}

				// If force was true, verify old files were removed
				if tt.force {
					oldAgentPath := filepath.Join(tmpDir, "agents", "old-agent")
					if _, err := os.Stat(oldAgentPath); err == nil {
						t.Errorf("Expected old-agent to be removed, but it still exists")
					}
				}
			}
		})
	}
}

func TestHandleForce(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(string)
		wantErr   bool
	}{
		{
			name: "removes existing holt.yml",
			setupFunc: func(dir string) {
				os.WriteFile(filepath.Join(dir, "holt.yml"), []byte("content"), 0644)
			},
			wantErr: false,
		},
		{
			name: "removes existing agents directory",
			setupFunc: func(dir string) {
				os.MkdirAll(filepath.Join(dir, "agents", "test-agent"), 0755)
				os.WriteFile(filepath.Join(dir, "agents", "test-agent", "file.txt"), []byte("test"), 0644)
			},
			wantErr: false,
		},
		{
			name: "handles when files don't exist",
			setupFunc: func(dir string) {
				// No files to remove
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "force-test-*")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(tmpDir)

			originalDir, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			defer os.Chdir(originalDir)

			if err := os.Chdir(tmpDir); err != nil {
				t.Fatal(err)
			}

			tt.setupFunc(tmpDir)

			err = handleForce()

			if (err != nil) != tt.wantErr {
				t.Errorf("handleForce() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify files were removed
			if _, err := os.Stat(filepath.Join(tmpDir, "holt.yml")); err == nil {
				t.Errorf("holt.yml should have been removed")
			}

			if _, err := os.Stat(filepath.Join(tmpDir, "agents")); err == nil {
				t.Errorf("agents/ should have been removed")
			}
		})
	}
}

func TestGetTemplateFiles(t *testing.T) {
	files, err := getTemplateFiles()
	if err != nil {
		t.Fatalf("getTemplateFiles() error = %v", err)
	}

	expectedFiles := map[string]struct {
		permissions os.FileMode
	}{
		"holt.yml": {0644},
		filepath.Join("agents", "example-agent", "Dockerfile"): {0644},
		filepath.Join("agents", "example-agent", "run.sh"):     {0755},
		filepath.Join("agents", "example-agent", "README.md"):  {0644},
	}

	if len(files) != len(expectedFiles) {
		t.Errorf("getTemplateFiles() returned %d files, want %d", len(files), len(expectedFiles))
	}

	for _, file := range files {
		expected, ok := expectedFiles[file.Path]
		if !ok {
			t.Errorf("Unexpected file in template: %s", file.Path)
			continue
		}

		if file.Permissions != expected.permissions {
			t.Errorf("File %s has permissions %v, want %v", file.Path, file.Permissions, expected.permissions)
		}

		if len(file.Content) == 0 {
			t.Errorf("File %s has empty content", file.Path)
		}
	}
}

func TestCreateDirectories(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "create-dirs-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	if err := createDirectories(); err != nil {
		t.Fatalf("createDirectories() error = %v", err)
	}

	expectedDirs := []string{
		"agents",
		filepath.Join("agents", "example-agent"),
	}

	for _, dir := range expectedDirs {
		fullPath := filepath.Join(tmpDir, dir)
		info, err := os.Stat(fullPath)
		if err != nil {
			t.Errorf("Expected directory %s to exist, but got error: %v", dir, err)
			continue
		}

		if !info.IsDir() {
			t.Errorf("Expected %s to be a directory", dir)
		}
	}
}

func TestWriteFiles(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func() (string, func())
		files     []FileInfo
		wantErr   bool
	}{
		{
			name: "successful write",
			setupFunc: func() (string, func()) {
				tmpDir, err := os.MkdirTemp("", "write-files-test-*")
				if err != nil {
					t.Fatal(err)
				}
				return tmpDir, func() { os.RemoveAll(tmpDir) }
			},
			files: []FileInfo{
				{
					Path:        "test.txt",
					Content:     []byte("test content"),
					Permissions: 0644,
				},
				{
					Path:        "script.sh",
					Content:     []byte("#!/bin/bash\necho test"),
					Permissions: 0755,
				},
			},
			wantErr: false,
		},
		{
			name: "fails when directory doesn't exist",
			setupFunc: func() (string, func()) {
				tmpDir, err := os.MkdirTemp("", "write-files-fail-test-*")
				if err != nil {
					t.Fatal(err)
				}
				return tmpDir, func() { os.RemoveAll(tmpDir) }
			},
			files: []FileInfo{
				{
					Path:        "nonexistent/dir/file.txt",
					Content:     []byte("test"),
					Permissions: 0644,
				},
			},
			wantErr: true,
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

			err = writeFiles(tt.files)

			if (err != nil) != tt.wantErr {
				t.Errorf("writeFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				for _, file := range tt.files {
					fullPath := filepath.Join(dir, file.Path)

					// Check file exists
					info, err := os.Stat(fullPath)
					if err != nil {
						t.Errorf("Expected file %s to exist, but got error: %v", file.Path, err)
						continue
					}

					// Check permissions
					if info.Mode().Perm() != file.Permissions {
						t.Errorf("File %s has permissions %v, want %v", file.Path, info.Mode().Perm(), file.Permissions)
					}

					// Check content
					content, err := os.ReadFile(fullPath)
					if err != nil {
						t.Errorf("Failed to read file %s: %v", file.Path, err)
						continue
					}

					if string(content) != string(file.Content) {
						t.Errorf("File %s has content %q, want %q", file.Path, content, file.Content)
					}
				}
			}
		})
	}
}

func TestValidateCreatedFiles(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(string)
		wantErr   bool
	}{
		{
			name: "valid YAML",
			setupFunc: func(dir string) {
				validYaml := `version: '1.0'
agents:
  test-agent:
    role: 'test'
`
				os.WriteFile(filepath.Join(dir, "holt.yml"), []byte(validYaml), 0644)
			},
			wantErr: false,
		},
		{
			name: "invalid YAML",
			setupFunc: func(dir string) {
				invalidYaml := `version: '1.0'
agents:
  test-agent:
    role: 'test'
  - invalid syntax
`
				os.WriteFile(filepath.Join(dir, "holt.yml"), []byte(invalidYaml), 0644)
			},
			wantErr: true,
		},
		{
			name: "missing file",
			setupFunc: func(dir string) {
				// Don't create holt.yml
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "validate-test-*")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(tmpDir)

			originalDir, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			defer os.Chdir(originalDir)

			if err := os.Chdir(tmpDir); err != nil {
				t.Fatal(err)
			}

			tt.setupFunc(tmpDir)

			err = validateCreatedFiles()

			if (err != nil) != tt.wantErr {
				t.Errorf("validateCreatedFiles() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
