#!/bin/sh
# Example Reviewer agent tool script for M3.1
# Always outputs Review artefact with approval payload "{}"

set -e  # Exit on any error

# Read JSON input from stdin (required by cub contract)
input=$(cat)

# Log to stderr (visible in agent logs)
echo "Reviewer agent received claim, auto-approving..." >&2

# Output Review artefact with approval payload
# Payload of "{}" indicates approval (no feedback)
cat <<EOF
{
  "structural_type": "Review",
  "payload": "{}"
}
EOF
