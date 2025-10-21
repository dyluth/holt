# **Phase 2: "Single Agent" - Basic Execution**

**Goal**: One agent can claim and execute work with full Git integration.

## **Phase Success Criteria**

- End-to-end workflow: forage → claim → execute → artefact
- Agent can modify code and commit results
- Full audit trail on blackboard
- Git workspace integration functional

## **Implementation Milestones**

Phase 2 is broken down into **5 implementable milestones** that build single-agent functionality:

### **M2.1: Agent Pup Foundation**
**Status**: Design Complete ✅
**Dependencies**: M1.1, M1.2
**Estimated Effort**: Medium

**Goal**: Establish the agent pup binary with foundational concurrent architecture, configuration management, and health monitoring.

**Scope**:
- New binary `cmd/pup/main.go` with entrypoint
- Configuration loading from environment variables (`HOLT_INSTANCE_NAME`, `HOLT_AGENT_NAME`, `REDIS_URL`)
- Redis blackboard connection using `pkg/blackboard` client
- Health check endpoint (`/healthz`) with Redis PING
- Two-goroutine architecture structure (Claim Watcher, Work Executor) with placeholder work
- Graceful shutdown handling (SIGINT/SIGTERM)

**Deliverables**:
- `cmd/pup/main.go` - Pup binary entrypoint
- `internal/pup/config.go` - Configuration loading and validation
- `internal/pup/engine.go` - Concurrent engine with goroutine structure
- `internal/pup/health.go` - Health check HTTP server
- Unit and integration tests (90%+ coverage)
- Makefile targets: `build-pup`, `test-pup`

**Design Document**: ✅ [M2.1-agent-pup-foundation.md](./M2.1-agent-pup-foundation.md)

---

### **M2.2: Claim Watching & Bidding**
**Status**: Not Started
**Dependencies**: M2.1, M1.5
**Estimated Effort**: Medium

**Goal**: Implement claim event subscription, bidding logic, and orchestrator full consensus model.

**Scope**:
- Pup subscribes to `claim_events` Pub/Sub channel
- Basic bidding logic (submit "exclusive" bid for any new claim)
- Orchestrator enhancement: full consensus model (wait for single agent's bid)
- Orchestrator enhancement: claim granting logic
- Docker container integration (CLI launches agent containers)

**Deliverables**:
- Enhanced Claim Watcher goroutine with Pub/Sub subscription
- Bid submission logic
- Enhanced orchestrator claim engine (consensus + granting)
- Integration tests: pup receives claim, submits bid, orchestrator grants

**Design Document**: `M2.2-claim-watching-bidding.md`

---

### **M2.3: Basic Tool Execution**
**Status**: Not Started
**Dependencies**: M2.2
**Estimated Effort**: Medium

**Goal**: Implement the work executor loop and tool execution contract with a simple deterministic agent.

**Scope**:
- Work Executor goroutine implementation (reads from work queue)
- Tool execution contract: stdin (JSON context) → stdout (JSON artefact)
- Simple "echo" agent: receives claim, logs it, creates "Success" artefact
- Artefact creation and posting to blackboard
- Basic error handling (Failure artefacts)

**Deliverables**:
- Work Executor loop with tool command execution
- Tool contract implementation (stdin/stdout JSON)
- Example echo agent (deterministic, no Git)
- Integration tests: claim → bid → grant → execute → artefact

**Design Document**: `M2.3-basic-tool-execution.md`

---

### **M2.4: Context Assembly & Git Integration**
**Status**: Not Started
**Dependencies**: M2.3
**Estimated Effort**: Large

**Goal**: Implement context assembly algorithm and Git-based agent execution.

**Scope**:
- Context assembly algorithm (breadth-first graph traversal)
- Thread tracking (follow `source_artefacts`, get latest version via logical_id)
- Depth limits and safety (10-level max)
- Git workspace mounting for containers
- Git-based agent: creates file, commits changes, returns commit hash
- Workspace validation (clean repository requirements)

**Deliverables**:
- Context assembly logic in Work Executor
- Git workspace integration
- Example Git agent (creates file, commits, returns hash)
- Integration tests: context chain assembly, Git operations, commit artefacts

**Design Document**: `M2.4-context-assembly-git-integration.md`

---

### **M2.5: End-to-End Validation**
**Status**: Not Started
**Dependencies**: M2.4
**Estimated Effort**: Small

**Goal**: Complete Phase 2 with full integration testing and validation.

**Scope**:
- End-to-end test: `holt forage` → orchestrator creates claim → agent bids → agent executes → creates Git commit artefact
- Failure scenario testing (agent failures, container crashes)
- Performance validation
- Documentation updates

**Deliverables**:
- Complete E2E test suite for single-agent workflow
- Failure handling validation
- Phase 2 success criteria verification
- Updated documentation

**Design Document**: `M2.5-end-to-end-validation.md`

---

## **Milestone Dependency Graph**

```
    ┌─────────────────────┐
    │  M1.1: Blackboard   │
    │  Foundation         │
    └──────────┬──────────┘
               │
               ▼
    ┌─────────────────────┐
    │  M1.2: Blackboard   │
    │  Client Operations  │
    └──────────┬──────────┘
               │
               ▼
    ┌─────────────────────┐
    │  M2.1: Agent Pup    │◄──────────┐
    │  Foundation         │           │
    └──────────┬──────────┘           │
               │                      │
               ▼                      │
    ┌─────────────────────┐           │
    │  M2.2: Claim        │           │
    │  Watching & Bidding │           │
    └──────────┬──────────┘           │
               │                      │
               ▼              ┌───────┴────────┐
    ┌─────────────────────┐  │  M1.5: Orch.   │
    │  M2.3: Basic Tool   │  │  Claim Engine  │
    │  Execution          │  └────────────────┘
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

## **Implementation Constraints**

- Single agent only (no multi-agent coordination)
- Standard operational mode only (`replicas: 1`) - controller-worker pattern deferred to Phase 3
- No review or parallel phases (Phase 3 dependency)
- Basic error handling (enhanced in Phase 3)
- No human interaction (Phase 4 dependency)

## **Testing Requirements**

- Agent pup unit and integration tests (90%+ coverage)
- Git workflow end-to-end testing
- Container execution testing
- Blackboard state verification
- Tool execution contract validation

## **Dependencies**

- **Phase 1**: Requires functional blackboard and orchestrator (M1.1-M1.6)
- Clean Git repository and Docker environment
- testcontainers-go for integration testing

## **Phase 2 Completion Criteria**

Phase 2 is complete when:
- ✅ All 5 milestones have their Definition of Done satisfied
- ✅ End-to-end test passes: `holt forage --goal "test"` → agent bids → executes → creates Git commit artefact
- ✅ Full audit trail visible on blackboard (artefact chain from GoalDefined to CodeCommit)
- ✅ Git workspace integration functional (clean repo validation, commit workflow)
- ✅ No regressions in Phase 1 tests
- ✅ Documentation complete

## **Deliverables**

- Functional agent pup binary (`cmd/pup`)
- Working agent container execution
- Git integration with commit workflow
- Single-agent end-to-end workflow
- Complete test coverage for agent functionality