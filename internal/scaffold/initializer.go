package scaffold

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

//go:embed templates/*
var templatesFS embed.FS

// FileInfo represents a file to be created during initialization
type FileInfo struct {
	Path        string
	Content     []byte
	Permissions os.FileMode
}

// Initialize creates the Sett project structure
// If force is true, it will remove existing sett.yml and agents/ directory
func Initialize(force bool) error {
	// Handle --force flag
	if force {
		if err := handleForce(); err != nil {
			return err
		}
	}

	// Get template files
	files, err := getTemplateFiles()
	if err != nil {
		return err
	}

	// Create directories
	if err := createDirectories(); err != nil {
		return err
	}

	// Write files
	if err := writeFiles(files); err != nil {
		return err
	}

	// Validate created files
	if err := validateCreatedFiles(); err != nil {
		return err
	}

	return nil
}

// handleForce removes existing files if --force was specified
func handleForce() error {
	// Remove sett.yml if it exists
	if _, err := os.Stat("sett.yml"); err == nil {
		fmt.Println("⚠️  Removing existing sett.yml...")
		if err := os.Remove("sett.yml"); err != nil {
			return fmt.Errorf("failed to remove sett.yml: %w", err)
		}
	}

	// Remove agents/ directory if it exists
	if info, err := os.Stat("agents"); err == nil && info.IsDir() {
		fmt.Println("⚠️  Removing existing agents/ directory...")
		if err := os.RemoveAll("agents"); err != nil {
			return fmt.Errorf("failed to remove agents/ directory: %w", err)
		}
	}

	return nil
}

// getTemplateFiles reads and processes all template files
func getTemplateFiles() ([]FileInfo, error) {
	files := []FileInfo{}

	// sett.yml
	settYml, err := templatesFS.ReadFile("templates/sett.yml.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to read sett.yml template: %w", err)
	}
	files = append(files, FileInfo{
		Path:        "sett.yml",
		Content:     settYml,
		Permissions: 0644,
	})

	// agents/example-agent/Dockerfile
	dockerfile, err := templatesFS.ReadFile("templates/Dockerfile.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to read Dockerfile template: %w", err)
	}
	files = append(files, FileInfo{
		Path:        filepath.Join("agents", "example-agent", "Dockerfile"),
		Content:     dockerfile,
		Permissions: 0644,
	})

	// agents/example-agent/run.sh
	runSh, err := templatesFS.ReadFile("templates/run.sh.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to read run.sh template: %w", err)
	}
	files = append(files, FileInfo{
		Path:        filepath.Join("agents", "example-agent", "run.sh"),
		Content:     runSh,
		Permissions: 0755, // Executable
	})

	// agents/example-agent/README.md
	readme, err := templatesFS.ReadFile("templates/README.md.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to read README.md template: %w", err)
	}
	files = append(files, FileInfo{
		Path:        filepath.Join("agents", "example-agent", "README.md"),
		Content:     readme,
		Permissions: 0644,
	})

	return files, nil
}

// createDirectories creates the necessary directory structure
func createDirectories() error {
	dirs := []string{
		"agents",
		filepath.Join("agents", "example-agent"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// writeFiles writes all template files to disk
func writeFiles(files []FileInfo) error {
	for _, file := range files {
		if err := os.WriteFile(file.Path, file.Content, file.Permissions); err != nil {
			return fmt.Errorf("failed to write %s: %w", file.Path, err)
		}
	}

	return nil
}

// validateCreatedFiles validates that created files are correct
func validateCreatedFiles() error {
	// Validate sett.yml is valid YAML
	content, err := os.ReadFile("sett.yml")
	if err != nil {
		return fmt.Errorf("failed to read created sett.yml: %w", err)
	}

	var yamlData interface{}
	if err := yaml.Unmarshal(content, &yamlData); err != nil {
		return fmt.Errorf("created sett.yml is not valid YAML: %w", err)
	}

	return nil
}

// PrintSuccess prints the success message with created files
func PrintSuccess() {
	fmt.Println("\n✅ Successfully initialized Sett project!")
	fmt.Println("\nCreated:")
	fmt.Println("  ✓ sett.yml")
	fmt.Println("  ✓ agents/example-agent/Dockerfile")
	fmt.Println("  ✓ agents/example-agent/run.sh")
	fmt.Println("  ✓ agents/example-agent/README.md")
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Add '.sett/' to your .gitignore file")
	fmt.Println("  2. Customize sett.yml to add your own agents")
	fmt.Println("  3. Run 'sett up' to start the Sett orchestrator")
	fmt.Println("\nFor more information, visit: https://docs.sett.ai/getting-started")
}
