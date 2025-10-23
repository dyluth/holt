#!/bin/sh
set -e

echo "Validator Agent: Received recipe for review." >&2
input=$(cat)
commit_hash=$(echo "$input" | jq -r '.target_artefact.payload')

# Use 'git show' to read the file content from the specific commit
# without modifying the working directory. Pipe output directly to grep.
short_instruction=$(git show "${commit_hash}:recipe.yaml" | grep -E '^\s*-\s*".{1,14}"$' || true)

if [ -n "$short_instruction" ]; then
  echo "Validator Agent: Found issue: Instruction is too short." >&2
  # REJECTION: Use jq to properly build JSON with escaped strings
  jq -n \
    --arg issue "Instruction is too short. Please be more descriptive." \
    --arg line "$short_instruction" \
    '{
      artefact_type: "Review",
      artefact_payload: ({issue: $issue, line: $line} | tostring),
      summary: "Rejected: Instruction too short"
    }'
else
  echo "Validator Agent: Recipe looks good. Approving." >&2
  # APPROVAL: Output empty JSON object
  cat <<EOF
{
  "artefact_type": "Review",
  "artefact_payload": "{}",
  "summary": "Approved recipe draft"
}
EOF
fi