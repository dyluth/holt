#!/bin/sh
# Example Git agent tool script for M2.5
# Demonstrates CodeCommit workflow: create file → git add → git commit → return hash

set -e  # Exit on any error

# Read JSON input from stdin
input=$(cat)

# Log to stderr (visible in agent logs)
echo "Git agent received claim, processing..." >&2

# Parse target artefact payload (filename to create)
filename=$(echo "$input" | grep -o '"payload":"[^"]*"' | head -1 | cut -d'"' -f4)

# Parse claim ID from target artefact ID for commit message
claim_id=$(echo "$input" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

# Default to hello.txt if no filename provided
if [ -z "$filename" ]; then
  filename="hello.txt"
fi

echo "Creating file: $filename" >&2

# Navigate to workspace
cd /workspace

# Create file with simple content
cat > "$filename" <<EOF
# File created by Sett example-git-agent

This file was generated as part of a Sett workflow.
Filename: $filename
Timestamp: $(date -u +"%Y-%m-%d %H:%M:%S UTC")
EOF

echo "File created, adding to git..." >&2

# Git add the new file
git add "$filename"

# Commit with descriptive message including claim ID
git commit -m "[sett-agent: git-agent] Created $filename

Claim-ID: $claim_id" >&2

echo "Committed, extracting hash..." >&2

# Get commit hash
commit_hash=$(git rev-parse HEAD)

echo "Commit hash: $commit_hash" >&2

# Output CodeCommit JSON to stdout
cat <<EOF
{
  "artefact_type": "CodeCommit",
  "artefact_payload": "$commit_hash",
  "summary": "Created $filename and committed as $commit_hash"
}
EOF
