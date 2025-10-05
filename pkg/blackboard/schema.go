package blackboard

import "fmt"

// Redis key pattern helpers
//
// All Redis keys and Pub/Sub channels are namespaced by instance name to enable
// multiple Sett instances to safely coexist on a single Redis server.
//
// Key pattern: sett:{instance_name}:{entity}:{uuid}
// Channel pattern: sett:{instance_name}:{event_type}_events

// ArtefactKey returns the Redis key for an artefact.
// Pattern: sett:{instance_name}:artefact:{artefact_id}
func ArtefactKey(instanceName, artefactID string) string {
	return fmt.Sprintf("sett:%s:artefact:%s", instanceName, artefactID)
}

// ClaimKey returns the Redis key for a claim.
// Pattern: sett:{instance_name}:claim:{claim_id}
func ClaimKey(instanceName, claimID string) string {
	return fmt.Sprintf("sett:%s:claim:%s", instanceName, claimID)
}

// ClaimBidsKey returns the Redis key for a claim's bids hash.
// Pattern: sett:{instance_name}:claim:{claim_id}:bids
func ClaimBidsKey(instanceName, claimID string) string {
	return fmt.Sprintf("sett:%s:claim:%s:bids", instanceName, claimID)
}

// ThreadKey returns the Redis key for a thread tracking ZSET.
// Pattern: sett:{instance_name}:thread:{logical_id}
func ThreadKey(instanceName, logicalID string) string {
	return fmt.Sprintf("sett:%s:thread:%s", instanceName, logicalID)
}

// ArtefactEventsChannel returns the Pub/Sub channel name for artefact events.
// Pattern: sett:{instance_name}:artefact_events
func ArtefactEventsChannel(instanceName string) string {
	return fmt.Sprintf("sett:%s:artefact_events", instanceName)
}

// ClaimEventsChannel returns the Pub/Sub channel name for claim events.
// Pattern: sett:{instance_name}:claim_events
func ClaimEventsChannel(instanceName string) string {
	return fmt.Sprintf("sett:%s:claim_events", instanceName)
}
