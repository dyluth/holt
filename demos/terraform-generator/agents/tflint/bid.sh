#!/bin/sh
# Bid script for TfLint agent
# Bids "review" on TerraformCode artefacts to validate best practices

set -e

# Require jq for JSON parsing
if ! command -v jq > /dev/null; then
    echo "Error: jq is not installed. Cannot determine bid." >&2
    echo "ignore"
    exit 0
fi

input=$(cat)
artefact_type=$(echo "$input" | jq -r '.type')

# TfLint bids "review" on TerraformCode artefacts
# This runs in the review phase alongside TerraformFmt
if [ "$artefact_type" = "TerraformCode" ]; then
    echo "review"
else
    echo "ignore"
fi
