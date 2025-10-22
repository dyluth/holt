#!/bin/sh
# Conditional Reviewer agent tool script for M3.3 testing
# Rejects version 1, approves version 2+
# This enables testing of the feedback loop iteration

set -e  # Exit on any error

# Read JSON input from stdin (required by pup contract)
input=$(cat)

# Extract the target artefact version from input JSON
# Input structure: {"claim_type": "...", "target_artefact": {...}, "context_chain": [...]}
version=$(echo "$input" | grep -o '"version":[[:space:]]*[0-9]*' | head -1 | sed 's/[^0-9]//g')

# Default to 1 if version not found (defensive)
if [ -z "$version" ]; then
    version="1"
    echo "Warning: Could not extract version from input, defaulting to 1" >&2
fi

echo "Conditional reviewer received claim, checking version: $version" >&2

# Reject version 1, approve version 2+
if [ "$version" = "1" ]; then
    echo "Version 1 detected - REJECTING with feedback" >&2
    # Output Review artefact with feedback payload (non-empty = rejection)
    cat <<EOF
{
  "artefact_type": "Review",
  "artefact_payload": "{\"issue\": \"needs tests\", \"severity\": \"high\", \"details\": \"Please add unit tests for the implementation\"}",
  "summary": "Review rejected - needs tests",
  "structural_type": "Review"
}
EOF
else
    echo "Version $version detected - APPROVING" >&2
    # Output Review artefact with approval payload (empty object = approval)
    cat <<EOF
{
  "artefact_type": "Review",
  "artefact_payload": "{}",
  "summary": "Review approved after rework",
  "structural_type": "Review"
}
EOF
fi
