#!/bin/sh
# Example Parallel agent tool script for M3.2
# Outputs Standard artefact indicating parallel work completed

set -e  # Exit on any error

# Read JSON input from stdin (required by cub contract)
input=$(cat)

# Log to stderr (visible in agent logs)
echo "Parallel agent received claim, performing parallel work..." >&2

# Extract target artefact type from input for logging
target_type=$(echo "$input" | grep -o '"type":"[^"]*"' | head -1 | cut -d'"' -f4)
echo "Target artefact type: $target_type" >&2

# Simulate parallel work (e.g., running tests, generating docs)
sleep 1

# Output Standard artefact with completion message
cat <<EOF
{
  "artefact_type": "ParallelWorkComplete",
  "artefact_payload": "Parallel work completed successfully",
  "summary": "Completed parallel processing"
}
EOF
