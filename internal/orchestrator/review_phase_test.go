package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test isApproval with all edge cases from the design spec
func TestIsApproval_EmptyObject(t *testing.T) {
	assert.True(t, isApproval("{}"))
}

func TestIsApproval_EmptyArray(t *testing.T) {
	assert.True(t, isApproval("[]"))
}

func TestIsApproval_EmptyObjectWithWhitespace(t *testing.T) {
	assert.True(t, isApproval("{ }"))
	assert.True(t, isApproval("{\n}"))
	assert.True(t, isApproval("  {}  "))
}

func TestIsApproval_EmptyArrayWithWhitespace(t *testing.T) {
	assert.True(t, isApproval("[ ]"))
	assert.True(t, isApproval("[\n]"))
	assert.True(t, isApproval("  []  "))
}

func TestIsApproval_NonEmptyObject(t *testing.T) {
	assert.False(t, isApproval(`{"issue": "fix this"}`))
	assert.False(t, isApproval(`{"feedback": "needs improvement"}`))
	assert.False(t, isApproval(`{"a": 1}`))
}

func TestIsApproval_NonEmptyArray(t *testing.T) {
	assert.False(t, isApproval(`["problem"]`))
	assert.False(t, isApproval(`["a", "b"]`))
	assert.False(t, isApproval(`[1, 2, 3]`))
}

func TestIsApproval_EmptyString(t *testing.T) {
	assert.False(t, isApproval(""))
}

func TestIsApproval_JSONString(t *testing.T) {
	assert.False(t, isApproval(`"{}"`))
	assert.False(t, isApproval(`"approved"`))
	assert.False(t, isApproval(`"true"`))
}

func TestIsApproval_JSONBoolean(t *testing.T) {
	assert.False(t, isApproval("true"))
	assert.False(t, isApproval("false"))
}

func TestIsApproval_JSONNumber(t *testing.T) {
	assert.False(t, isApproval("0"))
	assert.False(t, isApproval("42"))
	assert.False(t, isApproval("3.14"))
}

func TestIsApproval_JSONNull(t *testing.T) {
	assert.False(t, isApproval("null"))
}

func TestIsApproval_InvalidJSON(t *testing.T) {
	assert.False(t, isApproval("not json"))
	assert.False(t, isApproval("{invalid}"))
	assert.False(t, isApproval("["))
	assert.False(t, isApproval("}{"))
}

func TestIsApproval_PlainText(t *testing.T) {
	assert.False(t, isApproval("This needs improvement"))
	assert.False(t, isApproval("LGTM"))
	assert.False(t, isApproval("Please fix the bug"))
}
