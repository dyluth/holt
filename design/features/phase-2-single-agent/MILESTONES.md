# **Phase 2: "Single Agent" - Implementation Milestones**

**Phase Goal**: One agent can claim and execute work with full Git integration.

**Phase Success Criteria**:
- `sett forage --goal "test"` creates initial artefact
- Orchestrator creates corresponding claim
- Agent cub bids on claim and wins
- Agent executes work and creates Git commit artefact
- Full audit trail visible on blackboard (GoalDefined → CodeCommit)
- Git workspace integration functional (clean repo validation, commit workflow)

---

## **Milestone Overview**

Phase 2 is broken down into **5 implementable milestones** that build the single-agent execution system in dependency order:

### **M2.1: Agent Cub Foundation**
**Status**: Design Complete ✅
**Dependencies**: M1.1, M1.2
**Estimated Effort**: Medium

**Goal**: Establish the agent cub binary with foundational concurrent architecture, configuration management, and health monitoring infrastructure.

**Scope**:
- New binary `cmd/cub/main.go` with entrypoint
- Configuration loading from environment variables:
  - `SETT_INSTANCE_NAME` (required)
  - `SETT_AGENT_NAME` (required)
  - `REDIS_URL` (required)
- Redis blackboard connection using existing `pkg/blackboard` client
- Health check HTTP server (`GET /healthz`) with Redis PING
- Two-goroutine concurrent architecture (fully structured, placeholder work):
  - Claim Watcher goroutine (proper select loop, responds to shutdown)
  - Work Executor goroutine (proper select loop, responds to shutdown)
  - Work queue channel (`chan *blackboard.Claim`, buffer size 1)
- Graceful shutdown handling (SIGINT/SIGTERM, <5 second timeout)
- Fail-fast configuration validation

**Deliverables**:
- `cmd/cub/main.go` - Cub binary entrypoint
- `internal/cub/config.go` - Configuration struct, LoadConfig(), Validate()
- `internal/cub/engine.go` - Engine struct, Start(), goroutine methods
- `internal/cub/health.go` - HealthServer, /healthz handler
- Unit tests for config validation, health checks, engine lifecycle (90%+ coverage)
- Integration tests with testcontainers-go (binary + Redis, no Docker image)
- Makefile targets: `build-cub`, `test-cub`

**Design Document**: ✅ [M2.1-agent-cub-foundation.md](./M2.1-agent-cub-foundation.md)

---

### **M2.2: Claim Watching & Bidding**
**Status**: Not Started
**Dependencies**: M2.1, M1.5
**Estimated Effort**: Medium

**Goal**: Implement claim event subscription, bidding logic, and enhance orchestrator with full consensus model and granting.

**Scope**:
- **Agent Cub enhancements:**
  - Claim Watcher subscribes to `sett:{instance}:claim_events` Pub/Sub channel
  - Implement basic bidding logic: submit "exclusive" bid for any new claim
  - Bid submission via Redis HSET to `sett:{instance}:claim:{uuid}:bids`
  - Poll claim status after bidding to detect grant
- **Orchestrator enhancements (M1.5 updates):**
  - Implement full consensus model: wait for bid from all known agents (single agent in Phase 2)
  - Load agent registry from `sett.yml` configuration
  - Implement claim granting logic for exclusive phase
  - Update claim status: `pending_review` → `pending_exclusive` → `complete`
  - Publish granted claim to enable agent to proceed
- **CLI enhancements:**
  - Parse agent definitions from `sett.yml`
  - Launch agent containers with cub entrypoint
  - Pass required environment variables to agent containers
  - Mount workspace (read-only for M2.2, read-write in M2.4)
- **Docker integration:**
  - Create example agent Dockerfile with cub binary
  - Container naming conventions: `sett-{instance}-agent-{name}`
  - Network integration: agents connect to Redis

**Deliverables**:
- `internal/cub/watcher.go` - Claim Watcher with Pub/Sub and bidding
- `internal/orchestrator/consensus.go` - Full consensus bidding model
- `internal/orchestrator/granting.go` - Claim granting logic
- `cmd/sett/commands/up.go` - Enhanced to launch agent containers
- `agents/example-echo-agent/Dockerfile` - Example agent image
- Integration tests: cub receives claim event, submits bid, orchestrator grants
- E2E test: `sett up` launches agent, agent connects and bids on test claim

**Design Document**: `M2.2-claim-watching-bidding.md`

---

### **M2.3: Basic Tool Execution**
**Status**: Not Started
**Dependencies**: M2.2
**Estimated Effort**: Medium

**Goal**: Implement the work executor loop and tool execution contract (stdin/stdout) with a simple deterministic agent.

**Scope**:
- **Work Executor implementation:**
  - Read granted claim from work queue channel
  - Fetch target artefact from blackboard
  - Format tool input (JSON to stdin): `{"claim_type": "exclusive", "target_artefact": {...}, "context_chain": []}`
  - Execute agent-specific command script (from `sett.yml` agent definition)
  - Parse tool output (JSON from stdout): `{"artefact_type": "...", "artefact_payload": "...", "summary": "..."}`
  - Create and post new artefact to blackboard
  - Publish artefact to `artefact_events` channel
- **Tool execution contract:**
  - **Input schema (stdin):** JSON with `claim_type`, `target_artefact`, `context_chain`
  - **Output schema (stdout):** JSON with `artefact_type`, `artefact_payload`, `summary`
  - **Error handling:** Non-zero exit code or invalid JSON → create Failure artefact
- **Example echo agent:**
  - Simple shell script: reads stdin, logs claim info, outputs success JSON
  - No Git operations, no LLM calls (deterministic)
  - Agent type: "EchoSuccess" artefact
- **Error handling:**
  - Script exit code validation
  - JSON schema validation
  - Failure artefact creation with error details
  - Claim termination on agent failure

**Deliverables**:
- `internal/cub/executor.go` - Work Executor loop, tool command execution
- `internal/cub/contract.go` - Tool contract types (stdin/stdout JSON schemas)
- `agents/example-echo-agent/run.sh` - Echo agent script
- `agents/example-echo-agent/Dockerfile` - Updated with run.sh
- Integration tests: claim → grant → execute → artefact creation
- E2E test: Full workflow with echo agent (forage → claim → bid → execute → artefact)

**Design Document**: `M2.3-basic-tool-execution.md`

---

### **M2.4: Context Assembly & Git Integration**
**Status**: Not Started
**Dependencies**: M2.3
**Estimated Effort**: Large

**Goal**: Implement the context assembly algorithm (the "brain" of the cub) and Git-based agent execution with workspace integration.

**Scope**:
- **Context assembly algorithm:**
  - Breadth-first traversal of `source_artefacts` graph
  - Thread tracking: for each logical_id, fetch latest version via `sett:{instance}:thread:{logical_id}` ZSET
  - De-duplication: use map keyed by logical_id
  - Depth limit: 10 levels maximum (safety valve)
  - Assemble context_chain array for tool input
- **Git workspace integration:**
  - **CLI enhancements:**
    - Verify clean Git repository before `sett up`
    - Detect workspace path (Git repository root)
    - Mount workspace into agent containers as volume
    - Validate no uncommitted changes or untracked files
  - **Mount configuration:**
    - Read-write mount for Git-based agents
    - Agent's working directory = repository root
  - **Git state management:**
    - Agent script can execute Git commands (checkout, add, commit)
    - Workspace remains clean between executions (Phase 2: single agent only)
- **Git-based example agent:**
  - Receives context (target artefact with instructions)
  - Creates or modifies a file in workspace
  - Commits changes with descriptive message
  - Returns commit hash as `artefact_payload`
  - Artefact type: "CodeCommit"
- **Workspace validation:**
  - Pre-flight checks: `.git` exists, working directory clean
  - Error handling: fail `sett up` if workspace invalid

**Deliverables**:
- `internal/cub/context.go` - Context assembly algorithm
- `internal/cub/thread.go` - Thread tracking and latest version lookup
- `cmd/sett/commands/up.go` - Git workspace validation and mounting
- `internal/git/` - Git workspace validation utilities
- `agents/example-git-agent/run.sh` - Git-based agent script
- `agents/example-git-agent/Dockerfile` - Git-enabled agent image
- Integration tests: context chain assembly (multi-level source_artefacts)
- Integration tests: Git operations (commit, hash retrieval)
- E2E test: Git agent creates file, commits, returns hash as CodeCommit artefact

**Design Document**: `M2.4-context-assembly-git-integration.md`

---

### **M2.5: End-to-End Validation**
**Status**: Not Started
**Dependencies**: M2.4
**Estimated Effort**: Small

**Goal**: Complete Phase 2 with comprehensive integration testing, failure scenario validation, and documentation updates.

**Scope**:
- **End-to-end test suite:**
  - Complete workflow: `sett forage --goal "create hello.txt"` → orchestrator creates claim → Git agent bids → executes → creates file → commits → posts CodeCommit artefact
  - Verify audit trail: GoalDefined artefact → CodeCommit artefact with source_artefacts chain
  - Validate Git commit exists in repository with correct hash
  - Verify workspace remains clean after execution
- **Failure scenario testing:**
  - Agent script fails (non-zero exit) → Failure artefact created, claim terminated
  - Agent script outputs invalid JSON → Failure artefact created
  - Agent container crashes → orchestrator detects failure, creates Failure artefact
  - Redis unavailable → agent health check fails, cub exits gracefully
  - Git workspace dirty → `sett up` fails with clear error message
- **Performance validation:**
  - Startup time: `sett up` completes in <10 seconds
  - Claim-to-execution latency: <2 seconds from claim creation to agent execution start
  - Context assembly: handles 10-level graph in <1 second
- **Documentation updates:**
  - Update main README with Phase 2 status
  - Document agent development workflow (creating new agents)
  - Document Git workspace requirements
  - Update troubleshooting guide with agent-related issues
- **Regression testing:**
  - Verify all Phase 1 tests still pass
  - Verify multi-instance support still works (multiple setts with different agents)

**Deliverables**:
- `cmd/sett/commands/e2e_test.go` - Complete Phase 2 E2E test suite
- `cmd/orchestrator/orchestrator_integration_test.go` - Enhanced with agent scenarios
- `internal/cub/cub_integration_test.go` - Comprehensive failure scenario tests
- Updated `README.md` with Phase 2 completion status
- Updated `docs/agent-development.md` (new guide for creating agents)
- Updated `docs/troubleshooting.md` with agent-related issues
- Performance benchmark results documented

**Design Document**: `M2.5-end-to-end-validation.md`

---

## **Milestone Dependency Graph**

```
                    ┌─────────────────────┐
                    │  M1.1: Blackboard   │
                    │  Foundation (Types) │
                    └──────────┬──────────┘
                               │
                               ▼
                    ┌─────────────────────┐
                    │  M1.2: Blackboard   │
                    │  Client Operations  │
                    └──────┬───────┬──────┘
                           │       │
              ┌────────────┘       └────────────┐
              ▼                                 ▼
   ┌─────────────────────┐         ┌─────────────────────┐
   │  M1.5: Orchestrator │         │  M2.1: Agent Cub    │
   │  Claim Engine       │         │  Foundation         │
   └──────────┬──────────┘         └──────────┬──────────┘
              │                                 │
              │                    ┌────────────┘
              │                    ▼
              │         ┌─────────────────────┐
              │         │  M2.2: Claim        │
              │         │  Watching & Bidding │
              └─────────►  (Orch + Cub + CLI) │
                        └──────────┬──────────┘
                                   │
                                   ▼
                        ┌─────────────────────┐
                        │  M2.3: Basic Tool   │
                        │  Execution (Echo)   │
                        └──────────┬──────────┘
                                   │
                                   ▼
                        ┌─────────────────────┐
                        │  M2.4: Context &    │
                        │  Git Integration    │
                        └──────────┬──────────┘
                                   │
                                   ▼
                        ┌─────────────────────┐
                        │  M2.5: E2E          │
                        │  Validation         │
                        └─────────────────────┘
```

## **Implementation Order**

The milestones must be implemented in strict sequential order due to dependencies:

**Wave 1: Foundation**
- **M2.1**: Agent Cub Foundation (depends on M1.1, M1.2)

**Wave 2: Integration**
- **M2.2**: Claim Watching & Bidding (depends on M2.1, M1.5)

**Wave 3: Execution**
- **M2.3**: Basic Tool Execution (depends on M2.2)

**Wave 4: Intelligence**
- **M2.4**: Context Assembly & Git Integration (depends on M2.3)

**Wave 5: Validation**
- **M2.5**: End-to-End Validation (depends on M2.4)

## **Phase 2 Completion Criteria**

Phase 2 is complete when:
- ✅ All 5 milestones have their Definition of Done satisfied
- ✅ End-to-end test passes: `sett forage --goal "create hello.txt"` → agent bids → executes → creates Git commit artefact
- ✅ Full audit trail visible on blackboard:
  - GoalDefined artefact with `type: "GoalDefined"`, `payload: "create hello.txt"`
  - CodeCommit artefact with `type: "CodeCommit"`, `payload: "<commit-hash>"`, `source_artefacts: [GoalDefined.id]`
- ✅ Git workspace integration functional:
  - `sett up` validates clean repository
  - Agent creates file `hello.txt` in workspace
  - Agent commits changes with proper message
  - Workspace remains clean after execution
- ✅ Agent cub lifecycle management:
  - Cub connects to Redis and stays healthy
  - Cub bids on claims reliably
  - Cub executes work and posts artefacts
  - Cub shuts down gracefully on `sett down`
- ✅ No regressions in Phase 1 tests
- ✅ All core data structures are implemented and tested
- ✅ Documentation is complete (agent development guide, updated README)
- ✅ Test coverage maintained or improved (90%+ for new packages)

## **Testing Strategy**

Each milestone includes:
- **Unit tests**: For isolated logic (config parsing, context assembly, JSON serialization)
- **Integration tests**: With real Redis and Docker (using testcontainers-go)
- **E2E tests**: User-facing workflows from CLI perspective

**Phase 2 E2E Test Suite** (M2.5):
1. **Agent cub foundation**: Cub starts, connects to Redis, responds to health checks, shuts down gracefully
2. **Bidding workflow**: Orchestrator creates claim → agent receives event → agent bids → orchestrator grants
3. **Echo agent execution**: Echo agent receives claim → executes → posts EchoSuccess artefact
4. **Git agent execution**: Git agent receives claim → assembles context → creates file → commits → posts CodeCommit artefact
5. **Failure scenarios**: Agent crashes → Failure artefact created, claim terminated
6. **State verification**: Redis contains expected artefacts, claims, and thread tracking data

## **Key Architectural Decisions**

### **Standard Mode Only (replicas: 1)**
Phase 2 implements only the standard operational mode where a single agent container runs persistently with both Claim Watcher and Work Executor goroutines active. The controller-worker pattern (for `replicas > 1`) is explicitly deferred to Phase 3.

**Rationale**: Simplifies Phase 2 to focus on proving the core single-agent workflow before adding multi-instance complexity.

### **No Review or Parallel Phases**
The orchestrator in Phase 2 only implements the exclusive phase. All claims are granted exclusively to the single agent. Review and parallel phases are deferred to Phase 3 (multi-agent coordination).

**Rationale**: Phase 2's "single agent" constraint makes review and parallel phases unnecessary. Implementing them would violate YAGNI.

### **Deterministic Agent First (M2.3), Git Agent Second (M2.4)**
We build up complexity gradually:
1. **M2.3**: Echo agent (simple script, no Git, no LLM) proves the tool execution contract works
2. **M2.4**: Git agent (file creation, commits) proves the Git workflow and context assembly

**Rationale**: Incremental complexity reduces integration risk and makes debugging easier.

### **Context Assembly Algorithm**
The breadth-first graph traversal with thread tracking (logical_id → latest version) is the core intelligence of the cub. This ensures agents always receive the most recent context without manual version management.

**Rationale**: Aligns with Sett's immutable artefact model while providing stateful-like behavior through graph traversal.

### **Fail-Fast Configuration Validation**
All environment variable validation happens at cub startup, before any Redis connection or goroutine launch. Invalid configuration causes immediate exit with clear error messages.

**Rationale**: Provides fast feedback loop for developers and prevents partial initialization failures.

## **Non-Goals (Explicitly Deferred)**

### **Deferred to Phase 3: "Coordination"**
- Multi-agent coordination (review, parallel, exclusive phases)
- Controller-worker pattern for agent scaling (`replicas > 1`)
- Bid strategy selection (currently hardcoded to "exclusive")
- Timeout-based consensus (currently waits indefinitely)
- Agent-to-agent communication

### **Deferred to Phase 4: "Human-in-the-Loop"**
- Question/Answer artefacts
- Human approval gates
- `sett questions` and `sett answer` commands
- Interactive workflows

### **Future Enhancements (Beyond Phase 4)**
- Dynamic agent registration (currently static from `sett.yml`)
- Agent hot-reloading (currently requires `sett down && sett up`)
- Advanced context assembly (custom graph traversal strategies)
- LLM-based bidding strategies
- Artefact retention policies

## **Risk Management**

### **High-Risk Areas**

1. **Concurrent goroutine complexity (M2.1, M2.2)**
   - **Risk**: Race conditions, deadlocks, goroutine leaks
   - **Mitigation**: Comprehensive integration tests with `-race` flag, explicit timeout enforcement

2. **Docker container integration (M2.2)**
   - **Risk**: Container networking issues, volume mount problems
   - **Mitigation**: Use Docker SDK best practices, test with testcontainers-go

3. **Git workspace state management (M2.4)**
   - **Risk**: Workspace corruption, uncommitted changes, merge conflicts
   - **Mitigation**: Strict clean-repository validation, fail-fast on dirty workspace

4. **Context assembly performance (M2.4)**
   - **Risk**: Graph traversal slowdowns with deep chains
   - **Mitigation**: 10-level depth limit, benchmark tests in M2.5

### **Medium-Risk Areas**

5. **Tool execution contract reliability (M2.3)**
   - **Risk**: Agents output malformed JSON or unexpected formats
   - **Mitigation**: Strict JSON schema validation, comprehensive error handling

6. **Orchestrator consensus timing (M2.2)**
   - **Risk**: Orchestrator waits indefinitely for dead agent
   - **Mitigation**: Document this as V1 limitation, defer timeout logic to Phase 3

## **Success Metrics**

### **Functional Metrics**
- ✅ Single-agent workflow completes successfully (GoalDefined → CodeCommit)
- ✅ Zero crashes or panics in 100 consecutive workflow executions
- ✅ All Phase 1 tests continue to pass (no regressions)

### **Performance Metrics**
- ✅ Cub startup time: <1 second
- ✅ Claim-to-execution latency: <2 seconds
- ✅ Context assembly (10-level graph): <1 second
- ✅ Git commit operation: <5 seconds

### **Quality Metrics**
- ✅ Test coverage: 90%+ for all new packages (`internal/cub`, `internal/git`)
- ✅ Zero race conditions detected (all tests pass with `-race`)
- ✅ Zero goroutine leaks (verified in integration tests)

### **Developer Experience Metrics**
- ✅ Onboarding time (git clone → running workflow): <10 minutes
- ✅ Clear error messages for all common failure scenarios
- ✅ Documentation complete and accurate

## **Next Steps**

After Phase 2 completion, proceed to **Phase 3: "Coordination"** which adds:
- Multi-agent coordination (review → parallel → exclusive phases)
- Full consensus bidding model with multiple agents
- Controller-worker pattern for agent scaling (`replicas > 1`)
- Enhanced error handling and failure recovery
- Sophisticated claim lifecycle management
