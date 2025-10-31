#!/bin/sh
# TfLint agent - Validates Terraform code best practices and errors
# Tool-based reviewer using official TfLint tool

set -e

input=$(cat)
cd /workspace

echo "TfLint: Starting Terraform linting validation..." >&2

# Extract commit hash from target artefact
commit_hash=$(echo "$input" | jq -r '.target_artefact.payload')

echo "TfLint: Checking commit $commit_hash" >&2

# Checkout the commit to a temporary directory for linting
# We use read-only mode but need files on disk for tflint
temp_dir="/tmp/tflint_$$"
mkdir -p "$temp_dir"

# Extract all .tf files from the commit
tf_files=$(git show --name-only --format="" "$commit_hash" | grep '\.tf$' || true)

if [ -z "$tf_files" ]; then
    echo "TfLint: ERROR - No .tf files found in commit" >&2
    rm -rf "$temp_dir"
    cat <<EOF
{
  "artefact_type": "Review",
  "artefact_payload": "{\"error\": \"No Terraform files found in commit\"}",
  "summary": "Review rejected: No .tf files found"
}
EOF
    exit 0
fi

echo "TfLint: Found Terraform files: $tf_files" >&2

# Extract files to temp directory
for tf_file in $tf_files; do
    file_dir=$(dirname "$tf_file")
    mkdir -p "$temp_dir/$file_dir"
    git show "$commit_hash:$tf_file" > "$temp_dir/$tf_file"
done

# Run tflint in the temp directory
cd "$temp_dir"

echo "TfLint: Initializing tflint..." >&2
tflint --init > /dev/null 2>&1 || true

echo "TfLint: Running linter..." >&2
lint_output=$(tflint --format=compact 2>&1) || lint_exit=$?

cd /workspace
rm -rf "$temp_dir"

# Check if linting found issues
if [ -n "$lint_exit" ] && [ "$lint_exit" != "0" ]; then
    echo "TfLint: REJECTION - Linting issues found" >&2
    echo "TfLint output: $lint_output" >&2

    # Use jq to build proper JSON with escaped strings
    jq -n \
        --arg output "$lint_output" \
        '{
            artefact_type: "Review",
            artefact_payload: ({error: "TfLint found issues", output: $output} | tostring),
            summary: "Review rejected: Terraform linting issues"
        }'
else
    echo "TfLint: APPROVAL - No linting issues found" >&2
    # Empty JSON object signals approval
    cat <<EOF
{
  "artefact_type": "Review",
  "artefact_payload": "{}",
  "summary": "Review approved: Terraform linting passed"
}
EOF
fi
