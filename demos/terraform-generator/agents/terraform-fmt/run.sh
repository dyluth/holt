#!/bin/sh
# TerraformFmt agent - Validates Terraform code formatting
# Tool-based reviewer using official Terraform CLI

set -e

input=$(cat)

echo "TerraformFmt: Starting Terraform formatting validation..." >&2

# Extract commit hash from target artefact
commit_hash=$(echo "$input" | jq -r '.target_artefact.payload')

echo "TerraformFmt: Checking commit $commit_hash" >&2

# Use 'git show' to read files without modifying workspace
# Extract all .tf files from the commit and validate formatting
tf_files=$(git show --name-only --format="" "$commit_hash" | grep '\.tf$' || true)

if [ -z "$tf_files" ]; then
    echo "TerraformFmt: ERROR - No .tf files found in commit" >&2
    cat <<EOF
{
  "artefact_type": "Review",
  "artefact_payload": "{\"error\": \"No Terraform files found in commit\"}",
  "summary": "Review rejected: No .tf files found"
}
EOF
    exit 0
fi

echo "TerraformFmt: Found Terraform files: $tf_files" >&2

# Check formatting for each file
format_issues=""
for tf_file in $tf_files; do
    echo "TerraformFmt: Checking format of $tf_file..." >&2

    # Extract file content and write to temp location
    file_content=$(git show "$commit_hash:$tf_file")
    temp_file="/tmp/check_$$.tf"
    echo "$file_content" > "$temp_file"

    # Run terraform fmt -check (exit code 0 = properly formatted, 3 = needs formatting)
    if ! terraform fmt -check "$temp_file" > /dev/null 2>&1; then
        format_issues="$format_issues\n- $tf_file needs formatting"
        echo "TerraformFmt: ❌ $tf_file needs formatting" >&2
    else
        echo "TerraformFmt: ✓ $tf_file is properly formatted" >&2
    fi

    rm -f "$temp_file"
done

# If any formatting issues found, reject with feedback
if [ -n "$format_issues" ]; then
    echo "TerraformFmt: REJECTION - Formatting issues found" >&2
    # Use jq to build proper JSON with escaped strings
    jq -n \
        --arg issues "$format_issues" \
        '{
            artefact_type: "Review",
            artefact_payload: ({error: "Formatting issues found", files: $issues} | tostring),
            summary: "Review rejected: Terraform formatting issues"
        }'
else
    echo "TerraformFmt: APPROVAL - All Terraform files properly formatted" >&2
    # Empty JSON object signals approval
    cat <<EOF
{
  "artefact_type": "Review",
  "artefact_payload": "{}",
  "summary": "Review approved: Terraform formatting validated"
}
EOF
fi
