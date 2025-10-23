#!/bin/sh
# Dynamic bidding script for the Formatter agent

set -e

# This agent requires jq to parse the input JSON.
if ! command -v jq > /dev/null; then
    echo "Error: jq is not installed. Cannot determine bid." >&2
    echo "ignore"
    exit 0
fi

input=$(cat)
artefact_type=$(echo "$input" | jq -r '.type')

# The Formatter's job is to create a markdown file from an approved RecipeYAML artefact.
if [ "$artefact_type" = "RecipeYAML" ]; then
  echo "claim"
else
  # It ignores everything else.
  echo "ignore"
fi
