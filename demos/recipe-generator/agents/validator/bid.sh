#!/bin/sh
# Dynamic bidding script for the Validator agent

set -e

# This agent requires jq to parse the input JSON.
if ! command -v jq > /dev/null; then
    echo "Error: jq is not installed. Cannot determine bid." >&2
    echo "ignore"
    exit 0
fi

input=$(cat)
artefact_type=$(echo "$input" | jq -r '.type')

# The Validator's job is to review RecipeYAML artefacts.
if [ "$artefact_type" = "RecipeYAML" ]; then
  echo "review"
else
  # It ignores everything else.
  echo "ignore"
fi
