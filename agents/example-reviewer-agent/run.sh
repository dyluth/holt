#!/bin/sh
# Example Reviewer agent tool script for M3.2
# Always outputs Review artefact with approval payload "{}"

set -e  # Exit on any error

# Read JSON input from stdin (required by pup contract)
input=$(cat)

# Log to stderr (visible in agent logs)
echo "Reviewer agent received claim, auto-approving..." >&2

# Output Review artefact with approval payload
# Payload of "{}" indicates approval (no feedback)
# Tool contract requires: artefact_type, artefact_payload, summary, structural_type
cat <<EOF
{
  "artefact_type": "Review",
  "artefact_payload": "{}",
  "summary": "Review approved - no issues found",
  "structural_type": "Review"
}
EOF
