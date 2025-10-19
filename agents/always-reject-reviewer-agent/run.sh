#!/bin/sh
# Always-Reject Reviewer agent tool script for M3.3 testing
# Always rejects with feedback regardless of version
# Enables testing of max iterations termination

set -e  # Exit on any error

# Read JSON input from stdin (required by cub contract)
input=$(cat)

# Extract the target artefact version from input JSON
version=$(echo "$input" | grep -o '"version":[0-9]*' | head -1 | grep -o '[0-9]*')

echo "Always-reject reviewer received claim, version: $version - REJECTING" >&2

# Always output Review artefact with feedback payload (non-empty = rejection)
cat <<EOF
{
  "artefact_type": "Review",
  "artefact_payload": "{\"issue\": \"still needs improvement\", \"version_reviewed\": $version}",
  "summary": "Review rejected - iteration $version",
  "structural_type": "Review"
}
EOF
