# **Holt Quick Reference: Key Concepts & Patterns**

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
produced_by_role: string (agent's 'role' from holt.yml or 'user')
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
A Redis Hash (`holt:{instance_name}:claim:{uuid}:bids`) where each key-value pair is:
- **Key**: Agent's logical name (e.g., 'go-coder-agent')
- **Value**: Bid type (`review`, `claim`, `exclusive`, `ignore`)

## **Redis Key Patterns**

```
# Global keys
holt:instance_counter                          # Atomic counter for instance naming
holt:instances                                 # HASH of active instance metadata (workspace path, run_id, etc.)

# Instance-specific keys
holt:{instance_name}:artefact:{uuid}           # Artefact data
holt:{instance_name}:claim:{uuid}              # Claim data
holt:{instance_name}:claim:{uuid}:bids         # Bid data (see above)
holt:{instance_name}:thread:{logical_id}       # Version tracking (ZSET)
holt:{instance_name}:lock                      # Instance lock (TTL-based, heartbeat)
```

## **Pub/Sub Channels**

```
holt:{instance_name}:artefact_events    # Orchestrator watches for new artefacts
holt:{instance_name}:claim_events       # Agents watch for new claims
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

## **Agent Pup Operational Modes**
*(See 'Agent scaling and concurrency' in holt-system-specification.md for details)*

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

### **Agent Pup**
```
HOLT_INSTANCE_NAME     # Holt instance identifier
HOLT_AGENT_NAME        # Agent logical name from holt.yml  
REDIS_URL              # Blackboard connection
HOLT_PROMPT_CLAIM      # Claim evaluation prompt
HOLT_PROMPT_EXECUTION  # Execution prompt
```

### **Orchestrator**
```
HOLT_INSTANCE_NAME     # Holt instance identifier
REDIS_URL              # Blackboard connection
HOLT_CONFIG_PATH       # Path to holt.yml
```

## **Common CLI Commands**

```bash
holt init                                # Bootstrap new project
holt up [--name <instance>] [--force]    # Start holt (auto-increment name, blocks on workspace collision unless --force)
holt down [--name <instance>]            # Stop holt (name defaults to most recent)
holt list                                # List active instances
holt forage --goal "description"         # Start workflow
holt watch [--name <instance>]           # Live activity (name defaults to most recent)
holt hoard [--name <instance>]           # List artefacts (name defaults to most recent)
holt questions [--wait]                  # Human Q&A
holt answer <id> "response"              # Answer questions
holt logs <agent-name>                   # Debug logs
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
