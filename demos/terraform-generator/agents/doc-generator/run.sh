#!/bin/sh
# DocGenerator agent - Generates README.md documentation from Terraform code
# DUAL-MODE: Uses OpenAI API if OPENAI_API_KEY set, otherwise uses mocked responses

set -e

input=$(cat)
cd /workspace

# Configure git user for commits
git config user.email "docgenerator@holt.demo"
git config user.name "Holt DocGenerator"

# Capture original branch to preserve user's workspace state
original_branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")

# Extract commit hash from target artefact
commit_hash=$(echo "$input" | jq -r '.target_artefact.payload')

echo "DocGenerator: Received TerraformCode commit: $commit_hash" >&2
echo "DocGenerator: Original branch: $original_branch" >&2

# Checkout the Terraform code to read it
git checkout "$commit_hash" --quiet

# Read the Terraform code from the context chain
terraform_code=$(cat main.tf 2>/dev/null || echo "")

if [ -z "$terraform_code" ]; then
    echo "ERROR: Could not read main.tf from commit $commit_hash" >&2
    exit 1
fi

# Determine mode: OpenAI API or mocked
if [ -n "$OPENAI_API_KEY" ]; then
    echo "DocGenerator: Using OpenAI API (gpt-4o-mini)" >&2

    system_prompt="You are a technical documentation writer specializing in Terraform modules. Generate a comprehensive README.md for the provided Terraform code. Include sections for: description, features, usage examples, requirements, inputs, outputs, and important notes. Use proper markdown formatting. Return ONLY the markdown content, no explanations."

    user_prompt="Generate a comprehensive README.md for this Terraform module:

\`\`\`hcl
$terraform_code
\`\`\`"

    # Call OpenAI API
    api_response=$(curl -s https://api.openai.com/v1/chat/completions \
      -H "Authorization: Bearer $OPENAI_API_KEY" \
      -H "Content-Type: application/json" \
      -d @- <<EOF_JSON
{
  "model": "gpt-4o-mini",
  "messages": [
    {
      "role": "system",
      "content": $(echo "$system_prompt" | jq -Rs .)
    },
    {
      "role": "user",
      "content": $(echo "$user_prompt" | jq -Rs .)
    }
  ],
  "temperature": 0.7,
  "max_tokens": 2000
}
EOF_JSON
    ) || {
        echo "ERROR: OpenAI API call failed" >&2
        exit 1
    }

    # Check for API errors
    if echo "$api_response" | jq -e '.error' > /dev/null 2>&1; then
        error_msg=$(echo "$api_response" | jq -r '.error.message')
        echo "ERROR: OpenAI API error: $error_msg" >&2
        exit 1
    fi

    # Parse response
    generated_readme=$(echo "$api_response" | jq -r '.choices[0].message.content') || {
        echo "ERROR: Failed to parse OpenAI response" >&2
        echo "Response: $api_response" >&2
        exit 1
    }

    # Validate non-empty
    if [ -z "$generated_readme" ] || [ "$generated_readme" = "null" ]; then
        echo "ERROR: Empty LLM response" >&2
        exit 1
    fi

    # Write generated README to file
    echo "$generated_readme" > README.md

else
    echo "DocGenerator: Using mocked response (OPENAI_API_KEY not set)" >&2

    # MOCKED LLM RESPONSE: Hardcoded README.md content
    cat > README.md <<'EOF'
# S3 Static Website Hosting Module

Terraform module for provisioning an AWS S3 bucket configured for static website hosting with public access.

## Features

- S3 bucket with website hosting configuration
- Public access configuration for static website serving
- Bucket policy for public read access
- Configurable index and error documents
- Outputs for website endpoint and bucket details

## Usage

```hcl
module "static_website" {
  source = "./path/to/module"

  bucket_name = "my-unique-website-bucket"
}
```

### Custom Index and Error Documents

```hcl
module "static_website" {
  source = "./path/to/module"

  bucket_name     = "my-unique-website-bucket"
  index_document  = "home.html"
  error_document  = "404.html"
}
```

## Requirements

| Name | Version |
|------|---------|
| terraform | >= 1.0 |
| aws | ~> 5.0 |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| bucket_name | Name of the S3 bucket for static website hosting | `string` | n/a | yes |
| index_document | Index document for the website | `string` | `"index.html"` | no |
| error_document | Error document for the website | `string` | `"error.html"` | no |

## Outputs

| Name | Description |
|------|-------------|
| website_endpoint | Website endpoint URL |
| bucket_name | Name of the S3 bucket |
| bucket_arn | ARN of the S3 bucket |

## Important Notes

- This module creates a **publicly accessible** S3 bucket for static website hosting
- Ensure you understand the security implications before deploying
- The bucket policy allows public read access to all objects
- Consider implementing CloudFront for production use cases
- Remember to configure proper DNS records for custom domains

## Example Deployment

After applying this module, upload your website files:

```bash
aws s3 sync ./website-files s3://your-bucket-name
```

Access your website at the endpoint provided in the `website_endpoint` output.

## License

This module is provided as-is for demonstration purposes.

---

*Generated by Holt DocGenerator*
EOF
fi

# Commit the documentation
git add README.md

# Check if there are changes to commit
if git diff --cached --quiet; then
    echo "DocGenerator: No changes to commit (file unchanged)" >&2
    new_commit_hash=$(git rev-parse HEAD)
else
    git commit -m "[holt-agent: DocGenerator] Generated module documentation

Terraform commit: $commit_hash" >&2
    new_commit_hash=$(git rev-parse HEAD)
fi

echo "DocGenerator: Committed documentation as $new_commit_hash" >&2

# Update the original branch to point to our new commit and checkout to it
# This preserves the branch for the next agent in the chain
if [ -n "$original_branch" ] && [ "$original_branch" != "HEAD" ]; then
    echo "DocGenerator: Updating branch $original_branch to point to new commit" >&2
    git branch -f "$original_branch" "$new_commit_hash" 2>/dev/null || true
    git checkout "$original_branch" --quiet 2>/dev/null || true
fi

# Output CodeCommit artefact with type "TerraformDocumentation"
cat <<EOF
{
  "artefact_type": "TerraformDocumentation",
  "artefact_payload": "$new_commit_hash",
  "summary": "Generated comprehensive README.md for Terraform module"
}
EOF
