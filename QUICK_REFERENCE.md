# **Sett Quick Reference: Key Concepts & Patterns**

**Purpose**: Essential patterns, structures, and workflows for rapid development  
**Scope**: Reference - quick lookup for common development patterns  
**Estimated tokens**: ~800 tokens  
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
produced_by_role: string (agent's 'role' from sett.yml or 'user')
```

### **Claim (Redis Hash)**
```
id: UUID
artefact_id: UUID
status: pending_review|pending_parallel|pending_exclusive|complete|terminated
granted_review_agents: JSON array
granted_parallel_agents: JSON array  
granted_exclusive_agent: string
```

### **Bid (On Claim)**
A Redis Hash (`sett:{instance_name}:claim:{uuid}:bids`) where each key-value pair is:
- **Key**: Agent's logical name (e.g., 'go-coder-agent')
- **Value**: Bid type (`review`, `claim`, `exclusive`, `ignore`)

## **Redis Key Patterns**

```
# Global keys
sett:instance_counter                          # Atomic counter for instance naming
sett:instances                                 # HASH of active instance metadata (workspace path, run_id, etc.)

# Instance-specific keys
sett:{instance_name}:artifact:{uuid}           # Artefact data
sett:{instance_name}:claim:{uuid}              # Claim data
sett:{instance_name}:claim:{uuid}:bids         # Bid data (see above)
sett:{instance_name}:thread:{logical_id}       # Version tracking (ZSET)
sett:{instance_name}:lock                      # Instance lock (TTL-based, heartbeat)
```

## **Pub/Sub Channels**

```
sett:{instance_name}:artefact_events    # Orchestrator watches for new artefacts
sett:{instance_name}:claim_events       # Agents watch for new claims
```

## **Component Communication Flow**

```
CLI → Artefact → [artefact_events] → Orchestrator → Claim → [claim_events] → Agents
Agents → Bid → Orchestrator → Grant → Agent → Work → New Artefact
```

## **Claim Lifecycle**

```
pending_review → pending_parallel → pending_exclusive → complete
             ↘ terminated (if review feedback or failure)
```

## **Agent Cub Operational Modes**
*(See 'Agent scaling and concurrency' in sett-system-specification.md for details)*

### **Standard Mode (replicas: 1)**
- Both Claim Watcher and Work Executor active
- Full bidding and execution lifecycle

### **Bidder-Only Mode (replicas > 1, Controller)**
- Only Claim Watcher active
- Bids on behalf of agent type, never executes

### **Execute-Only Mode (replicas > 1, Worker)**
- Launched with `--execute-claim <claim_id>`
- No bidding, direct work assignment, single-use

## **Tool Execution Contract**

### **Input (stdin JSON)**
```json
{
  "claim_type": "review|claim|exclusive",
  "target_artefact": { /* full artefact object */ },
  "context_chain": [ /* array of artefact objects */ ]
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

### **Question Escalation**
```json
{
  "structural_type": "Question",
  "payload": "question text"
}
```

## **Environment Variables**

### **Agent Cub**
```
SETT_INSTANCE_NAME     # Sett instance identifier
SETT_AGENT_NAME        # Agent logical name from sett.yml  
REDIS_URL              # Blackboard connection
SETT_PROMPT_CLAIM      # Claim evaluation prompt
SETT_PROMPT_EXECUTION  # Execution prompt
```

### **Orchestrator**
```
SETT_INSTANCE_NAME     # Sett instance identifier
REDIS_URL              # Blackboard connection
SETT_CONFIG_PATH       # Path to sett.yml
```

## **Common CLI Commands**

```bash
sett init                                # Bootstrap new project
sett up [--name <instance>] [--force]    # Start sett (auto-increment name, blocks on workspace collision unless --force)
sett down [--name <instance>]            # Stop sett (name defaults to most recent)
sett list                                # List active instances
sett forage --goal "description"         # Start workflow
sett watch [--name <instance>]           # Live activity (name defaults to most recent)
sett hoard [--name <instance>]           # List artefacts (name defaults to most recent)
sett questions [--wait]                  # Human Q&A
sett answer <id> "response"              # Answer questions
sett logs <agent-name>                   # Debug logs
```

## **Git Integration Pattern**

```bash
# Agent script workflow:
git checkout <commit_hash>    # From artefact payload
# ... make changes ...
git add .
git commit -m "message"
git rev-parse HEAD           # Return as artefact_payload
```

## **Health Check Endpoints**

```
GET /healthz
200 OK           # Connected to Redis
503 Unavailable  # Redis connection failed
```

## **Review Logic**

```
Review Artefact Payload: "{}" or "[]" → Approval (Any other content implies feedback)
```

## **Error Handling Pattern**

```
Agent failure → Failure artefact → Claim terminated → Manual intervention required
```
