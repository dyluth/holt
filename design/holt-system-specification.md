# **Holt: A container-native AI agent orchestrator**

**Purpose**: Complete system architecture, shared components, and implementation details  
**Scope**: Reference - comprehensive system specification  
**Estimated tokens**: ~5,700 tokens  
**Read when**: Need complete architecture understanding, implementing core components

Holt is a standalone, **container-native orchestration engine** designed to manage a clan of specialised, tool-equipped AI agents. It provides a robust, scalable, and auditable platform for automating complex workflows by leveraging the power of containerisation and the familiar paradigms of DevOps. While initially focused on software engineering tasks, Holt's immutable audit trails and human-in-the-loop design make it particularly valuable for regulated industries, compliance workflows, and any environment where AI transparency and accountability are business-critical.

It is not an LLM-chaining library. It is an orchestration engine for the real-world toolchains that software professionals use every day. It enables the automation of tasks that rely on compilers, CLIs, and infrastructure tools (git, docker, kubectl) by orchestrating agents whose tools are not just Python functions, but any command-line tool that can be packaged into a container.

## **Guiding principles**

Holt is an opinionated tool. Our development philosophy is guided by a clear set of principles that prioritise simplicity, robustness, and pragmatism.

* **Pragmatism over novelty (YAGNI):** We prioritise using existing, battle-hardened tools rather than building our own. The core of Holt is an orchestrator, not a database or a container runtime. We use Docker for containers and Redis for state, because they are excellent.  
* **Zero-configuration, progressively enhanced:** The experience must be seamless out of the box. A developer should be able to get a basic holt running with a single command. Smart defaults cover 90% of use cases, while advanced features and enterprise-grade workflows are available for those who need them.  
* **Small, single-purpose components:** Each element in the system—the orchestrator, the CLI, the agent pup—has a clear, well-defined job and does that one thing excellently. Complexity is managed by composing simple parts.  
* **Auditability is a core feature:** Artefacts are immutable. Every decision and agent interaction is recorded on the blackboard, providing a complete, auditable history of the workflow. This makes Holt particularly valuable for regulated industries, compliance workflows, and any environment where AI transparency and accountability are legally required or business-critical.  
* **ARM64-first design:** Development and deployment are optimised for ARM64, with AMD64 as a fully supported, compatible target.  
* **Principle of least privilege:** Agents run in non-root containers with the minimal set of privileges required to perform their function.

## **System architecture overview**

The architecture separates concerns between three primary components: the **Orchestrator**, the **Agent Pups**, and the **CLI**. The Orchestrator watches for new **Artefacts** and creates **Claims** based on them. Agent Pups watch for **Claims** and bid on them. This creates a robust, event-driven workflow.

### **Component responsibilities**

* **Orchestrator**: Event-driven coordination engine that manages claim lifecycles and agent coordination (see `design/holt-orchestrator-component.md`)
* **Agent Pup**: Lightweight binary that runs in agent containers, handling bidding, context assembly, and work execution (see `design/agent-pup.md`)  
* **CLI**: User interface and workflow initiation tool that provides project management and human-in-the-loop commands
* **Blackboard**: Redis-based shared state system where all components interact via well-defined data structures

### **High-level workflow**

1. **Initiation**: User runs `holt forage --goal "Create a REST API"` 
2. **Artefact creation**: CLI creates a GoalDefined artefact on the blackboard
3. **Claim creation**: Orchestrator sees the artefact and creates a corresponding claim
4. **Bidding**: All agents evaluate the claim and submit bids ('review', 'claim', 'exclusive', 'ignore')
5. **Phased execution**: Orchestrator grants claims in review → parallel → exclusive phases
6. **Work execution**: Agent pups execute their tools and create new artefacts
7. **Iteration**: New artefacts trigger new claims, continuing the workflow
8. **Termination**: Workflow ends when an agent creates a Terminal artefact

For detailed orchestration logic, see `design/holt-orchestrator-component.md`.

### **Phased execution**

The Orchestrator manages Claims through a strict three-phase lifecycle. This model implies a permission structure that aligns with the principle of least privilege:

*   **Review Phase (`ro`):** Reviewers should only need read-only access to inspect artefacts.
*   **Parallel Phase (`ro`):** Parallel agents (like linters or testers) should also operate with read-only access.
*   **Exclusive Phase (`rw`):** The agent with the exclusive grant is the only one expected to modify the workspace, thus requiring read-write access.

## **Blackboard data structures**

The blackboard is the primary API of the system and serves as a lightweight ledger for an external, version-controlled system like Git. It stores metadata and pointers, not large data blobs. All components interact via these well-defined data structures stored in Redis. The schemas will be formalised into a Go package (pkg/blackboard/types.go) that will serve as the canonical source of truth.

### **Redis key patterns**

All keys are namespaced to the instance to enable multiple holts on the same Redis instance:

**Global keys (not instance-specific):**
* `holt:instance_counter` - Atomic counter for instance naming
* `holt:instances` - Redis Hash storing metadata for all active instances (workspace paths, run IDs, timestamps)

**Instance-specific keys:**
* `holt:{instance_name}:artefact:{uuid}` - Individual artefact data
* `holt:{instance_name}:claim:{uuid}` - Individual claim data
* `holt:{instance_name}:claim:{uuid}:bids` - Bids for a specific claim
* `holt:{instance_name}:thread:{logical_id}` - Sorted set for version tracking
* `holt:{instance_name}:lock` - Instance lock (TTL-based, heartbeat)

### **Redis Pub/Sub channels**

* `holt:{instance_name}:artefact_events` - For the orchestrator to watch for new artefacts
* `holt:{instance_name}:claim_events` - For agents to watch for new claims

### **Data structures**

#### **Artefacts (holt:{instance_name}:artefact:{uuid} - Redis Hash)**

The central, immutable data object in Holt.

* **id** (string): The unique UUID of this specific artefact
* **logical_id** (string): A shared UUID that groups all versions of the same logical artefact together. For the first version, logical_id is the same as id
* **version** (int): A simple, incrementing version number within a logical thread
* **structural_type** (string enum): The role an artefact plays in the orchestration flow. Hardcoded types the Orchestrator understands:
  * `Standard` - Regular workflow artefacts
  * `Review` - Review feedback artefacts
  * `Question` - Human input required
  * `Answer` - Human responses
  * `Failure` - Error reports
  * `Terminal` - Workflow completion markers
* **type** (string): A user-defined, domain-specific string that only agents care about. Opaque to the Orchestrator (e.g., "DesignSpec", "CodeSummary", "UnitTestResult", "GoalDefined")
* **payload** (string): The main content of the artefact. For code artefacts, this is a git commit hash. For Questions/Answers, this is plain text. For Reviews, this is JSON (empty = approval, non-empty = feedback)
* **source_artefacts** (string): A JSON-encoded array of UUIDs establishing the DAG of dependencies
* **produced_by_role** (string): The role of the agent that created this artefact

#### **Claims (holt:{instance_name}:claim:{uuid} - Redis Hash)**

A record of the Orchestrator's decisions about a specific Artefact.

* **id** (string): The UUID of the claim
* **artefact_id** (string): The ID of the artefact this claim is for
* **additional_context_ids** (string): A JSON-encoded array of Artefact IDs (e.g., `Review` artefacts) to be included as context. Used for the automated feedback loop.
* **status** (string): The current state of the claim:
  * `pending_consensus` - Waiting for all agents to bid
  * `pending_review` - Waiting for review phase completion
  * `pending_parallel` - Waiting for parallel phase completion  
  * `pending_exclusive` - Waiting for exclusive phase completion
  * `pending_assignment` - Waiting to be assigned to an agent for rework (feedback loop)
  * `complete` - All phases finished successfully
  * `terminated` - Failed or killed due to feedback
* **granted_review_agents** (string): A JSON-encoded array of agent roles (from holt.yml) whose review bids were granted
* **granted_parallel_agents** (string): A JSON-encoded array of agent roles whose claim bids were granted
* **granted_exclusive_agent** (string): The single agent role whose exclusive bid was granted

#### **Bids (holt:{instance_name}:claim:{uuid}:bids - Redis Hash)**

A collection of bids submitted by agents for a specific claim.

* **Key-Value Pairs:** The hash is a map where each key is the agent's **role** (e.g., 'go-coder-agent' from holt.yml) and the value is its bid type:
  * `review` - Request to review the artefact
  * `claim` - Request to work on the artefact in parallel
  * `exclusive` - Request exclusive access to work on the artefact
  * `ignore` - Explicit declaration of no interest

#### **Thread tracking (holt:{instance_name}:thread:{logical_id} - Redis Sorted Set)**

Efficient "latest version by logical_id" lookup using Redis ZSET. For each logical thread, artefact_ids are added with their version as the score. Getting the latest version is a single `ZREVRANGE ... LIMIT 1` call.

### **Large payload handling**

The agent's container has the project's working directory mounted. The payload for a code artefact is a git commit hash. The agent's command script is responsible for executing `git checkout <hash>` to get the codebase into the correct state before running its tools.

## **The holt.yml configuration file**

The `holt.yml` file is the central, declarative configuration for a Holt instance. It defines the clan of agents available to the orchestrator.

### **Agent Definition**

Each top-level key under the `agents` map is the agent's unique **role**. This role is the definitive identifier used for bidding, granting, and logging.

```yaml
version: '1.0'

agents:
  # This key is the agent's unique role.
  doc-writer:
    # Standard Docker build context. Alternatively, use a pre-built `image`.
    build:
      context: './agents/doc-writer'
    
    # The mandatory command the pup will execute.
    command: ["/usr/bin/run.sh"]
    
    # The workspace is mounted read-write for this agent.
    workspace:
      mode: 'rw'

  # A scalable agent using the controller-worker pattern.
  go-linter:
    build:
      context: './agents/go-linter'
    command: ["/usr/bin/run-linter.sh"]
    
    # This agent acts as a controller, enabling scaled execution.
    mode: controller
    
    # The controller will delegate work to up to 5 ephemeral worker containers.
    max_concurrent: 5

    # The workspace is mounted read-only, as a linter shouldn't modify files.
    workspace:
      mode: 'ro'

  # An intelligent agent that uses a script to decide when to bid.
  refactor-agent:
    build:
      context: './agents/refactor-agent'
    command: ["/usr/bin/run-refactor.sh"]
    workspace:
      mode: 'rw'

    # Delegates the bid decision to an external script.
    bid_script: "/usr/bin/decide-bid.sh"

    # Environment variables, mirroring docker-compose syntax.
    environment:
      - OPENAI_API_KEY

# Optional: Overrides for core infrastructure services
services:
  orchestrator:
    image: 'holt/orchestrator:v0.1.0'
  redis:
    image: 'redis:7-alpine'
```

## **Agent Scaling and Concurrency**

Holt supports two distinct operational models for agents, configured via the `mode` property in `holt.yml`.

### **1. Standard Agents (Default)**

If the `mode` property is omitted, the agent runs in standard mode. The orchestrator manages a single, persistent container for this role. The `pup` process inside this container is fully autonomous: it watches for claims, bids on them, and executes any work it is granted.

This is the simplest operational model, suitable for roles that do not require horizontal scaling.

### **2. Scalable Agents (`mode: controller`)**

For roles that need to handle multiple tasks concurrently (like linters or test runners), Holt uses a **controller-worker pattern**. This is enabled by setting `mode: controller` for an agent role.

This pattern involves two types of `pup` processes:

*   **The Controller `pup`**: When `holt up` is run, the orchestrator launches **one persistent container** for the role. The `pup` inside this container runs in a special **controller mode**. It watches for and bids on claims, but **it never executes work itself**. Its sole job is to acquire work for its fleet of workers.

*   **The Worker `pup`s**: When the orchestrator grants a claim to this role, it launches a **new, ephemeral container** for the specific task. The `pup` inside this worker container is launched in **worker mode** (`pup --execute-claim <claim_id>`). It executes the single assigned claim, publishes its result, and then exits. The orchestrator is responsible for cleaning up the ephemeral container.

The `max_concurrent` property in `holt.yml` defines the maximum number of worker containers that the orchestrator will launch in parallel for that role.

This pattern provides clean separation of concerns, eliminates race conditions for work acquisition, and allows for efficient, horizontal scaling of agent execution.

## **The thematic CLI**

The holt CLI is designed to be intuitive and memorable, using the holt metaphor to create a cohesive user experience.

### **Project management commands**

* **`holt init`** - Bootstrap command that scaffolds a new project with:
  * `holt.yml` - Pre-populated configuration file with commented example agent
  * `agents/` - Directory to hold agent definitions
  * `agents/example-agent/` - Example agent with Dockerfile and simple run.sh script

### **Holt lifecycle commands**

* **`holt up [--name <instance>] [--force]`** - Brings a new holt online. Fails if the name is in use or if another instance is active on the same workspace path (unless `--force` is used). Name defaults to an auto-incrementing value (e.g., 'default-1').
* **`holt down [--name <instance>]`** - Takes a holt offline (name defaults to the most recently created instance)
* **`holt list`** - Lists all active holts on the host

### **Workflow commands**

* **`holt forage --goal "Your goal here"`** - The primary command to start a new task. Creates the initial GoalDefined artefact that triggers the workflow
* **`holt watch [--name <instance>]`** - Provides a live view of the holt's activity log (name defaults to 'default')

### **Inspection commands**

* **`holt hoard [--name <instance>]`** - Lists all artefacts produced by the agents (name defaults to 'default')
* **`holt unearth <artefact-id>`** - Retrieves the content of a specific artefact

### **Human-in-the-loop commands**

* **`holt questions [--wait]`** - Lists and manages questions escalated for human review
  * Default mode: Lists all currently unanswered Question artefacts (and their IDs) and exits
  * With `--wait` flag: Blocks until a new Question artefact appears, prints its details and ID, then exits
* **`holt answer <question-id> "<answer-text>"`** - Responds to a specific question. Creates the corresponding Answer artefact on the blackboard, unblocking waiting agents

### **Debug and monitoring commands**

* **`holt logs <agent-logical-name>`** - Debug tool that provides a user-friendly wrapper around `docker logs`. Translates the agent's logical name into the full, namespaced container name and streams the logs using the Docker Go SDK. Works for both running `reuse` agents and stopped containers from `fresh_per_call` agents

## **Human interaction details**

### **Question/Answer workflow**
The Question/Answer flow is managed via a simple, scriptable, two-command CLI interface:

**Question format:** The payload for Question artefacts is a simple string containing the text of the question.

**Answer format:** The payload for Answer artefacts is a simple string containing the answer text.

**Question escalation:** A script can create a Question artefact by outputting specific JSON to stdout:
```json
{
  "structural_type": "Question",
  "payload": "The design spec is ambiguous about null handling. Is it in scope?"
}
```

### **Review logic**
The review process is deterministic with clear pass/fail definitions:

**Feedback detection:** The Review artefact's payload determines the outcome:
* Empty JSON object `{}` or empty list `[]` = approval
* Any other valid JSON (e.g., `{"comments": ["..."]}`) = feedback

**Decision-making:** The Orchestrator makes decisions based on a simple check: is the Review artefact's payload empty? It does not interpret the content of feedback.

**Feedback loop:** When a claim is rejected, the Orchestrator initiates the **Automated Feedback Loop**. It creates a new claim, directly assigned to the original agent, and populates the claim's `additional_context_ids` field with the IDs of the `Review` artefacts. This provides the necessary context for the agent to address the feedback in the next iteration.

## **Technical implementation details**

### **Project structure**
The project uses the standard Go layout for multiple binaries:

```
holt/
├── cmd/             # Binaries: holt, orchestrator, pup
├── pkg/             # Shared public packages (e.g., blackboard types)
├── internal/        # Private implementation details
├── agents/          # Example agent definitions (Dockerfiles, scripts)
└── Makefile
```

### **Prerequisites**

* Go (version 1.22 or later)
* Docker Engine
* make

### **Build & deployment**

**Container images:** Simple, multi-stage Dockerfiles produce minimal, secure agent images.

**Makefile targets:** Standard targets: `build`, `test`, `lint`, `docker-build-all`, `clean`.

**Deployment model:** The primary target is Docker on a local machine. Kubernetes is a future consideration.

### **Build & run**

1. **Clone the repository:**
   ```bash
   git clone https://github.com/your-repo/holt.git
   cd holt
   ```

2. **Build all binaries:**
   ```bash
   make build
   ```
   This will place the holt CLI binary in the ./bin/ directory.

3. **Run your first holt:**
   ```bash
   ./bin/holt up --name my-first-holt
   ```

4. **Tear it down:**
   ```bash
   ./bin/holt down --name my-first-holt
   ```

### **Testing strategy**

**Unit vs integration:** Dependency injection and mock implementations of the blackboard client for fast unit tests. For integration tests, the testcontainers-go library manages ephemeral Redis instances.

**LLM mocking:** Agent command scripts are tested by pointing them to a mock HTTP server that returns pre-defined LLM responses.

### **Health checks and monitoring**

**Health check endpoints:** The orchestrator and pups expose a `GET /healthz` endpoint. Returns `200 OK` if connected to Redis, `503 Service Unavailable` otherwise.

**Monitoring:** V1 uses high-quality, structured (JSON) logging to stdout.

## **Error handling and resilience**

### **Redis failures**
The orchestrator and pups implement a connection-retry policy with exponential backoff. If Redis is unreachable after retries, their health checks will fail, and the processes will exit loudly.

### **Agent crashes**
If an agent container dies, the orchestrator will post a Failure artefact and terminate the parent Claim. The work is considered lost.

### **Partial failures**
Each claim phase is an atomic, all-or-nothing transaction. If any agent in a parallel phase fails, the orchestrator will terminate the parent Claim and will not proceed to the next phase.

### **Failure recovery**
When a Failure artefact is created, the workflow for that claim stops. For V1, the only next step is manual intervention. A human operator must inspect the failure, diagnose the problem, and restart the entire workflow from the beginning with `holt forage`. Resuming or restarting a failed workflow is not supported in V1.

## **Professional standards**

* **Comprehensive testing**: A minimum of 50% test coverage is required, with a healthy mix of unit, integration, and E2E tests.  
* **CI/CD automation**: All builds, tests, and security scans are managed via GitHub Actions.  
* **Makefile automation**: All common development tasks (build, test, lint, clean) are available as make targets.

## Phased delivery plan

Holt is being developed through a series of well-defined phases, each delivering a significant leap in capabilities. The project's status is tracked against this roadmap.

### Phase 1: "Heartbeat" ✅
*Goal: Prove the core blackboard architecture works with basic orchestrator and CLI functionality.*
- **Features:** Redis blackboard with Pub/Sub, CLI for instance management, basic orchestrator claim engine.

### Phase 2: "Single Agent" ✅
*Goal: Enable a single agent to perform a complete, useful task.*
- **Features:** Agent `pup` implementation, claim bidding, Git workspace integration, and context assembly.

### Phase 3: "Coordination" ✅
*Goal: Orchestrate multiple, specialized agents in a collaborative workflow.*
- **Features:** Multi-stage pipelines (review → parallel → exclusive), controller-worker scaling pattern, consensus bidding, automated feedback loops, and powerful CLI observability features.

### Phase 4: "Human-in-the-Loop" 🚧
*Goal: Make the system production-ready with human oversight.*
- **Features:** `Question`/`Answer` artefacts for human guidance and mandatory approval gates for critical actions.

### Phase 5: "Complex Coordination" 📋
*Goal: Enable the orchestration of complex, non-linear workflows (DAGs).*
- **Features:** Support for "fan-in" synchronization patterns and conditional workflow pathing based on agent bidding logic.

### Phase 6: "Kubernetes-Native" 📋
*Goal: Evolve Holt into a first-class, native Kubernetes platform.*
- **Features:** A **Holt Operator** for managing instances via Custom Resource Definitions (CRDs), native integration with Kubernetes networking and storage, and **Prometheus metrics endpoints**.

### Future Enhancements
For a detailed look at long-term, enterprise-focused ideas like RBAC, Secrets Management, and High Availability, see the living document at **[design/future-enhancements.md](./design/future-enhancements.md)**.

## **Future Work**

### **Role-Based Access Control (RBAC)**

As Holt matures and deployments grow, we anticipate the need for access control around destructive operations and sensitive data.

**Scope of RBAC (Post-V1):**
- **Authentication**: Identify users and API clients accessing Holt commands
- **Authorization**: Control who can execute destructive operations like `holt destroy`
- **Audit trails**: Log all administrative actions with user attribution
- **Role definitions**: Define roles such as `admin`, `operator`, `viewer`
- **Protected operations**: Restrict commands like:
  - `holt destroy` - Permanent data deletion
  - `holt down --force` - Forceful instance termination
  - Modification of production instances
  - Access to sensitive artefact payloads

**Implementation Approach:**
- Redis ACLs for connection-level access control
- CLI token-based authentication (environment variable or config file)
- Orchestrator API authentication for programmatic access
- Integration with enterprise identity providers (LDAP, OAuth, SAML)

**Priority:** Post-V1 enhancement, prioritized based on enterprise adoption and regulatory requirements.

**Documentation Reference:** See `design/features/future/rbac.md` (to be created when needed)