#!/bin/sh
# Bid script for TerraformFmt agent
# Bids "review" on TerraformCode artefacts to validate formatting

set -e

# Require jq for JSON parsing
if ! command -v jq > /dev/null; then
    echo "Error: jq is not installed. Cannot determine bid." >&2
    echo "ignore"
    exit 0
fi

input=$(cat)
artefact_type=$(echo "$input" | jq -r '.type')

# TerraformFmt bids "review" on TerraformCode artefacts
# This runs in the review phase alongside TfLint
if [ "$artefact_type" = "TerraformCode" ]; then
    echo "review"
else
    echo "ignore"
fi
