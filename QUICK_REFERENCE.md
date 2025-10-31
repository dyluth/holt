# **Holt Quick Reference: Key Concepts & Patterns**

**Purpose**: Essential patterns, structures, and workflows for rapid development  
**Scope**: Reference - quick lookup for common development patterns  
**Read when**: Need quick reference during implementation, lookup patterns

## **Core Data Structures**

### **Artefact (Redis Hash)**
```
id: UUID
logical_id: UUID (groups versions)
version: int
structural_type: Standard|Review|Question|Answer|Failure|Terminal
type: user-defined string (e.g., "CodeCommit", "DesignSpec")
payload: string (git hash, JSON, text)
source_artefacts: JSON array of UUIDs
produced_by_role: string (agent key from holt.yml, which IS the role, or 'user')
created_at_ms: int64 (Unix milliseconds) # M3.9
```

### **Claim (Redis Hash)**
```
id: UUID
artefact_id: UUID
status: pending_review|pending_parallel|pending_exclusive|pending_assignment|complete|terminated
granted_review_agents: JSON array
granted_parallel_agents: JSON array  
granted_exclusive_agent: string
granted_agent_image_id: string # M3.9: sha256 digest of the agent image
additional_context_ids: JSON array # M3.3: For feedback loops
termination_reason: string # M3.3: Reason for termination
```

### **Bid (On Claim)**
A Redis Hash (`holt:{instance_name}:claim:{uuid}:bids`) where each key-value pair is:
- **Key**: Agent's role (e.g., 'Coder', 'Reviewer')
- **Value**: Bid type (`review`, `claim`, `exclusive`, `ignore`)

## **Redis Key Patterns**

```
# Global keys
holt:instance_counter                          # Atomic counter for instance naming
holt:instances                                 # HASH of active instance metadata

# Instance-specific keys
holt:{instance_name}:artefact:{uuid}           # Artefact data
holt:{instance_name}:claim:{uuid}              # Claim data
holt:{instance_name}:claim:{uuid}:bids         # Bid data
holt:{instance_name}:thread:{logical_id}       # Version tracking (ZSET)
holt:{instance_name}:lock                      # Instance lock (TTL-based, heartbeat)
holt:{instance_name}:agent_images              # HASH of role -> image_id mapping (M3.9)
holt:{instance_name}:grant_queue:{role}        # ZSET for paused grants (M3.5)
```

## **Pub/Sub Channels**

```
holt:{instance_name}:artefact_events    # Orchestrator watches for new artefacts
holt:{instance_name}:claim_events       # Agents watch for new claims
holt:{instance_name}:workflow_events    # Bids and grants for real-time watch (M2.6)
holt:{instance_name}:agent:{role}:events # Agent-specific grant notifications (M2.2)
```

## **Claim Lifecycle**

```
pending_review → pending_parallel → pending_exclusive → complete
             ↘ terminated (if review feedback or failure)
```

## **Agent Pup Operational Modes**
*(See `design/agent-pup.md` for details)*

### **Standard Mode**
- Both Claim Watcher and Work Executor active.

### **Controller Mode (`mode: controller`)**
- Only Claim Watcher active. Bids on behalf of its role.

### **Worker Mode (`pup --execute-claim <id>`)**
- Only Work Executor active. Executes a single assigned claim and exits.

## **Tool Execution Contract**

### **Input (stdin JSON)**
```json
{
  "claim_type": "review|claim|exclusive",
  "target_artefact": { /* full artefact object */ },
  "context_chain": [ /* array of historical artefact objects */ ]
}
```

### **Output (stdout JSON)**
```json
{
  "artefact_type": "string",
  "artefact_payload": "string", 
  "summary": "string"
}
```

## **Common CLI Commands**

### **Global Flags**
```bash
--config, -f <path>   # Path to holt.yml
--debug, -d           # Enable verbose debug output
--quiet, -q           # Suppress all non-essential output
```

### **Instance & Workflow**
```bash
holt init                                # Bootstrap new project
holt up [--name <instance>] [--force]    # Start holt instance
holt down [--name <instance>]            # Stop holt instance
holt list                                # List active instances
holt forage --goal "description"         # Start a new workflow
```

### **Observability & Debugging**
*Note: All commands support short IDs (e.g., `abc123de`)*

**`holt watch [--since <duration>] [--type <glob>] [--agent <role>] [--output jsonl]`**

The primary tool for observing a Holt instance. It has two modes:

*   **Live Mode (default):** Streams all events on the Blackboard in real-time.
*   **Historical Replay Mode (`--since`):** Use a duration (e.g., `1h`, `30m`) to get a complete, chronological replay of a past workflow. This replay reconstructs the entire sequence of events, including:
    *   Artefacts (with original creation timestamps)
    *   Claims (including terminated claims)
    *   Bids, grants, and review results
    *   Rework assignments from feedback loops

**`holt hoard [--since <duration>] [--type <glob>] [--agent <role>] [--output jsonl]`**

Inspects historical artefacts. Use the filtering flags to find specific artefacts created in the past. To see the full history of a workflow, use `holt watch --since`.

**`holt hoard <artefact-id>`**

Retrieves and displays the full details for a single artefact.

**`holt logs <agent-role|orchestrator>`**

Views the logs for a specific running or stopped container (e.g., `holt logs Coder`).

### **Human-in-the-Loop (Phase 4+)**
```bash
holt questions [--wait]                  # List questions from agents
holt answer <question-id> "response"     # Answer an agent's question
```

## **Health Check Endpoints**

### **Default (`/healthz`)**
```
GET /healthz
200 OK           # Connected to Redis
503 Unavailable  # Redis connection failed
```

### **Configurable (M3.9+)**
Agents can define a custom `health_check` command in `holt.yml`. The `/healthz` endpoint will reflect the success or failure of that custom command.
