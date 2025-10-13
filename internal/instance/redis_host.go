package instance

import (
	"fmt"
	"os"
)

// GetRedisHost returns the appropriate Redis hostname for the current environment.
// In Docker-in-Docker scenarios, it returns "host.docker.internal" to access
// the host's published ports. Otherwise, it returns "localhost".
func GetRedisHost() string {
	// Check if we're running in Docker
	if _, err := os.Stat("/.dockerenv"); err == nil {
		// We're in Docker, use host.docker.internal to reach the host's published ports
		return "host.docker.internal"
	}
	return "localhost"
}

// GetRedisURL constructs the full Redis URL for a given port.
func GetRedisURL(port int) string {
	host := GetRedisHost()
	return fmt.Sprintf("redis://%s:%d", host, port)
}
