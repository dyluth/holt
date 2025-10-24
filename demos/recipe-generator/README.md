# Recipe Generator Demo

This demo showcases **Phase 3.7** features of Holt:
- **Simplified agent identity** (M3.7: agent key IS the role)
- **Dynamic bidding** with `bid_script` (agents decide bid types based on artefact properties)
- **M3.3 feedback workflow** (automatic review → rejection → rework → approval cycle)
- **Multi-agent coordination** (3 agents collaborate with different roles)

## What This Demo Shows

### Scenario

1.  A user provides a goal to create a recipe
2.  **Writer agent**:
    - Bids `exclusive` on `GoalDefined` artefact
    - Creates draft `recipe.yaml` with deliberately vague instruction ("Cook.")
    - Commits as version 1
3.  **Validator agent**:
    - Bids `review` on `RecipeYAML` artefact
    - Finds vague instruction, rejects with feedback
4.  **Orchestrator** creates feedback claim:
    - Assigns back to Writer (no bidding)
    - Includes Review artefact as context
5.  **Writer agent** receives feedback:
    - Reworks recipe with better instruction
    - Commits as version 2
6.  **Validator agent** reviews version 2:
    - Finds detailed instruction, approves (empty Review payload)
7.  **Formatter agent**:
    - Bids `claim` on approved `RecipeYAML`
    - Converts YAML to `RECIPE.md`
    - Creates Terminal artefact

### Key Technical Features Demonstrated

- **Dynamic bid scripts**: Each agent uses a shell script to inspect artefact type and decide bid
- **Structural type mapping**: Review artefacts automatically map to `StructuralTypeReview` (prevents infinite claim loops)
- **Feedback loop**: M3.3 `pending_assignment` claims bypass bidding for rework
- **Version management**: Drafter creates v1 → v2 with same logical_id
- **Phase coordination**: Review → Parallel phases with proper claim lifecycle

### How to Run

1.  **Build the agent images** (run from the root of the `holt` project):
    ```bash
    make -f demos/recipe-generator/Makefile build-demo-recipe
    ```

2.  **Create a clean directory** for the demo run:
    ```bash
    mkdir /tmp/holt-recipe-demo && cd /tmp/holt-recipe-demo
    ```

3.  **Initialize a Git repository**:
    ```bash
    git init && git commit --allow-empty -m "Initial commit"
    ```

4.  **Initialize Holt**:
    ```bash
    # Make sure 'holt' is in your PATH
    holt init
    ```

5.  **Copy the demo's configuration**:
    ```bash
    # Replace <holt-repo> with the path to your Holt repository
    cp <holt-repo>/demos/recipe-generator/holt.yml .
    ```

6.  **Start the Holt instance**:
    ```bash
    holt up
    ```

7.  **Run the workflow and watch the agents collaborate**:
    ```bash
    holt forage --watch --goal "Create a recipe for a classic spaghetti bolognese"
    ```

8.  **Inspect the results**:
    ```bash
    ls -l               # Should show recipe.yaml and RECIPE.md
    git log --oneline   # Should show 4+ commits
    cat RECIPE.md       # View the final formatted recipe
    holt hoard          # View all artefacts created
    ```

### Automated Testing

Run the integration test to verify the complete workflow:

```bash
cd <holt-repo>
./demos/recipe-generator/test_workflow.sh
```

## Expected Output

### Successful Workflow

When running with `--watch`, you should see:

```
[12:51:07] ✨ Artefact created: by=user, type=GoalDefined, id=...
[12:51:07] ⏳ Claim created: claim=..., artefact=..., status=pending_review
[12:51:07] 🙋 Bid submitted: agent=recipe-validator, claim=..., type=review
[12:51:07] 🙋 Bid submitted: agent=recipe-formatter, claim=..., type=claim
[12:51:07] 🙋 Bid submitted: agent=recipe-drafter, claim=..., type=exclusive
[12:51:07] 🏆 Claim granted: agent=recipe-validator, claim=..., type=review
[12:51:08] ✨ Artefact created: by=Validator, type=Review, id=... (rejection)
[12:51:08] 🏆 Claim granted: agent=recipe-drafter, claim=..., type=exclusive (feedback)
[12:51:09] ✨ Artefact created: by=Writer, type=RecipeYAML, id=... (v2)
[12:51:09] 🏆 Claim granted: agent=recipe-validator, claim=..., type=review
[12:51:10] ✨ Artefact created: by=Validator, type=Review, id=... (approval)
[12:51:10] 🏆 Claim granted: agent=recipe-formatter, claim=..., type=claim
[12:51:11] ✨ Artefact created: by=Formatter, type=RecipeMarkdown, id=...
```

### Created Files

- **recipe.yaml**: YAML format recipe (final version after rework)
- **RECIPE.md**: Human-readable markdown format

### Git History

```bash
$ git log --oneline
abc1234 [holt-agent: Formatter] Generated RECIPE.md
def5678 [holt-agent: Writer] Revised recipe based on feedback
ghi9012 [holt-agent: Writer] Drafted initial recipe for spaghetti
jkl3456 Initial commit
```

## Troubleshooting

### Issue: Agents bidding on all artefacts (infinite loop)

**Symptoms**:
- Watch output shows agents submitting bids for `Review` artefacts
- Claims keep getting created for Review artefacts
- Workflow never completes

**Cause**: One of two problems:
1. Missing `jq` in agent containers (bid scripts fail silently)
2. Review artefacts not mapping to `StructuralTypeReview`

**Fix**:
```bash
# 1. Verify jq is installed in agent containers
docker run recipe-validator-agent:latest jq --version
docker run recipe-formatter-agent:latest jq --version

# If jq is missing, rebuild with updated Dockerfiles
make -f demos/recipe-generator/Makefile build-demo-recipe

# 2. Check orchestrator logs for claim creation
holt logs holt-orchestrator | grep "claim_creation_skipped"
# Should see: claim_creation_skipped for structural_type=Review
```

### Issue: Bid scripts returning "ignore" for everything

**Symptoms**:
- All agents bid `ignore` on all claims
- Workflow hangs waiting for bids

**Cause**: Bid script syntax error or missing dependencies

**Fix**:
```bash
# Test bid script manually
echo '{"type":"RecipeYAML"}' | docker run -i recipe-validator-agent:latest /app/bid.sh
# Should output: review

# Check agent logs for errors
holt logs recipe-validator
# Look for: "jq: not found" or other errors
```

### Issue: Drafter doesn't receive feedback

**Symptoms**:
- Validator rejects recipe (creates Review artefact with feedback)
- Original claim terminates (correct)
- No feedback claim created for drafter
- Workflow stops

**Cause**: M3.3 feedback claim creation not working

**Fix**:
```bash
# Check orchestrator logs
holt logs holt-orchestrator | grep "feedback_claim_created"
# Should see: feedback_claim_created with assigned_agent=recipe-drafter

# Check orchestrator version supports M3.3
holt logs holt-orchestrator | grep "M3.3"
```

### Issue: Formatter runs before approval

**Symptoms**:
- Formatter creates RECIPE.md from rejected recipe (v1)
- Validator never gets to review

**Cause**: Formatter bidding `exclusive` instead of `claim`

**Fix**:
```bash
# Check formatter bid script
cat demos/recipe-generator/agents/formatter/bid.sh
# Should output "claim" for RecipeYAML (not "exclusive")

# Rebuild if needed
make -f demos/recipe-generator/Makefile build-demo-recipe
```

### Issue: Config validation error about bidding_strategy

**Symptoms**:
```
Error: agent 'recipe-drafter': bidding_strategy is required
```

**Cause**: Using old Holt version without M3.6 support

**Fix**:
```bash
# Verify Holt supports M3.6 (optional bidding_strategy)
holt up 2>&1 | grep -i "M3.6\|bid_script"

# Or add fallback bidding_strategy to holt.yml
# agents:
#   recipe-drafter:
#     bid_script: ["/app/bid.sh"]
#     bidding_strategy: "exclusive"  # Fallback if script fails
```

### Debugging Tips

**1. Enable verbose logging**:
```bash
# Check all agent logs
holt logs recipe-drafter
holt logs recipe-validator
holt logs recipe-formatter
holt logs holt-orchestrator
```

**2. Inspect blackboard state**:
```bash
# View all artefacts
holt hoard

# Count artefacts by type
holt hoard | grep -c "RecipeYAML"  # Should be 2 (v1 rejected, v2 approved)
holt hoard | grep -c "Review"       # Should be 2 (rejection, approval)
```

**3. Check bid decisions**:
```bash
# Watch for bid submissions
holt watch | grep "🙋 Bid submitted"
# Each agent should bid appropriately based on artefact type
```

**4. Verify git state**:
```bash
# Ensure clean workspace before starting
git status
# Should show: working tree clean

# Check commits were made by agents
git log --oneline
# Should show commits with [holt-agent: ...] messages
```

## Understanding the Bid Scripts

Each agent has a simple shell script that decides its bid based on artefact type:

**Drafter (recipe-drafter/bid.sh)**:
```bash
#!/bin/sh
artefact_type=$(echo "$input" | jq -r '.type')
if [ "$artefact_type" = "GoalDefined" ]; then
  echo "exclusive"  # Only drafter creates initial recipe
else
  echo "ignore"     # Ignore everything else
fi
```

**Validator (recipe-validator/bid.sh)**:
```bash
#!/bin/sh
artefact_type=$(echo "$input" | jq -r '.type')
if [ "$artefact_type" = "RecipeYAML" ]; then
  echo "review"  # Review all recipe drafts
else
  echo "ignore"  # Ignore everything else
fi
```

**Formatter (recipe-formatter/bid.sh)**:
```bash
#!/bin/sh
artefact_type=$(echo "$input" | jq -r '.type')
if [ "$artefact_type" = "RecipeYAML" ]; then
  echo "claim"   # Format after review phase completes
else
  echo "ignore"  # Ignore everything else
fi
```

## Design Philosophy

### Artefacts vs Git Commits

**Important architectural clarification:**

- **Artefacts** are stored in Redis (metadata + payload)
- **Git commits** are referenced BY artefacts (payload = commit hash)
- Artefacts are NOT commits; they POINT TO commits

**In this demo:**
1. Drafter creates recipe.yaml → commits → artefact payload is the git hash
2. Validator reads from that commit hash → creates Review artefact (no git commit)
3. Drafter reworks → commits v2 → artefact payload is new git hash
4. Formatter creates RECIPE.md → commits → artefact payload is final git hash

**Why commit intermediate work (recipe.yaml)?**
- Demonstrates version management (v1 rejected → v2 approved)
- Shows git-centric workflow pattern
- Provides reproducibility (can checkout any version)
- Creates complete audit trail in git history

**Workspace modes:**
- **Drafter**: `rw` (read-write) - gets exclusive claims, commits recipe.yaml
- **Formatter**: `rw` (read-write) - gets parallel claims, commits RECIPE.md
- **Validator**: `ro` (read-only) - reads via `git show`, never modifies workspace

**Note:**
- Git commits happen with **exclusive** or **parallel** claims that have `rw` workspace access
- The formatter uses parallel claims (bids `claim`) because multiple formatters could theoretically run on different approved recipes
- The drafter uses exclusive claims because only one agent should draft the initial recipe
- Review agents should **never** modify the workspace (always `ro` mode)

### Complete Artefact Flow

Here's what gets stored in Redis vs Git:

**Artefacts in Redis:**
1. `GoalDefined` - payload: "Create a recipe..." (user input, no git commit)
2. `RecipeYAML` v1 - payload: `abc123` (git commit hash)
3. `Review` (rejection) - payload: `{"issue": "...", "line": "..."}` (JSON, no git commit)
4. `RecipeYAML` v2 - payload: `def456` (git commit hash, same logical_id as v1)
5. `Review` (approval) - payload: `{}` (empty JSON, no git commit)
6. `RecipeMarkdown` - payload: `ghi789` (git commit hash)

**Git Commits:**
```bash
$ git log --oneline
ghi789 [holt-agent: Formatter] Generated RECIPE.md      ← Artefact 6 points here
def456 [holt-agent: Writer] Revised recipe v2           ← Artefact 4 points here
abc123 [holt-agent: Writer] Drafted initial recipe v1    ← Artefact 2 points here
initial Initial commit
```

**Key Insight:** Only 3 of 6 artefacts involve git commits. Review artefacts live entirely in Redis.

### Detached HEAD State

After the demo runs, your workspace will be in detached HEAD state because agents checkout specific commits. This is expected behavior.

To clean up:
```bash
git checkout main           # Return to main branch
git clean -fd               # Remove untracked files (optional)
```

## Agent Development Notes

These demo agents demonstrate best practices for Holt agent development:

**1. Git configuration in scripts**:
```bash
# Required at the start of any agent that makes commits
git config user.email "agent@holt.example"
git config user.name "Holt Agent Name"
```

**2. Stdout vs Stderr**:
- **Stdout**: ONLY for the final JSON output that the pup reads
- **Stderr**: ALL logging, debug output, and git command output
```bash
# Correct: Git output to stderr
git commit -m "message" >&2

# Correct: Debug logging to stderr
echo "Processing artefact..." >&2

# Correct: Final JSON to stdout (no >&2)
cat <<EOF
{
  "artefact_type": "RecipeYAML",
  "artefact_payload": "${commit_hash}",
  "summary": "Created recipe"
}
EOF
```

**3. Bid scripts**:
- Receive full artefact as JSON on stdin
- Output single bid type to stdout: `review`, `claim`, `exclusive`, or `ignore`
- Use `jq` for JSON parsing (ensure it's in Dockerfile)

**4. Structural types**:
- Most agents output `artefact_type` and let pup handle `structural_type`
- `artefact_type: "Review"` automatically maps to `StructuralTypeReview`
- This prevents Review artefacts from creating new claims (avoiding infinite loops)

## Further Reading

- **M3.6 Design Document**: `/design/features/phase-3-coordination/M3.6-dynamic-bidding-demo.md`
- **M3.3 Feedback Workflow**: `/design/features/phase-3-coordination/M3.3-automated-feedback-loop.md`
- **Agent Development Guide**: `/docs/agent-development.md`
