#!/bin/sh
# MarkdownLint agent - Formats markdown documentation
# Tool-based parallel worker using markdownlint-cli2

set -e

input=$(cat)
cd /workspace

# Configure git user for commits
git config user.email "markdownlint@holt.demo"
git config user.name "Holt MarkdownLint"

# Capture original branch to preserve user's workspace state
original_branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")

# Extract commit hash from target artefact
commit_hash=$(echo "$input" | jq -r '.target_artefact.payload')

echo "MarkdownLint: Received TerraformDocumentation commit: $commit_hash" >&2
echo "MarkdownLint: Original branch: $original_branch" >&2
echo "MarkdownLint: Formatting markdown files..." >&2

# Checkout the documentation to format it
git checkout "$commit_hash" --quiet

# Find all markdown files
md_files=$(find . -name "*.md" -not -path "./.git/*" || true)

if [ -z "$md_files" ]; then
    echo "MarkdownLint: WARNING - No markdown files found" >&2
    # Still output a CodeCommit artefact (no changes made)
    cat <<EOF
{
  "artefact_type": "FormattedDocumentation",
  "artefact_payload": "$commit_hash",
  "summary": "No markdown files to format"
}
EOF
    exit 0
fi

echo "MarkdownLint: Found markdown files: $md_files" >&2

# Run markdownlint-cli2 to auto-fix formatting issues
# Note: markdownlint-cli2-fix automatically formats files
for md_file in $md_files; do
    echo "MarkdownLint: Formatting $md_file..." >&2
    # Use markdownlint-cli2-fix for auto-fixing
    markdownlint-cli2-fix "$md_file" > /dev/null 2>&1 || true
done

# Check if any changes were made
if git diff --quiet; then
    echo "MarkdownLint: No formatting changes needed" >&2
    # No changes, return original commit
    new_commit_hash=$commit_hash
else
    echo "MarkdownLint: Committing formatted documentation..." >&2
    # Commit the formatting changes
    git add .

    # Double-check there are still changes after git add
    if git diff --cached --quiet; then
        echo "MarkdownLint: No changes to commit after staging" >&2
        new_commit_hash=$commit_hash
    else
        git commit -m "[holt-agent: MarkdownLint] Formatted markdown documentation

Original commit: $commit_hash" >&2
        new_commit_hash=$(git rev-parse HEAD)
    fi

    echo "MarkdownLint: Committed formatted documentation as $new_commit_hash" >&2
fi

# Always update the original branch and checkout to it, even if no changes
# This preserves the branch for the next agent in the chain
if [ -n "$original_branch" ] && [ "$original_branch" != "HEAD" ]; then
    echo "MarkdownLint: Updating branch $original_branch to point to commit" >&2
    git branch -f "$original_branch" "$new_commit_hash" 2>/dev/null || true
    git checkout "$original_branch" --quiet 2>/dev/null || true
fi

# Output CodeCommit artefact with type "FormattedDocumentation"
cat <<EOF
{
  "artefact_type": "FormattedDocumentation",
  "artefact_payload": "$new_commit_hash",
  "summary": "$([ "$new_commit_hash" = "$commit_hash" ] && echo "Markdown files already properly formatted" || echo "Formatted markdown documentation with markdownlint-cli2")"
}
EOF
