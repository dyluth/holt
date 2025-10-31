#!/bin/sh
# Bid script for ModulePackager agent
# Bids "exclusive" on FormattedDocumentation artefacts to create final package

set -e

# Require jq for JSON parsing
if ! command -v jq > /dev/null; then
    echo "Error: jq is not installed. Cannot determine bid." >&2
    echo "ignore"
    exit 0
fi

input=$(cat)
artefact_type=$(echo "$input" | jq -r '.type')

# ModulePackager bids "exclusive" on FormattedDocumentation artefacts
# This is the final step that creates the Terminal artefact
if [ "$artefact_type" = "FormattedDocumentation" ]; then
    echo "exclusive"
else
    echo "ignore"
fi
