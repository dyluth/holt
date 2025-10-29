#!/bin/sh
# ModulePackager agent - Packages Terraform module into distributable archive
# Tool-based agent that creates Terminal artefact to end workflow

set -e

input=$(cat)
cd /workspace

# Capture the original branch before any git operations
# This ensures we return to the user's starting branch, not just 'main'
original_branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")
original_commit=$(git rev-parse HEAD 2>/dev/null || echo "")

# Extract commit hash from target artefact
commit_hash=$(echo "$input" | jq -r '.target_artefact.payload')

echo "ModulePackager: Received FormattedDocumentation commit: $commit_hash" >&2
echo "ModulePackager: Creating final package..." >&2

# Checkout the final state
git checkout "$commit_hash" --quiet

# Verify required files exist
if [ ! -f "main.tf" ]; then
    echo "ModulePackager: ERROR - main.tf not found" >&2
    cat <<EOF
{
  "structural_type": "Failure",
  "artefact_payload": "Missing required file: main.tf",
  "summary": "Packaging failed: main.tf not found"
}
EOF
    exit 0
fi

if [ ! -f "README.md" ]; then
    echo "ModulePackager: WARNING - README.md not found, continuing anyway..." >&2
fi

# Create package with all relevant files
package_name="s3-module.tar.gz"

echo "ModulePackager: Packaging files into $package_name..." >&2

# Package main.tf and README.md (and any other .tf files)
tar -czf "$package_name" main.tf README.md *.tf 2>/dev/null || tar -czf "$package_name" main.tf README.md

# Verify package was created
if [ ! -f "$package_name" ]; then
    echo "ModulePackager: ERROR - Failed to create package" >&2
    cat <<EOF
{
  "structural_type": "Failure",
  "artefact_payload": "Failed to create tar.gz package",
  "summary": "Packaging failed: tar command error"
}
EOF
    exit 0
fi

package_size=$(ls -lh "$package_name" | awk '{print $5}')
echo "ModulePackager: Package created successfully ($package_size)" >&2

# List contents for verification
echo "ModulePackager: Package contents:" >&2
tar -tzf "$package_name" | while read -r file; do
    echo "  - $file" >&2
done

# Return workspace to original branch and update it to point to current commit
# This avoids leaving the workspace in detached HEAD state
if [ -n "$original_branch" ] && [ "$original_branch" != "HEAD" ]; then
    # We know the original branch, return to it
    current_commit=$(git rev-parse HEAD)
    git branch -f "$original_branch" "$current_commit" 2>/dev/null
    git checkout "$original_branch" --quiet 2>/dev/null
else
    # Fallback: try to return to main/master if original branch unknown
    current_commit=$(git rev-parse HEAD)
    git branch -f main "$current_commit" 2>/dev/null || git branch -f master "$current_commit" 2>/dev/null
    git checkout main --quiet 2>/dev/null || git checkout master --quiet 2>/dev/null
fi

# Output Terminal artefact with type "PackagedModule"
# This signals workflow completion
cat <<EOF
{
  "artefact_type": "PackagedModule",
  "structural_type": "Terminal",
  "artefact_payload": "$package_name",
  "summary": "Created distributable Terraform module package: $package_name ($package_size)"
}
EOF

echo "ModulePackager: ✅ Workflow complete - Terminal artefact created" >&2
