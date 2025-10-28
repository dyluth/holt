#!/bin/sh
# Bid script for DocGenerator agent
# Bids "exclusive" on TerraformCode artefacts (after reviews pass) to generate documentation

set -e

# Require jq for JSON parsing
if ! command -v jq > /dev/null; then
    echo "Error: jq is not installed. Cannot determine bid." >&2
    echo "ignore"
    exit 0
fi

input=$(cat)
artefact_type=$(echo "$input" | jq -r '.type')

# DocGenerator bids "exclusive" on TerraformCode artefacts
# The orchestrator ensures reviews have passed before granting this exclusive claim
if [ "$artefact_type" = "TerraformCode" ]; then
    echo "exclusive"
else
    echo "ignore"
fi
