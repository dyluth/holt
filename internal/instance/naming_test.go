package instance

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateName(t *testing.T) {
	testCases := []struct {
		name      string
		inputName string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid simple name",
			inputName: "prod",
			wantErr:   false,
		},
		{
			name:      "valid name with hyphens",
			inputName: "staging-1",
			wantErr:   false,
		},
		{
			name:      "valid name with numbers",
			inputName: "default-123",
			wantErr:   false,
		},
		{
			name:      "empty name",
			inputName: "",
			wantErr:   true,
			errMsg:    "cannot be empty",
		},
		{
			name:      "name with uppercase",
			inputName: "Prod",
			wantErr:   true,
			errMsg:    "must be lowercase",
		},
		{
			name:      "name starting with hyphen",
			inputName: "-prod",
			wantErr:   true,
			errMsg:    "not at start/end",
		},
		{
			name:      "name ending with hyphen",
			inputName: "prod-",
			wantErr:   true,
			errMsg:    "not at start/end",
		},
		{
			name:      "name with underscore",
			inputName: "prod_env",
			wantErr:   true,
			errMsg:    "must be lowercase alphanumeric",
		},
		{
			name:      "name with special characters",
			inputName: "prod@123",
			wantErr:   true,
			errMsg:    "must be lowercase alphanumeric",
		},
		{
			name:      "name too long",
			inputName: "this-is-a-very-long-instance-name-that-exceeds-the-maximum-length-of-63-characters",
			wantErr:   true,
			errMsg:    "too long",
		},
		{
			name:      "single character name",
			inputName: "a",
			wantErr:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateName(tc.inputName)
			if tc.wantErr {
				assert.Error(t, err)
				if tc.errMsg != "" {
					assert.Contains(t, err.Error(), tc.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateName_MaxLength(t *testing.T) {
	// Test exactly 63 characters (max allowed)
	name63 := "a23456789012345678901234567890123456789012345678901234567890123"
	assert.Len(t, name63, 63)
	err := ValidateName(name63)
	assert.NoError(t, err)

	// Test 64 characters (too long)
	name64 := "a234567890123456789012345678901234567890123456789012345678901234"
	assert.Len(t, name64, 64)
	err = ValidateName(name64)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too long")
}
