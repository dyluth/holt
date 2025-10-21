# Holt Agent Development Guide

**Target Audience:** Developers building custom agents for Holt workflows

**Prerequisites:**
- Understanding of Docker and containerization
- Familiarity with Git workflows
- Basic knowledge of JSON and stdin/stdout patterns

---

## Table of Contents

1. [Overview](#overview)
2. [Agent Architecture](#agent-architecture)
3. [Tool Contract Specification](#tool-contract-specification)
4. [Building Your First Agent](#building-your-first-agent)
5. [Git Workflow Pattern](#git-workflow-pattern)
6. [Context Chain Usage](#context-chain-usage)
7. [Derivative Artefacts](#derivative-artefacts)
8. [Error Handling](#error-handling)
9. [Advanced Patterns](#advanced-patterns)
10. [Testing Your Agent](#testing-your-agent)
11. [Examples](#examples)

---

## Overview

Holt agents are specialized, tool-equipped containers that execute work in response to claims on the blackboard. Each agent:

- Runs as a Docker container with the **agent pup** binary as entrypoint
- Executes a **tool script** (your custom logic) when work is assigned
- Communicates via **stdin/stdout JSON contract**
- Creates **immutable artefacts** as work products
- Maintains complete **audit trail** of all actions

**Key Principle:** Agents are small, single-purpose components that do one thing excellently.

---

## Agent Architecture

```
┌────────────────────────────────────────┐
│  your-agent container                  │
│                                        │
│  ┌──────────────────────────────────┐ │
│  │  Agent Pup (entrypoint)          │ │
│  │  ┌────────────────────────────┐  │ │
│  │  │  Claim Watcher             │  │ │
│  │  │  - Subscribe to claims     │  │ │
│  │  │  - Submit bids             │  │ │
│  │  └────────────────────────────┘  │ │
│  │  ┌────────────────────────────┐  │ │
│  │  │  Work Executor             │  │ │
│  │  │  - Assemble context        │  │ │
│  │  │  - Execute tool script     │  │ │
│  │  │  - Create artefacts        │  │ │
│  │  └────────────────────────────┘  │ │
│  └──────────────────────────────────┘ │
│                                        │
│  ┌──────────────────────────────────┐ │
│  │  Your Tool Script                │ │
│  │  (e.g., /app/run.sh)             │ │
│  │                                  │ │
│  │  - Reads JSON from stdin        │ │
│  │  - Performs work (LLM, git,     │ │
│  │    analysis, code generation)    │ │
│  │  - Writes JSON to stdout        │ │
│  └──────────────────────────────────┘ │
│                                        │
│  /workspace (mounted Git repo)        │
└────────────────────────────────────────┘
```

**Separation of Concerns:**
- **Agent pup** handles Holt orchestration (bidding, context, artefacts)
- **Your tool script** implements domain-specific logic (what makes your agent unique)

---

## Tool Contract Specification

Your agent tool script must follow a strict stdin/stdout JSON contract.

### Input: JSON on stdin

When the pup executes your tool script, it passes a JSON object via stdin:

```json
{
  "claim_type": "exclusive",
  "target_artefact": {
    "id": "artefact-uuid",
    "logical_id": "logical-uuid",
    "version": 1,
    "structural_type": "Standard",
    "type": "GoalDefined",
    "payload": "Build user authentication",
    "source_artefacts": [],
    "produced_by_role": "user"
  },
  "context_chain": [
    {
      "id": "previous-uuid",
      "type": "DesignSpec",
      "payload": "REST API with JWT...",
      "version": 1,
      ...
    }
  ]
}
```

**Input Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `claim_type` | string | Type of claim: "exclusive", "claim", or "review" (Phase 2: always "exclusive") |
| `target_artefact` | object | The artefact your agent is processing |
| `context_chain` | array | Historical context (chronological, oldest → newest) |

**target_artefact Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique artefact ID (UUID) |
| `logical_id` | string | Groups versions of same logical work |
| `version` | int | Version number (1-indexed) |
| `structural_type` | string | "Standard", "Review", "Question", "Answer", "Failure", "Terminal" |
| `type` | string | User-defined type (e.g., "CodeCommit", "DesignSpec") |
| `payload` | string | Main content (varies by type) |
| `source_artefacts` | array | IDs of artefacts this was derived from |
| `produced_by_role` | string | Agent role or "user" |

**context_chain Array:**
- Populated via BFS traversal of source_artefacts graph
- Filtered to include "Standard", "Answer", and "Review" artefacts (M3.3+)
- Uses thread tracking to ensure latest versions
- Empty array `[]` for root artefacts (no sources)
- Provides complete historical context for informed decisions
- **M3.3**: Review artefacts included for feedback-based iteration

### Output: JSON on stdout

Your tool script must output **exactly ONE** JSON object to stdout:

```json
{
  "artefact_type": "CodeCommit",
  "artefact_payload": "abc123def456...",
  "summary": "Implemented user authentication endpoint"
}
```

**Output Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `artefact_type` | string | Yes | Type of artefact being created (e.g., "CodeCommit", "DesignSpec") |
| `artefact_payload` | string | Yes | Main content (commit hash, JSON data, text) |
| `summary` | string | Yes | Human-readable description of work performed |
| `structural_type` | string | No | Defaults to "Standard" (can override to "Terminal" to end workflow) |

**Special Artefact Types:**

- **`CodeCommit`**: Payload must be a git commit hash. Pup validates commit exists via `git cat-file -e <hash>`. If validation fails, Failure artefact created.
- **`Terminal`**: Set `structural_type: "Terminal"` to signal workflow completion. No further processing.

**Error Output:**

For errors, either:
1. **Exit with non-zero code**: Pup creates Failure artefact with stderr output
2. **Output structural_type="Failure"**: Explicit failure with custom error message

Example:
```json
{
  "structural_type": "Failure",
  "artefact_payload": "Could not connect to database: timeout after 30s",
  "summary": "Database connection failed"
}
```

---

## Building Your First Agent

### Step 1: Create Agent Directory

```bash
mkdir -p agents/my-agent
cd agents/my-agent
```

### Step 2: Write Tool Script

Create `run.sh`:

```bash
#!/bin/sh
# Simple echo agent that processes any goal

set -e  # Exit on error

# Read input from stdin
input=$(cat)

# Log to stderr (visible in holt logs)
echo "Processing claim..." >&2

# Parse target artefact payload
goal=$(echo "$input" | grep -o '"payload":"[^"]*"' | head -1 | cut -d'"' -f4)

echo "Goal: $goal" >&2

# Perform work (this agent just echoes)
result="Processed: $goal"

# Output success JSON to stdout
cat <<EOF
{
  "artefact_type": "ProcessedGoal",
  "artefact_payload": "$result",
  "summary": "Successfully processed goal"
}
EOF
```

Make it executable:
```bash
chmod +x run.sh
```

### Step 3: Create Dockerfile

Create `Dockerfile`:

```dockerfile
# Build stage - compile agent pup
FROM golang:1.24-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY cmd/pup ./cmd/pup
COPY internal/pup ./internal/pup
COPY pkg/blackboard ./pkg/blackboard

RUN CGO_ENABLED=0 GOOS=linux go build -o pup ./cmd/pup

# Runtime stage
FROM alpine:latest

# Install any tools your agent needs (jq, git, etc.)
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy pup binary
COPY --from=builder /build/pup /app/pup

# Copy your tool script
COPY agents/my-agent/run.sh /app/run.sh
RUN chmod +x /app/run.sh

# Run as non-root
RUN adduser -D -u 1000 agent
USER agent

ENTRYPOINT ["/app/pup"]
```

### Step 4: Configure in holt.yml

Add agent to your project's `holt.yml`:

```yaml
version: "1.0"
agents:
  my-agent:
    role: "My Agent"
    image: "my-agent:latest"
    command: ["/app/run.sh"]
    workspace:
      mode: ro  # Read-only (use "rw" if agent needs to write files)

services:
  redis:
    image: redis:7-alpine
```

### Step 5: Build and Test

```bash
# Build Docker image (from project root)
docker build -t my-agent:latest -f agents/my-agent/Dockerfile .

# Start Holt
holt up

# Submit work
holt forage --goal "test my agent"

# View logs
holt logs my-agent

# View results
holt hoard
```

---

## Git Workflow Pattern

For agents that generate or modify code, the **Git workflow pattern** is essential.

### Example: Git Agent Implementation

```bash
#!/bin/sh
# Git agent that creates files and commits them

set -e

# Read input
input=$(cat)

# Parse filename from payload
filename=$(echo "$input" | grep -o '"payload":"[^"]*"' | head -1 | cut -d'"' -f4)

# Parse claim ID for commit message
claim_id=$(echo "$input" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

echo "Creating file: $filename" >&2

# Navigate to workspace
cd /workspace

# Create file
cat > "$filename" <<EOF
# Generated by Holt

This file was created as part of a workflow.
EOF

# Git workflow
git add "$filename"
git commit -m "[holt-agent: my-agent] Created $filename

Claim-ID: $claim_id"

# Get commit hash
commit_hash=$(git rev-parse HEAD)

echo "Committed as: $commit_hash" >&2

# Output CodeCommit artefact
cat <<EOF
{
  "artefact_type": "CodeCommit",
  "artefact_payload": "$commit_hash",
  "summary": "Created $filename and committed"
}
EOF
```

### Recommended Commit Message Format

```
[holt-agent: {agent-role}] {summary}

Claim-ID: {claim-id}
```

**Benefits:**
- `[holt-agent: ...]` prefix identifies Holt-generated commits
- Agent role shows which agent made the change
- Claim ID enables audit trail back to blackboard

### Workspace Configuration

For Git agents, set `workspace.mode: rw` in holt.yml:

```yaml
agents:
  git-agent:
    workspace:
      mode: rw  # Read-write required for commits
```

### Git Validation

When you output a `CodeCommit` artefact, the pup automatically validates:

```bash
git cat-file -e <commit-hash>
```

If validation fails (commit doesn't exist), a Failure artefact is created instead.

**Important:** Ensure `git commit` runs BEFORE `git rev-parse HEAD`:

```bash
# Correct order
git commit -m "message"
commit_hash=$(git rev-parse HEAD)  # Gets NEW commit

# Wrong order
commit_hash=$(git rev-parse HEAD)  # Gets OLD commit
git commit -m "message"
```

---

## Context Chain Usage

The `context_chain` array provides historical context for your agent to make informed decisions.

### Basic Usage: Accessing Context

```bash
#!/bin/sh
input=$(cat)

# Extract context chain (requires jq)
context=$(echo "$input" | jq -r '.context_chain')

# Iterate through historical artefacts
echo "$context" | jq -c '.[]' | while read -r artefact; do
  type=$(echo "$artefact" | jq -r '.type')
  payload=$(echo "$artefact" | jq -r '.payload')

  echo "Previous work: $type" >&2
  echo "Content: $payload" >&2
done
```

### Context Chain Characteristics

- **Chronological order**: Oldest → Newest
- **Filtered**: "Standard", "Answer", and "Review" artefacts included (M3.3+)
- **Latest versions**: Uses thread tracking (logical_id → max version)
- **Empty for root**: `[]` if target_artefact has no source_artefacts
- **Depth limit**: Maximum 10 levels (safety valve)
- **M3.3**: Review artefacts provide feedback for iterative refinement

### Example: Building on Previous Work

```bash
#!/bin/sh
input=$(cat)

# Check if previous CodeCommit exists in context
if echo "$input" | jq -e '.context_chain[] | select(.type=="CodeCommit")' > /dev/null; then
  echo "Building on previous code commit..." >&2
  previous_commit=$(echo "$input" | jq -r '.context_chain[] | select(.type=="CodeCommit") | .payload' | tail -1)

  # Checkout previous commit
  cd /workspace
  git checkout $previous_commit

  # Make incremental changes
  echo "// Additional code" >> existing-file.go
else
  echo "Starting fresh implementation..." >&2
  # Create new files
fi
```

---

## Derivative Artefacts

**Critical Concept:** When your agent executes work, it creates a **derivative artefact**, not an evolutionary version.

### Derivative vs Evolutionary

**Derivative** (your agent creates):
- **New** `logical_id` (fresh UUID)
- `version` = 1 (first version of new work product)
- `source_artefacts` = [target_artefact.id] (provenance chain)
- **Different** `type` (e.g., GoalDefined → CodeCommit)

**Evolutionary** (automatic in M3.3 feedback loops):
- **Same** `logical_id`
- `version` incremented (v1 → v2)
- Same `type` (e.g., CodeCommit v1 → CodeCommit v2)
- **The pup handles this automatically** when reworking based on review feedback

### Example Flow

```
User:
  GoalDefined (logical_id: A, version: 1, type: "GoalDefined")
      ↓
Your Agent:
  CodeCommit (logical_id: B, version: 1, type: "CodeCommit")
  source_artefacts: [A]
      ↓
Another Agent:
  TestResults (logical_id: C, version: 1, type: "TestResults")
  source_artefacts: [B]
```

**The pup handles this automatically** - you just output the artefact_type and payload.

---

## Automatic Version Management (M3.3+)

**New in Phase 3 M3.3:** The Pup now automatically manages versioning for feedback-based iterations, so your agent code remains simple and unaware of version management.

### How It Works

When a reviewer rejects your agent's work and provides feedback:

1. **Orchestrator detects review rejection** and creates a **feedback claim**
2. **Your agent is automatically reassigned** (no bidding required)
3. **Pup injects review feedback** into the `context_chain`
4. **Your agent executes** using the same tool script
5. **Pup automatically creates a new version** (v2, v3, etc.) with:
   - Same `logical_id` (preserves thread)
   - Incremented `version` number
   - Same `type` as original artefact
   - Updated `source_artefacts` (includes original + review artefacts)

### What Your Agent Sees

**Feedback Claim Input:**
```json
{
  "claim_type": "exclusive",
  "target_artefact": {
    "id": "original-uuid",
    "logical_id": "thread-uuid",
    "version": 1,
    "type": "CodeCommit",
    "payload": "abc123...",
    ...
  },
  "context_chain": [
    {
      "type": "GoalDefined",
      "payload": "Build authentication"
    },
    {
      "type": "Review",
      "payload": "{\"issue\": \"needs tests\", \"severity\": \"high\"}",
      "structural_type": "Review"
    }
  ]
}
```

**What Your Agent Outputs (unchanged):**
```json
{
  "artefact_type": "CodeCommit",
  "artefact_payload": "def456...",
  "summary": "Added tests per review feedback"
}
```

**What Pup Creates Automatically:**
- `logical_id`: "thread-uuid" (same as v1)
- `version`: 2 (automatically incremented)
- `type`: "CodeCommit" (preserved from v1)
- `source_artefacts`: ["original-uuid", "review-uuid"]

### Key Benefits

1. **Agents stay simple** - no version management logic needed
2. **Automatic thread preservation** - all versions linked via logical_id
3. **Complete audit trail** - source_artefacts shows provenance chain
4. **Review feedback in context** - accessible via context_chain
5. **Iteration limits** - orchestrator prevents infinite loops

### Using Review Feedback

Your agent can access review feedback from `context_chain`:

```bash
#!/bin/sh
input=$(cat)

# Check if this is rework (Review artefact in context)
if echo "$input" | jq -e '.context_chain[] | select(.structural_type=="Review")' > /dev/null; then
  echo "Processing review feedback..." >&2

  # Extract review comments
  feedback=$(echo "$input" | jq -r '.context_chain[] | select(.structural_type=="Review") | .payload')
  echo "Reviewer feedback: $feedback" >&2

  # Address feedback in your implementation
  # ...
else
  echo "Fresh implementation (no feedback)" >&2
fi

# Output same format regardless
cat <<EOF
{
  "artefact_type": "CodeCommit",
  "artefact_payload": "$commit_hash",
  "summary": "Implementation complete"
}
EOF
```

### Configuration

Iteration limits are configured in `holt.yml`:

```yaml
version: "1.0"

orchestrator:
  max_review_iterations: 3  # Max times work can be reworked

agents:
  # ... your agents ...
```

When `max_review_iterations` is reached, the orchestrator creates a Failure artefact and terminates the workflow.

---

## Error Handling

### Method 1: Exit with Non-Zero Code

```bash
#!/bin/sh
set -e  # Exit on any error

filename="$1"

if [ -z "$filename" ]; then
  echo "Error: No filename provided" >&2
  exit 1  # Pup creates Failure artefact with stderr
fi

# Continue with work...
```

**Result:** Pup creates Failure artefact with:
- `payload`: stderr output + exit code
- `structural_type`: "Failure"

### Method 2: Explicit Failure Artefact

```bash
#!/bin/sh

# Check precondition
if ! command -v jq > /dev/null; then
  cat <<EOF
{
  "structural_type": "Failure",
  "artefact_payload": "jq command not found - install jq in Dockerfile",
  "summary": "Missing dependency: jq"
}
EOF
  exit 0  # Exit cleanly after outputting Failure JSON
fi

# Continue with work...
```

### Method 3: Validation Errors

```bash
#!/bin/sh
input=$(cat)

filename=$(echo "$input" | jq -r '.target_artefact.payload')

if [ -f "/workspace/$filename" ]; then
  cat <<EOF
{
  "structural_type": "Failure",
  "artefact_payload": "File $filename already exists. Refusing to overwrite.",
  "summary": "File conflict detected"
}
EOF
  exit 0
fi

# Proceed to create file...
```

### Best Practices

1. **Use `set -e`** to exit on any error
2. **Validate inputs** before performing work
3. **Provide actionable error messages** (tell user how to fix)
4. **Log to stderr** for debugging (`>&2`)
5. **Test failure paths** in development

---

## Advanced Patterns

### Pattern 1: LLM-Based Agent

```bash
#!/bin/sh
set -e

input=$(cat)

# Extract requirements from context
requirements=$(echo "$input" | jq -r '.context_chain[] | select(.type=="Requirements") | .payload')

# Call LLM API (Anthropic Claude)
response=$(curl -s -X POST https://api.anthropic.com/v1/messages \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  -H "content-type: application/json" \
  -d "{
    \"model\": \"claude-3-5-sonnet-20241022\",
    \"max_tokens\": 4096,
    \"messages\": [{
      \"role\": \"user\",
      \"content\": \"Generate code for: $requirements\"
    }]
  }")

# Extract generated code
code=$(echo "$response" | jq -r '.content[0].text')

# Write to workspace
echo "$code" > /workspace/implementation.go

# Commit
cd /workspace
git add implementation.go
git commit -m "[holt-agent: code-generator] Generated implementation"

commit_hash=$(git rev-parse HEAD)

# Output
cat <<EOF
{
  "artefact_type": "CodeCommit",
  "artefact_payload": "$commit_hash",
  "summary": "Generated code from requirements"
}
EOF
```

**Dockerfile additions:**
```dockerfile
RUN apk add --no-cache curl jq git
```

### Pattern 2: Multi-Step Processing

```bash
#!/bin/sh
set -e

input=$(cat)
goal=$(echo "$input" | jq -r '.target_artefact.payload')

echo "Step 1: Analyzing requirements..." >&2
# Analysis logic

echo "Step 2: Generating code..." >&2
# Generation logic

echo "Step 3: Running tests..." >&2
# Test execution

echo "Step 4: Committing result..." >&2
# Git workflow

# Output final result
```

### Pattern 3: Conditional Execution

```bash
#!/bin/sh
set -e

input=$(cat)
claim_type=$(echo "$input" | jq -r '.claim_type')

case "$claim_type" in
  "review")
    # Review existing work, output Review artefact
    echo "Performing review..." >&2
    # ...
    ;;
  "exclusive")
    # Execute new work
    echo "Executing work..." >&2
    # ...
    ;;
  *)
    echo "Unknown claim type: $claim_type" >&2
    exit 1
    ;;
esac
```

### Pattern 4: File Operations

```bash
#!/bin/sh
set -e

cd /workspace

# Create directory structure
mkdir -p src/api src/models src/tests

# Generate multiple files
cat > src/api/handler.go <<'EOF'
package api
// Handler code...
EOF

cat > src/models/user.go <<'EOF'
package models
// User model...
EOF

# Add all changes
git add .
git commit -m "[holt-agent: scaffolder] Created project structure"

commit_hash=$(git rev-parse HEAD)

# Output
cat <<EOF
{
  "artefact_type": "CodeCommit",
  "artefact_payload": "$commit_hash",
  "summary": "Created project scaffolding (3 files)"
}
EOF
```

---

## Testing Your Agent

### Local Testing (Without Holt)

Test your tool script in isolation:

```bash
# Create test input
cat > test-input.json <<EOF
{
  "claim_type": "exclusive",
  "target_artefact": {
    "id": "test-uuid",
    "type": "GoalDefined",
    "payload": "test goal",
    "version": 1,
    "logical_id": "test-logical",
    "source_artefacts": [],
    "produced_by_role": "user",
    "structural_type": "Standard"
  },
  "context_chain": []
}
EOF

# Test your script
cat test-input.json | ./run.sh

# Should output valid JSON
```

### Docker Testing

```bash
# Build image
docker build -t my-agent:latest -f agents/my-agent/Dockerfile .

# Run manually (debugging)
docker run -i my-agent:latest /app/run.sh < test-input.json
```

### Integration Testing with Holt

```bash
# Start Holt instance
holt up

# Submit test goal
holt forage --goal "test input"

# Monitor logs
holt logs my-agent

# Check results
holt hoard
```

### Validation Checklist

- [ ] Script reads JSON from stdin
- [ ] Script outputs valid JSON to stdout
- [ ] Exits with 0 on success, non-zero on failure
- [ ] Handles empty context_chain
- [ ] Handles missing payload gracefully
- [ ] Logs useful debugging info to stderr
- [ ] Creates valid git commits (if CodeCommit agent)
- [ ] Works when run multiple times (idempotent if possible)

---

## Examples

### Example 1: Echo Agent (Minimal)

**File:** `agents/example-agent/run.sh`

```bash
#!/bin/sh
input=$(cat)

echo "Echo agent processing..." >&2

timestamp=$(date +%s)

cat <<EOF
{
  "artefact_type": "EchoSuccess",
  "artefact_payload": "echo-$timestamp",
  "summary": "Echo agent processed successfully"
}
EOF
```

**Use case:** Testing, debugging, proof-of-concept

### Example 2: Git Agent (File Creator)

**File:** `agents/example-git-agent/run.sh`

```bash
#!/bin/sh
set -e

input=$(cat)

filename=$(echo "$input" | grep -o '"payload":"[^"]*"' | head -1 | cut -d'"' -f4)
claim_id=$(echo "$input" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

cd /workspace

cat > "$filename" <<EOF
# File created by Holt

Timestamp: $(date -u)
EOF

git add "$filename"
git commit -m "[holt-agent: git-agent] Created $filename

Claim-ID: $claim_id"

commit_hash=$(git rev-parse HEAD)

cat <<EOF
{
  "artefact_type": "CodeCommit",
  "artefact_payload": "$commit_hash",
  "summary": "Created $filename and committed"
}
EOF
```

**Use case:** Code generation, file creation, project scaffolding

### Example 3: Analyzer Agent (No Git)

```bash
#!/bin/sh
set -e

input=$(cat)

# Extract code from context
code=$(echo "$input" | jq -r '.context_chain[] | select(.type=="CodeCommit") | .payload' | head -1)

if [ -z "$code" ]; then
  cat <<EOF
{
  "structural_type": "Failure",
  "artefact_payload": "No code found in context to analyze",
  "summary": "Analysis failed: missing code"
}
EOF
  exit 0
fi

# Checkout code
cd /workspace
git checkout $code

# Run analysis (e.g., linter)
analysis_result=$(golint ./... 2>&1 || echo "No issues found")

# Output analysis artefact
cat <<EOF
{
  "artefact_type": "AnalysisReport",
  "artefact_payload": "$analysis_result",
  "summary": "Code analysis complete"
}
EOF
```

**Use case:** Code review, linting, security scanning

---

## Next Steps

1. **Start simple:** Begin with an echo agent to understand the contract
2. **Iterate:** Add complexity incrementally (Git, LLM calls, multi-step)
3. **Test thoroughly:** Validate both success and failure paths
4. **Document:** Add README to your agent directory
5. **Share:** Contribute useful agents back to the Holt community

For more examples, see:
- `agents/example-agent/` - Minimal echo agent
- `agents/example-git-agent/` - Git workflow agent

For troubleshooting, see: [docs/troubleshooting.md](./troubleshooting.md)

For system architecture, see: [PROJECT_CONTEXT.md](../PROJECT_CONTEXT.md)
