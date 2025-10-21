package scaffold

import (
	"fmt"
	"os"
)

// CheckExisting checks if holt.yml or agents/ directory already exist
// Returns an error if they do, nil otherwise
func CheckExisting() error {
	var existingFiles []string

	// Check for holt.yml
	if _, err := os.Stat("holt.yml"); err == nil {
		existingFiles = append(existingFiles, "holt.yml")
	}

	// Check for agents/ directory
	if info, err := os.Stat("agents"); err == nil && info.IsDir() {
		existingFiles = append(existingFiles, "agents/")
	}

	if len(existingFiles) > 0 {
		errMsg := "project already initialized\n\nFound existing"
		if len(existingFiles) == 1 {
			errMsg += fmt.Sprintf(": %s", existingFiles[0])
		} else {
			errMsg += " files:\n"
			for _, file := range existingFiles {
				errMsg += fmt.Sprintf("  - %s\n", file)
			}
		}
		errMsg += "\nUse 'holt init --force' to reinitialize (this will overwrite existing configuration)"

		return fmt.Errorf("%s", errMsg)
	}

	return nil
}
