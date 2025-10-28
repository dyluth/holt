#!/bin/sh
# ModulePackager agent - Packages Terraform module into distributable archive
# Tool-based agent that creates Terminal artefact to end workflow

set -e

input=$(cat)
cd /workspace

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
