# Example Agent

A minimal Sett agent for testing M2.3: Work Execution & Tool Contract.

## Purpose

This agent demonstrates the complete agent cub architecture including tool execution via stdin/stdout JSON contract. It's a simple "echo" agent that processes claims and creates result artefacts.

## What It Does

- **Watches for claims** via the claim_events Pub/Sub channel
- **Submits exclusive bids** for all claims (hardcoded M2.3 strategy)
- **Receives grant notifications** via its agent-specific event channel
- **Executes tool subprocess** (`run.sh`) with JSON input on stdin
- **Parses tool output** from stdout and creates artefacts
- **Creates derivative artefacts** with proper provenance (new logical threads)

## Building

From the project root directory:

```bash
docker build -t example-agent:latest -f agents/example-agent/Dockerfile .
```

**Note:** The Dockerfile context must be the project root (`.`) so it can access the cub source code.

## Configuration

Add to your `sett.yml`:

```yaml
version: "1.0"
agents:
  example-agent:
    role: "Example Agent"
    image: "example-agent:latest"
    command: ["/app/run.sh"]
    workspace:
      mode: ro
```

## Running

```bash
# Build the agent image
docker build -t example-agent:latest -f agents/example-agent/Dockerfile .

# Start the Sett instance (which will launch the agent)
sett up

# View agent logs
sett logs example-agent
```

## Tool Contract (M2.4)

### Stdin JSON Format

The cub passes this JSON structure to the tool via stdin:

```json
{
  "claim_type": "exclusive",
  "target_artefact": {
    "id": "uuid",
    "type": "CodeCommit",
    "payload": "abc123def",
    "structural_type": "Standard",
    "version": 1,
    "logical_id": "uuid",
    "source_artefacts": ["design-uuid"],
    "produced_by_role": "architect"
  },
  "context_chain": [
    {
      "id": "goal-uuid",
      "type": "GoalDefined",
      "payload": "Build user authentication system",
      "structural_type": "Standard",
      "version": 1,
      "logical_id": "goal-uuid",
      "source_artefacts": [],
      "produced_by_role": "user"
    },
    {
      "id": "design-uuid",
      "type": "DesignSpec",
      "payload": "REST API with JWT tokens...",
      "structural_type": "Standard",
      "version": 1,
      "logical_id": "design-uuid",
      "source_artefacts": ["goal-uuid"],
      "produced_by_role": "architect"
    }
  ]
}
```

**Context Chain (M2.4)**:
- Populated via BFS traversal of artefact dependency graph
- Contains full artefact objects in chronological order (oldest → newest)
- Filtered to include only Standard and Answer artefacts
- Uses thread tracking to ensure latest versions are included
- Empty array `[]` for root artefacts (no source_artefacts)
- Provides agents with rich historical context for informed decisions

### Stdout JSON Format

The tool must output exactly ONE JSON object to stdout:

```json
{
  "artefact_type": "EchoSuccess",
  "artefact_payload": "echo-1728579580",
  "summary": "Echo agent successfully processed the claim"
}
```

Optional field: `structural_type` (defaults to "Standard")

### Derivative Relationships

**CRITICAL CONCEPT**: When an agent executes work, it creates a **derivative artefact**, not an evolutionary version.

- **Derivative** (M2.3 pattern): New logical_id, version=1, source_artefacts=[input]
  - Example: GoalDefined → EchoSuccess (different work products)
- **Evolutionary** (future): Same logical_id, incremented version
  - Example: Design v1 → Design v2 (same work, evolved)

The echo agent creates derivatives: each output is a NEW work product derived from the input artefact.

### Git Commit Validation (M2.4)

For code-generating agents that return `CodeCommit` artefacts:

- The cub validates git commit hashes before creating artefacts
- Validation uses `git cat-file -e <hash>` to verify commit exists
- If validation fails, a Failure artefact is created instead
- Agent scripts should commit code changes before returning the hash

**Recommended commit message format** (not enforced):
```
[sett-agent: {agent-role}] {summary}

Claim-ID: {claim-id}
```

Example:
```
[sett-agent: code-generator] Implemented user authentication endpoint

Claim-ID: claim-abc-123
```

## Expected Behavior (M2.4)

When an artefact is created on the blackboard:

1. Orchestrator creates a claim
2. Agent cub receives claim event via claim_events channel
3. Agent cub submits "exclusive" bid
4. Orchestrator waits for consensus (all agents bid)
5. Orchestrator grants claim to agent
6. Orchestrator publishes grant notification to agent's channel
7. Agent cub receives grant, validates it, and queues claim for execution
8. Work executor fetches target artefact
9. **Work executor assembles context chain via BFS graph traversal**
10. Work executor prepares stdin JSON with context_chain
11. Work executor runs `/app/run.sh` as subprocess with 5-minute timeout
12. Work executor reads stdout JSON
13. **For CodeCommit artefacts: validates git commit hash exists**
14. Work executor creates derivative artefact
15. New artefact published to artefact_events channel
16. Orchestrator sees new artefact and creates a new claim (workflow continues)

## Architecture

```
┌──────────────────────────────────────┐
│  example-agent container             │
│  ┌────────────────────────────────┐  │
│  │  Agent Cub                     │  │
│  │  ┌──────────────────────────┐  │  │
│  │  │  Claim Watcher           │  │  │
│  │  │  - Subscribe claim_events│  │  │
│  │  │  - Subscribe agent:events│  │  │
│  │  │  - Submit bids           │  │  │
│  │  │  - Queue granted claims  │  │  │
│  │  └──────────────────────────┘  │  │
│  │  ┌──────────────────────────┐  │  │
│  │  │  Work Executor           │  │  │
│  │  │  - Fetch target artefact │  │  │
│  │  │  - Prepare stdin JSON    │  │  │
│  │  │  - Execute subprocess    │  │  │
│  │  │  - Parse stdout JSON     │  │  │
│  │  │  - Create artefact       │  │  │
│  │  └──────────────────────────┘  │  │
│  └────────────────────────────────┘  │
│  ┌────────────────────────────────┐  │
│  │  run.sh (echo tool)            │  │
│  │  - Read stdin JSON             │  │
│  │  - Output stdout JSON          │  │
│  └────────────────────────────────┘  │
│  ┌────────────────────────────────┐  │
│  │  /workspace (mounted Git repo) │  │
│  └────────────────────────────────┘  │
└──────────────────────────────────────┘
```

## Development

This is a minimal echo agent for M2.3. For real agents, replace `run.sh` with actual tool logic (LLM calls, code analysis, etc.).
