#!/bin/sh
# Bid script for TerraformDrafter agent
# Bids "exclusive" on GoalDefined artefacts to generate Terraform code

set -e

# Require jq for JSON parsing
if ! command -v jq > /dev/null; then
    echo "Error: jq is not installed. Cannot determine bid." >&2
    echo "ignore"
    exit 0
fi

input=$(cat)
artefact_type=$(echo "$input" | jq -r '.type')

# TerraformDrafter bids "exclusive" on GoalDefined artefacts
# This is the entry point of the workflow
if [ "$artefact_type" = "GoalDefined" ]; then
    echo "exclusive"
else
    echo "ignore"
fi
