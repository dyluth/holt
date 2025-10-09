# Example Agent

A minimal Sett agent for testing M2.2: Claim Watching & Bidding.

## Purpose

This agent demonstrates the basic agent cub architecture without implementing actual work execution. It's used for testing the claim-bid-grant cycle in M2.2.

## What It Does

- **Watches for claims** via the claim_events Pub/Sub channel
- **Submits exclusive bids** for all claims (hardcoded M2.2 strategy)
- **Receives grant notifications** via its agent-specific event channel
- **Pushes granted claims to work queue** (but doesn't execute them)

The work executor simply sleeps - real tool execution will be implemented in M2.3.

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

## Expected Behavior (M2.2)

When an artefact is created on the blackboard:

1. Orchestrator creates a claim
2. Agent cub receives claim event via claim_events channel
3. Agent cub submits "exclusive" bid
4. Orchestrator waits for consensus (all agents bid)
5. Orchestrator grants claim to agent
6. Orchestrator publishes grant notification to agent's channel
7. Agent cub receives grant, validates it, and logs it
8. Work executor receives claim on queue but just logs it (no execution in M2.2)

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
│  │  │  - Handle grants         │  │  │
│  │  └──────────────────────────┘  │  │
│  │  ┌──────────────────────────┐  │  │
│  │  │  Work Executor (stubbed) │  │  │
│  │  │  - Logs received claims  │  │  │
│  │  └──────────────────────────┘  │  │
│  └────────────────────────────────┘  │
│  ┌────────────────────────────────┐  │
│  │  run.sh (sleep infinity)       │  │
│  └────────────────────────────────┘  │
└──────────────────────────────────────┘
```

## Development

This is a test agent for M2.2. For real agents, replace `run.sh` with actual tool execution logic in M2.3.
