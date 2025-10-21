# Example Git Agent

A reference implementation for Holt agents that produce CodeCommit artefacts through Git workflow integration.

## Purpose

This agent demonstrates the canonical pattern for code-generating agents in Holt:
1. Receive context via stdin JSON
2. Create or modify files in the Git workspace
3. Commit changes with proper metadata
4. Return commit hash as CodeCommit artefact

## What It Does

The git agent:
- **Reads target artefact payload** to determine filename to create (e.g., "hello.txt")
- **Creates file** in `/workspace` with timestamped content
- **Executes git workflow**: `git add` → `git commit` with descriptive message
- **Returns commit hash** as CodeCommit artefact payload
- **Validates commit** via pup's git validation (M2.4 feature)

## Building

From the project root directory:

```bash
docker build -t example-git-agent:latest -f agents/example-git-agent/Dockerfile .
```

**Note:** The Dockerfile context must be the project root (`.`) to access pup source code.

## Configuration

Add to your `holt.yml`:

```yaml
version: "1.0"
agents:
  git-agent:
    role: "Git Agent"
    image: "example-git-agent:latest"
    command: ["/app/run.sh"]
    workspace:
      mode: rw  # Read-write required for git commits
```

## Running

```bash
# Build the agent image
docker build -t example-git-agent:latest -f agents/example-git-agent/Dockerfile .

# Start Holt instance (launches agent)
holt up

# Trigger workflow with filename as goal
holt forage --goal "my-file.txt"

# View agent logs
holt logs git-agent

# View artefacts (should see GoalDefined → CodeCommit chain)
holt hoard
```

## Git Workflow Pattern

### 1. Agent receives stdin JSON

```json
{
  "claim_type": "exclusive",
  "target_artefact": {
    "id": "artefact-uuid",
    "type": "GoalDefined",
    "payload": "my-file.txt",
    "structural_type": "Standard",
    "version": 1,
    "logical_id": "logical-uuid",
    "source_artefacts": [],
    "produced_by_role": "user"
  },
  "context_chain": []
}
```

### 2. Agent creates file in workspace

```bash
cd /workspace
cat > my-file.txt <<EOF
# Content here
EOF
```

### 3. Agent commits with metadata

```bash
git add my-file.txt
git commit -m "[holt-agent: git-agent] Created my-file.txt

Claim-ID: artefact-uuid"
```

**Commit Message Format** (recommended):
```
[holt-agent: {agent-role}] {summary}

Claim-ID: {claim-id}
```

This format provides:
- **Prefix** identifies Holt-generated commits
- **Agent role** shows which agent made the change
- **Claim ID** enables audit trail back to blackboard

### 4. Agent extracts commit hash

```bash
commit_hash=$(git rev-parse HEAD)
# Returns: abc123def456...
```

### 5. Agent outputs CodeCommit JSON

```json
{
  "artefact_type": "CodeCommit",
  "artefact_payload": "abc123def456...",
  "summary": "Created my-file.txt and committed as abc123def456..."
}
```

### 6. Pup validates commit exists (M2.4)

```bash
git cat-file -e abc123def456...
# Exit code 0 = valid commit
# Exit code != 0 = Failure artefact created
```

## Tool Contract Details

### Input (stdin JSON)

**Required fields:**
- `claim_type`: "exclusive" (single-agent Phase 2)
- `target_artefact`: Full artefact object with payload containing filename
- `context_chain`: Array of historical artefacts (empty for root goals)

**Accessing fields in shell:**
```bash
# Extract filename from payload
filename=$(echo "$input" | grep -o '"payload":"[^"]*"' | head -1 | cut -d'"' -f4)

# Extract claim ID for commit message
claim_id=$(echo "$input" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
```

### Output (stdout JSON)

**Required fields:**
- `artefact_type`: "CodeCommit" (triggers git validation)
- `artefact_payload`: Git commit hash from `git rev-parse HEAD`
- `summary`: Human-readable description

**Error handling:**
- Exit code != 0 → Failure artefact created
- Invalid JSON → Failure artefact created
- Invalid commit hash → Failure artefact created (git validation fails)

## Context Chain Usage

For more sophisticated agents, the `context_chain` provides historical context:

```bash
# Extract context_chain from stdin
context=$(echo "$input" | jq -r '.context_chain')

# Iterate through historical artefacts
echo "$context" | jq -c '.[]' | while read -r artefact; do
  type=$(echo "$artefact" | jq -r '.type')
  payload=$(echo "$artefact" | jq -r '.payload')
  echo "Previous work: $type with payload: $payload" >&2
done
```

**Context chain characteristics (M2.4):**
- Chronologically ordered (oldest → newest)
- Filtered to Standard and Answer artefacts only
- Uses thread tracking (latest versions)
- Empty array `[]` for root artefacts

## Derivative Artefacts

**Important concept:** When this agent executes, it creates a **derivative artefact**, not an evolutionary version.

**Derivative** (this agent's pattern):
- New `logical_id` (fresh UUID)
- `version` = 1 (first version of new work product)
- `source_artefacts` = [target_artefact.id] (provenance chain)
- Different `type` (GoalDefined → CodeCommit)

**Example:**
```
GoalDefined (logical_id: A, version: 1)
    ↓
CodeCommit (logical_id: B, version: 1, source_artefacts: [A])
```

## Workspace Requirements

**Prerequisites:**
- Git repository must be initialized (`git init`)
- Working directory must be clean (no uncommitted changes)
- Repository must have initial commit

**Validation:**

Holt validates workspace before launching agents:
```bash
holt up  # Checks: .git exists, git status --porcelain is empty
```

**Workspace mount:**
- Read-write mount required for git commits
- Agent's working directory: `/workspace` (repository root)
- All git commands execute in `/workspace`

## Advanced Patterns

### Multi-file Changes

```bash
# Create multiple files
echo "content1" > file1.txt
echo "content2" > file2.txt

# Add all changes
git add .

# Single commit for atomic change
git commit -m "[holt-agent: git-agent] Created multiple files

Claim-ID: $claim_id"
```

### Conditional Logic Based on Context

```bash
# Check if previous CodeCommit exists in context
if echo "$input" | grep -q '"type":"CodeCommit"'; then
  echo "Building on previous code commit..." >&2
  # Modify existing files
else
  echo "Starting fresh..." >&2
  # Create new files
fi
```

### Error Handling

```bash
set -e  # Exit on any error

# Validate filename
if [ -z "$filename" ]; then
  echo "Error: No filename provided" >&2
  exit 1  # Pup will create Failure artefact
fi

# Check file doesn't already exist
if [ -f "$filename" ]; then
  echo "Error: File $filename already exists" >&2
  exit 1
fi
```

## Troubleshooting

### Agent commits but returns invalid hash

**Problem:** Agent outputs valid JSON but commit hash doesn't exist

**Solution:** Ensure `git rev-parse HEAD` runs AFTER `git commit`

```bash
# Wrong order
commit_hash=$(git rev-parse HEAD)  # Gets OLD commit
git commit -m "message"

# Correct order
git commit -m "message"
commit_hash=$(git rev-parse HEAD)  # Gets NEW commit
```

### Pup creates Failure artefact: "commit does not exist"

**Problem:** Git validation fails in pup

**Possible causes:**
1. Agent returned hash from wrong repository
2. Workspace mount not configured correctly
3. Agent didn't actually commit (dry-run mode)

**Debug:**
```bash
# Check agent logs
holt logs git-agent

# Verify workspace mount
docker inspect holt-{instance}-agent-git-agent | grep -A 10 Mounts

# Check git history in workspace
cd /path/to/workspace && git log --oneline
```

### Permission denied when committing

**Problem:** Agent can't write to workspace

**Solution:** Verify workspace mode is `rw` in holt.yml:
```yaml
workspace:
  mode: rw  # Not ro (read-only)
```

## Real-World Usage

This example agent is intentionally simple (hardcoded file content). Real agents would:

1. **Call LLMs** to generate code based on context
2. **Run linters/formatters** before committing
3. **Execute tests** to validate changes
4. **Parse complex specifications** from artefact payloads
5. **Handle multi-step workflows** (design → implement → test)

**Example LLM-based agent pattern:**
```bash
# Extract requirements from context_chain
requirements=$(echo "$input" | jq -r '.context_chain[] | select(.type=="Requirements") | .payload')

# Call LLM API to generate code
generated_code=$(curl -X POST https://api.anthropic.com/v1/messages \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  -d "{ \"prompt\": \"$requirements\" }" | jq -r '.code')

# Write generated code to file
echo "$generated_code" > implementation.go

# Commit result
git add implementation.go
git commit -m "[holt-agent: code-generator] Implemented from requirements"
```

## Further Reading

- **Tool Contract Specification**: See `docs/agent-development.md`
- **Context Assembly Algorithm**: See M2.4 design document
- **Error Handling Patterns**: See `docs/troubleshooting.md`
- **System Architecture**: See `PROJECT_CONTEXT.md`
