#!/bin/sh
set -e
echo "Formatter Agent: Received approved recipe. Generating Markdown." >&2
input=$(cat)
commit_hash=$(echo "$input" | jq -r '.target_artefact.payload')

cd /workspace

# Configure git user for commits (required for agents)
git config user.email "agent@holt.example"
git config user.name "Holt Recipe Formatter"

git checkout "$commit_hash" --quiet

# Simple sed script to convert YAML to Markdown
{
    echo "# $(grep 'title:' recipe.yaml | cut -d' ' -f2-)"
    echo ""
    echo "**Prep Time:** $(grep 'prep_time:' recipe.yaml | cut -d' ' -f2-)"
    echo "**Cook Time:** $(grep 'cook_time:' recipe.yaml | cut -d' ' -f2-)"
    echo ""
    echo "## Ingredients"
    grep -A 10 'ingredients:' recipe.yaml | grep -- '- { name:' | sed 's/- { name: "\(.*\)", quantity: "\(.*\)" }/- \2 of \1/'
    echo ""
    echo "## Instructions"
    grep -A 10 'instructions:' recipe.yaml | grep -- '- "' | sed 's/- "\(.*\)"/1. \1/'
} > RECIPE.md

git add RECIPE.md
git commit -m "[holt-agent: Formatter] Generated RECIPE.md" >&2
new_commit_hash=$(git rev-parse HEAD)

echo "Formatter Agent: Committed RECIPE.md as ${new_commit_hash}" >&2

cat <<EOF
{
  "artefact_type": "RecipeMarkdown",
  "artefact_payload": "${new_commit_hash}",
  "summary": "Generated RECIPE.md from recipe.yaml"
}
EOF
