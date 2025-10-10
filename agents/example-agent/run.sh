#!/bin/sh
# Example agent tool script for M2.3
# This is a simple echo agent that demonstrates the stdin/stdout JSON contract

# Read JSON input from stdin
input=$(cat)

# Log to stderr (visible in agent logs, not sent to cub)
echo "Echo agent received claim, processing..." >&2
echo "Input: $input" >&2

# Generate timestamp for unique payload
timestamp=$(date +%s)

# Output success JSON to stdout
# This JSON will be parsed by the cub and converted to an artefact
cat <<EOF
{
  "artefact_type": "EchoSuccess",
  "artefact_payload": "echo-$timestamp",
  "summary": "Echo agent successfully processed the claim"
}
EOF
