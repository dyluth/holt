package printer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestError(t *testing.T) {
	t.Run("returns error with title", func(t *testing.T) {
		err := Error("Test Error", "This is a test error", []string{})
		require.Error(t, err)
		require.Equal(t, "Test Error", err.Error())
	})

	t.Run("returns error with title when including suggestions", func(t *testing.T) {
		err := Error("Test Error", "Explanation", []string{"Try this fix"})
		require.Error(t, err)
		require.Equal(t, "Test Error", err.Error())
	})

	t.Run("returns error with title for multiple suggestions", func(t *testing.T) {
		err := Error("Test Error", "Explanation", []string{
			"First option",
			"Second option",
		})
		require.Error(t, err)
		require.Equal(t, "Test Error", err.Error())
	})
}

func TestErrorWithContext(t *testing.T) {
	t.Run("returns error with title", func(t *testing.T) {
		context := map[string]string{
			"Workspace": "/path/to/workspace",
			"Instance":  "test-instance",
		}
		err := ErrorWithContext("Test Error", "Explanation", context, []string{})
		require.Error(t, err)
		require.Equal(t, "Test Error", err.Error())
	})

	t.Run("returns error with title when including suggestions", func(t *testing.T) {
		context := map[string]string{"Key": "Value"}
		err := ErrorWithContext("Test Error", "Explanation", context, []string{"Fix it"})
		require.Error(t, err)
		require.Equal(t, "Test Error", err.Error())
	})
}

// Note: The Error and ErrorWithContext functions print formatted output to stderr
// with colors. The error object returned only contains the title for Cobra's error handling.
// This is intentional to avoid duplicate output while providing rich formatted errors.
