#!/bin/sh
# Bid script for MarkdownLint agent
# Bids "claim" (parallel) on TerraformDocumentation artefacts to format markdown

set -e

# Require jq for JSON parsing
if ! command -v jq > /dev/null; then
    echo "Error: jq is not installed. Cannot determine bid." >&2
    echo "ignore"
    exit 0
fi

input=$(cat)
artefact_type=$(echo "$input" | jq -r '.type')

# MarkdownLint bids "claim" (parallel phase) on TerraformDocumentation artefacts
# This allows it to work concurrently with other parallel agents
if [ "$artefact_type" = "TerraformDocumentation" ]; then
    echo "claim"
else
    echo "ignore"
fi
