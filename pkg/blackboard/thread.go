package blackboard

// Thread tracking utilities
//
// Threads group multiple versions of the same logical artefact. They are stored
// in Redis as ZSETs (sorted sets) where:
// - Key: holt:{instance_name}:thread:{logical_id}
// - Members: artefact IDs
// - Score: The artefact's version number (as float64)
//
// This enables efficient retrieval of the latest version and traversal of version history.

// ThreadVersion represents a single version in a thread.
// This is used by Redis client code (M1.2) to represent thread members.
type ThreadVersion struct {
	ArtefactID string // UUID of the artefact
	Version    int    // Version number of this artefact
}

// ThreadScore converts an artefact version number to a Redis ZSET score.
// Version numbers start at 1 and increment sequentially.
func ThreadScore(version int) float64 {
	return float64(version)
}

// VersionFromScore converts a Redis ZSET score back to an artefact version number.
func VersionFromScore(score float64) int {
	return int(score)
}
