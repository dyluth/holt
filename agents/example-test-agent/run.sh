#!/bin/sh
# Example Test agent tool script for M3.1
# This script should never be called since the agent always bids "ignore"
# Included for completeness and debugging

set -e  # Exit on any error

# Read JSON input from stdin (required by cub contract)
input=$(cat)

# Log to stderr (visible in agent logs)
echo "ERROR: Test agent was granted a claim, but it should always bid ignore!" >&2
echo "This indicates a bug in the orchestrator or agent configuration." >&2

# Output failure artefact
cat <<EOF
{
  "structural_type": "Failure",
  "payload": "Test agent should never execute work (always bids ignore)"
}
EOF
