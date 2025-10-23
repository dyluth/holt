#!/bin/sh
set -e

input=$(cat)
cd /workspace

# Configure git user for commits (required for agents)
git config user.email "agent@holt.example"
git config user.name "Holt Recipe Drafter"

# Check if a "Review" artefact exists in the context chain.
# This indicates we are in a feedback loop.
review_feedback=$(echo "$input" | jq -r '.context_chain[] | select(.type=="Review") | .payload' 2>/dev/null)

if [ -n "$review_feedback" ]; then
    # --- FEEDBACK/REWORK PATH ---
    echo "Drafter Agent: Review feedback detected. Reworking recipe..." >&2
    echo "Feedback received: $review_feedback" >&2

    # Checkout the previous version of the recipe to modify it
    previous_commit=$(echo "$input" | jq -r '.target_artefact.payload')
    git checkout "$previous_commit" --quiet

    # Apply the fix: replace the short instruction with a better one
    sed -i 's/- "Cook."/- "Simmer sauce for 20 minutes."/' recipe.yaml

    git add recipe.yaml
    git commit -m "[holt-agent: Writer] Revised recipe based on feedback" >&2
else
    # --- INITIAL DRAFT PATH ---
    echo "Drafter Agent: No feedback detected. Drafting initial recipe..." >&2
    cat > recipe.yaml <<EOF
title: Spaghetti Bolognese
prep_time: 15 minutes
cook_time: 30 minutes
ingredients:
  - { name: "Ground Beef", quantity: "500g" }
  - { name: "Spaghetti", quantity: "400g" }
  - { name: "Onion", quantity: "1" }
instructions:
  - "Chop the onion."
  - "Brown the beef."
  - "Cook."
EOF

    git add recipe.yaml
    git commit -m "[holt-agent: Writer] Drafted initial recipe for spaghetti" >&2
fi

# Output the new commit hash for the CodeCommit artefact
commit_hash=$(git rev-parse HEAD)
echo "Drafter Agent: Committed changes as ${commit_hash}" >&2

cat <<EOF
{
  "artefact_type": "RecipeYAML",
  "artefact_payload": "${commit_hash}",
  "summary": "Created/updated recipe.yaml"
}
EOF
