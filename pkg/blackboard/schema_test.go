package blackboard

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

// TestArtefactKey tests artefact key generation
func TestArtefactKey(t *testing.T) {
	instanceName := "default-1"
	artefactID := uuid.New().String()

	key := ArtefactKey(instanceName, artefactID)

	expected := "holt:default-1:artefact:" + artefactID
	if key != expected {
		t.Errorf("ArtefactKey() = %q, expected %q", key, expected)
	}

	// Verify format
	if !strings.HasPrefix(key, "holt:") {
		t.Error("artefact key should start with 'holt:'")
	}
	if !strings.Contains(key, ":artefact:") {
		t.Error("artefact key should contain ':artefact:'")
	}
}

// TestClaimKey tests claim key generation
func TestClaimKey(t *testing.T) {
	instanceName := "myproject"
	claimID := uuid.New().String()

	key := ClaimKey(instanceName, claimID)

	expected := "holt:myproject:claim:" + claimID
	if key != expected {
		t.Errorf("ClaimKey() = %q, expected %q", key, expected)
	}

	// Verify format
	if !strings.HasPrefix(key, "holt:") {
		t.Error("claim key should start with 'holt:'")
	}
	if !strings.Contains(key, ":claim:") {
		t.Error("claim key should contain ':claim:'")
	}
}

// TestClaimBidsKey tests claim bids key generation
func TestClaimBidsKey(t *testing.T) {
	instanceName := "default-1"
	claimID := uuid.New().String()

	key := ClaimBidsKey(instanceName, claimID)

	expected := "holt:default-1:claim:" + claimID + ":bids"
	if key != expected {
		t.Errorf("ClaimBidsKey() = %q, expected %q", key, expected)
	}

	// Verify format
	if !strings.HasPrefix(key, "holt:") {
		t.Error("claim bids key should start with 'holt:'")
	}
	if !strings.Contains(key, ":claim:") {
		t.Error("claim bids key should contain ':claim:'")
	}
	if !strings.HasSuffix(key, ":bids") {
		t.Error("claim bids key should end with ':bids'")
	}
}

// TestThreadKey tests thread key generation
func TestThreadKey(t *testing.T) {
	instanceName := "test-instance"
	logicalID := uuid.New().String()

	key := ThreadKey(instanceName, logicalID)

	expected := "holt:test-instance:thread:" + logicalID
	if key != expected {
		t.Errorf("ThreadKey() = %q, expected %q", key, expected)
	}

	// Verify format
	if !strings.HasPrefix(key, "holt:") {
		t.Error("thread key should start with 'holt:'")
	}
	if !strings.Contains(key, ":thread:") {
		t.Error("thread key should contain ':thread:'")
	}
}

// TestArtefactEventsChannel tests artefact events channel name generation
func TestArtefactEventsChannel(t *testing.T) {
	instanceName := "default"

	channel := ArtefactEventsChannel(instanceName)

	expected := "holt:default:artefact_events"
	if channel != expected {
		t.Errorf("ArtefactEventsChannel() = %q, expected %q", channel, expected)
	}

	// Verify format
	if !strings.HasPrefix(channel, "holt:") {
		t.Error("artefact events channel should start with 'holt:'")
	}
	if !strings.HasSuffix(channel, ":artefact_events") {
		t.Error("artefact events channel should end with ':artefact_events'")
	}
}

// TestClaimEventsChannel tests claim events channel name generation
func TestClaimEventsChannel(t *testing.T) {
	instanceName := "myproject"

	channel := ClaimEventsChannel(instanceName)

	expected := "holt:myproject:claim_events"
	if channel != expected {
		t.Errorf("ClaimEventsChannel() = %q, expected %q", channel, expected)
	}

	// Verify format
	if !strings.HasPrefix(channel, "holt:") {
		t.Error("claim events channel should start with 'holt:'")
	}
	if !strings.HasSuffix(channel, ":claim_events") {
		t.Error("claim events channel should end with ':claim_events'")
	}
}

// TestAgentEventsChannel tests agent-specific events channel name generation
func TestAgentEventsChannel(t *testing.T) {
	instanceName := "default-1"
	agentName := "go-coder"

	channel := AgentEventsChannel(instanceName, agentName)

	expected := "holt:default-1:agent:go-coder:events"
	if channel != expected {
		t.Errorf("AgentEventsChannel() = %q, expected %q", channel, expected)
	}

	// Verify format
	if !strings.HasPrefix(channel, "holt:") {
		t.Error("agent events channel should start with 'holt:'")
	}
	if !strings.Contains(channel, ":agent:") {
		t.Error("agent events channel should contain ':agent:'")
	}
	if !strings.HasSuffix(channel, ":events") {
		t.Error("agent events channel should end with ':events'")
	}
}

// TestAgentEventsChannelNamespacing tests that different instances and agents produce unique channels
func TestAgentEventsChannelNamespacing(t *testing.T) {
	// Different instances, same agent
	channel1 := AgentEventsChannel("default-1", "go-coder")
	channel2 := AgentEventsChannel("default-2", "go-coder")
	if channel1 == channel2 {
		t.Error("channels for different instances should be different")
	}

	// Same instance, different agents
	channel3 := AgentEventsChannel("default-1", "go-coder")
	channel4 := AgentEventsChannel("default-1", "reviewer")
	if channel3 == channel4 {
		t.Error("channels for different agents should be different")
	}

	// All should have expected format
	channels := []string{channel1, channel2, channel3, channel4}
	for _, ch := range channels {
		if !strings.HasPrefix(ch, "holt:") || !strings.Contains(ch, ":agent:") || !strings.HasSuffix(ch, ":events") {
			t.Errorf("channel %q has incorrect format", ch)
		}
	}
}

// TestInstanceNameNamespacing tests that different instance names produce different keys
func TestInstanceNameNamespacing(t *testing.T) {
	artefactID := uuid.New().String()

	key1 := ArtefactKey("default-1", artefactID)
	key2 := ArtefactKey("default-2", artefactID)
	key3 := ArtefactKey("myproject", artefactID)

	// All keys should be different
	if key1 == key2 {
		t.Error("keys for different instances should be different")
	}
	if key1 == key3 {
		t.Error("keys for different instances should be different")
	}
	if key2 == key3 {
		t.Error("keys for different instances should be different")
	}

	// But they should all contain the same artefact ID
	if !strings.Contains(key1, artefactID) || !strings.Contains(key2, artefactID) || !strings.Contains(key3, artefactID) {
		t.Error("all keys should contain the artefact ID")
	}
}

// TestChannelNamespacing tests that different instance names produce different channel names
func TestChannelNamespacing(t *testing.T) {
	channel1 := ArtefactEventsChannel("default-1")
	channel2 := ArtefactEventsChannel("default-2")
	channel3 := ArtefactEventsChannel("myproject")

	// All channels should be different
	if channel1 == channel2 {
		t.Error("channels for different instances should be different")
	}
	if channel1 == channel3 {
		t.Error("channels for different instances should be different")
	}
	if channel2 == channel3 {
		t.Error("channels for different instances should be different")
	}

	// But they should all have the same suffix
	if !strings.HasSuffix(channel1, ":artefact_events") ||
		!strings.HasSuffix(channel2, ":artefact_events") ||
		!strings.HasSuffix(channel3, ":artefact_events") {
		t.Error("all artefact event channels should have the same suffix")
	}
}

// TestKeyFormatsWithSpecialCharacters tests key generation with various instance names
func TestKeyFormatsWithSpecialCharacters(t *testing.T) {
	testCases := []struct {
		name         string
		instanceName string
	}{
		{"simple name", "default"},
		{"with number", "default-1"},
		{"with hyphens", "my-project-123"},
		{"lowercase", "myproject"},
	}

	artefactID := uuid.New().String()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			key := ArtefactKey(tc.instanceName, artefactID)
			expectedPrefix := "holt:" + tc.instanceName + ":artefact:"
			if !strings.HasPrefix(key, expectedPrefix) {
				t.Errorf("key should start with %q, got %q", expectedPrefix, key)
			}
		})
	}
}
