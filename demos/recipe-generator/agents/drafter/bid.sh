#!/bin/sh
# Dynamic bidding script for the Drafter agent

set -e

# This agent requires jq to parse the input JSON.
if ! command -v jq > /dev/null; then
    echo "Error: jq is not installed. Cannot determine bid." >&2
    echo "ignore"
    exit 0
fi

input=$(cat)
artefact_type=$(echo "$input" | jq -r '.type')

# The Drafter's job is to create the first draft from a GoalDefined artefact.
if [ "$artefact_type" = "GoalDefined" ]; then
  echo "exclusive"
else
  # It ignores everything else.
  echo "ignore"
fi
