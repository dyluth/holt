package blackboard

import "fmt"

// Redis key pattern helpers
//
// All Redis keys and Pub/Sub channels are namespaced by instance name to enable
// multiple Holt instances to safely coexist on a single Redis server.
//
// Key pattern: holt:{instance_name}:{entity}:{uuid}
// Channel pattern: holt:{instance_name}:{event_type}_events

// ArtefactKey returns the Redis key for an artefact.
// Pattern: holt:{instance_name}:artefact:{artefact_id}
func ArtefactKey(instanceName, artefactID string) string {
	return fmt.Sprintf("holt:%s:artefact:%s", instanceName, artefactID)
}

// ClaimKey returns the Redis key for a claim.
// Pattern: holt:{instance_name}:claim:{claim_id}
func ClaimKey(instanceName, claimID string) string {
	return fmt.Sprintf("holt:%s:claim:%s", instanceName, claimID)
}

// ClaimBidsKey returns the Redis key for a claim's bids hash.
// Pattern: holt:{instance_name}:claim:{claim_id}:bids
func ClaimBidsKey(instanceName, claimID string) string {
	return fmt.Sprintf("holt:%s:claim:%s:bids", instanceName, claimID)
}

// ClaimByArtefactKey returns the Redis key for the artefact->claim index.
// This enables idempotency checking by looking up claims by artefact ID.
// Pattern: holt:{instance_name}:claim_by_artefact:{artefact_id}
func ClaimByArtefactKey(instanceName, artefactID string) string {
	return fmt.Sprintf("holt:%s:claim_by_artefact:%s", instanceName, artefactID)
}

// ThreadKey returns the Redis key for a thread tracking ZSET.
// Pattern: holt:{instance_name}:thread:{logical_id}
func ThreadKey(instanceName, logicalID string) string {
	return fmt.Sprintf("holt:%s:thread:%s", instanceName, logicalID)
}

// ArtefactEventsChannel returns the Pub/Sub channel name for artefact events.
// Pattern: holt:{instance_name}:artefact_events
func ArtefactEventsChannel(instanceName string) string {
	return fmt.Sprintf("holt:%s:artefact_events", instanceName)
}

// ClaimEventsChannel returns the Pub/Sub channel name for claim events.
// Pattern: holt:{instance_name}:claim_events
func ClaimEventsChannel(instanceName string) string {
	return fmt.Sprintf("holt:%s:claim_events", instanceName)
}

// AgentEventsChannel returns the agent-specific event channel name.
// Used by orchestrator to publish grant notifications to individual agents.
// Pattern: holt:{instance_name}:agent:{agent_name}:events
func AgentEventsChannel(instanceName, agentName string) string {
	return fmt.Sprintf("holt:%s:agent:%s:events", instanceName, agentName)
}

// WorkflowEventsChannel returns the Pub/Sub channel name for workflow events.
// This channel carries bid submissions and claim grants for real-time monitoring.
// Pattern: holt:{instance_name}:workflow_events
func WorkflowEventsChannel(instanceName string) string {
	return fmt.Sprintf("holt:%s:workflow_events", instanceName)
}

// AgentImagesKey returns the Redis key for the agent images hash (M3.9).
// This hash stores agent role → Docker image ID mappings for audit trails.
// Pattern: holt:{instance_name}:agent_images
func AgentImagesKey(instanceName string) string {
	return fmt.Sprintf("holt:%s:agent_images", instanceName)
}
