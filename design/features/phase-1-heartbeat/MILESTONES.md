# **Phase 1: "Heartbeat" - Implementation Milestones**

**Phase Goal**: Prove the blackboard architecture works with basic orchestrator and CLI functionality.

**Phase Success Criteria**:
- `holt forage --goal "hello world"` creates initial artefact
- Orchestrator creates corresponding claim
- System state visible via Redis CLI
- All core data structures implemented and functional

---

## **Milestone Overview**

Phase 1 is broken down into **6 implementable milestones** that build the foundation in dependency order:

### **M1.1: Redis Blackboard Foundation**
**Status**: Design Complete ✅
**Dependencies**: None
**Estimated Effort**: Small

**Goal**: Establish core data structures and Redis schema as Go types

**Scope**:
- Go type definitions (Artefact, Claim, Bid structs)
- Redis key pattern constants and helpers
- JSON serialization/deserialization functions
- Thread tracking ZSET utility functions
- Pub/Sub channel constants

**Deliverables**:
- `pkg/blackboard/types.go` - Core data structures
- `pkg/blackboard/schema.go` - Redis key patterns
- `pkg/blackboard/serialization.go` - Hash conversion helpers
- `pkg/blackboard/thread.go` - Thread tracking logic
- Unit tests for serialization and key generation (90%+ coverage)

**Design Document**: ✅ [M1.1-redis-blackboard-foundation.md](./M1.1-redis-blackboard-foundation.md)

---

### **M1.2: Blackboard Client Operations**
**Status**: Not Started
**Dependencies**: M1.1
**Estimated Effort**: Medium

**Goal**: Implement Redis client with CRUD operations for all blackboard entities

**Scope**:
- Redis connection management with retry logic
- Pub/Sub channel setup and subscription
- CRUD operations for Artefacts, Claims, and Bids
- Thread tracking ZSET operations (add version, get latest)
- Health check integration

**Deliverables**:
- `pkg/blackboard/client.go` - Redis client interface and implementation
- `pkg/blackboard/artefacts.go` - Artefact operations
- `pkg/blackboard/claims.go` - Claim operations
- `pkg/blackboard/pubsub.go` - Pub/Sub helpers
- Integration tests with Redis (using testcontainers-go)

**Design Document**: `blackboard-client-operations.md`

---

### **M1.3: CLI Project Initialization**
**Status**: Not Started
**Dependencies**: None
**Estimated Effort**: Small

**Goal**: Enable developers to bootstrap new Holt projects

**Scope**:
- `holt init` command implementation
- Project scaffolding logic (create directories, files)
- Template generation for:
  - `holt.yml` (with commented example agent)
  - `agents/` directory structure
  - `agents/example-agent/` with Dockerfile and run.sh

**Deliverables**:
- `cmd/holt/commands/init.go` - Init command
- `internal/templates/` - Embedded templates for scaffolding
- E2E test: Run `holt init` and verify file structure

**Design Document**: `cli-project-initialization.md`

---

### **M1.4: CLI Lifecycle Management**
**Status**: Not Started
**Dependencies**: M1.2, M1.3
**Estimated Effort**: Large

**Goal**: Manage Holt instance lifecycle with workspace safety and multi-instance support

**Scope**:
- `holt up [--name <instance>] [--force]` - Start Redis + orchestrator containers
- `holt down [--name <instance>]` - Stop and cleanup containers
- `holt list` - List all active Holt instances
- Docker SDK integration
- Container naming conventions (instance-based namespacing)
- Network and volume management
- Parse and validate `holt.yml` configuration
- **Implement instance name locking (`holt:{name}:lock`) to prevent duplicate active instances**
- **Implement atomic counter (`holt:instance_counter`) for generating default instance names**
- **Implement workspace path check using a global `holt:instances` hash**
- **Add `--force` flag to `holt up` to override workspace path collisions**
- **Define the `holt:instances` hash structure, including `run_id`, `workspace_path`, and `started_at` fields**

**Deliverables**:
- `cmd/holt/commands/up.go` - Up command with workspace safety
- `cmd/holt/commands/down.go` - Down command with cleanup
- `cmd/holt/commands/list.go` - List command
- `internal/docker/` - Docker SDK wrapper
- `internal/config/` - holt.yml parser and validator
- `internal/instance/` - Instance locking, naming, and workspace tracking
- Integration tests for full lifecycle including workspace collision detection

**Design Document**: `cli-lifecycle-management.md`

---

### **M1.5: Orchestrator Claim Engine**
**Status**: Not Started
**Dependencies**: M1.2
**Estimated Effort**: Large

**Goal**: Implement basic orchestrator that watches artefacts and creates claims

**Scope**:
- Orchestrator main loop with Pub/Sub subscription
- Artefact event handling (watch `artefact_events` channel)
- Claim creation logic for new artefacts
- Claim lifecycle state management (pending_review state only for Phase 1)
- Health check endpoint (`/healthz`)
- Graceful shutdown handling

**Deliverables**:
- `cmd/orchestrator/main.go` - Orchestrator entrypoint
- `internal/orchestrator/engine.go` - Core orchestration logic
- `internal/orchestrator/claims.go` - Claim creation and management
- `internal/orchestrator/health.go` - Health check HTTP server
- Unit tests for claim creation logic
- Integration tests with Redis

**Design Document**: `orchestrator-claim-engine.md`

---

### **M1.6: Workflow Initiation (Forage)**
**Status**: Not Started
**Dependencies**: M1.2, M1.4, M1.5
**Estimated Effort**: Small

**Goal**: Enable users to start workflows with `holt forage`

**Scope**:
- `holt forage --goal "description"` command
- GoalDefined artefact creation with proper structure
- Publish to `artefact_events` Pub/Sub channel
- Verify orchestrator receives and creates claim

**Deliverables**:
- `cmd/holt/commands/forage.go` - Forage command
- End-to-end test: Run forage, verify artefact and claim creation
- Validation of Phase 1 success criteria

**Design Document**: `workflow-initiation-forage.md`

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
   │  M1.5: Orchestrator │         │  M1.3: CLI Project  │
   │  Claim Engine       │         │  Initialization     │
   └──────────┬──────────┘         └──────────┬──────────┘
              │                                 │
              │                    ┌────────────┘
              │                    ▼
              │         ┌─────────────────────┐
              │         │  M1.4: CLI Lifecycle│
              │         │  Management (up/down)│
              │         └──────────┬──────────┘
              │                    │
              └────────────────────┼─────────────┐
                                   │             │
                                   ▼             │
                        ┌─────────────────────┐  │
                        │  M1.6: Workflow     │◄─┘
                        │  Initiation (forage)│
                        └─────────────────────┘
```

## **Implementation Order**

The milestones should be implemented in the following order to respect dependencies:

**Wave 1** (Parallel):
- **M1.1**: Redis Blackboard Foundation
- **M1.3**: CLI Project Initialization

**Wave 2**:
- **M1.2**: Blackboard Client Operations (depends on M1.1)

**Wave 3** (Parallel):
- **M1.4**: CLI Lifecycle Management (depends on M1.2, M1.3)
- **M1.5**: Orchestrator Claim Engine (depends on M1.2)

**Wave 4** (Integration):
- **M1.6**: Workflow Initiation (depends on M1.2, M1.4, M1.5)

## **Phase 1 Completion Criteria**

Phase 1 is complete when:
- ✅ All 6 milestones have their Definition of Done satisfied
- ✅ End-to-end test passes: `holt init && holt up && holt forage --goal "hello world"`
- ✅ Orchestrator creates a claim for the GoalDefined artefact
- ✅ System state is visible via Redis CLI (`redis-cli KEYS "holt:*"`)
- ✅ All core data structures are implemented and tested
- ✅ No regressions in tests
- ✅ Documentation is complete

## **Testing Strategy**

Each milestone includes:
- **Unit tests**: For isolated logic (serialization, key generation, parsing)
- **Integration tests**: With real Redis instance (using testcontainers-go)
- **E2E tests**: User-facing workflows from CLI perspective

**Phase 1 E2E Test Suite**:
1. Project initialization: `holt init` creates correct file structure
2. Lifecycle management: `holt up` starts containers, `holt list` shows them, `holt down` cleans up
3. Workflow initiation: `holt forage --goal "test"` creates artefact and claim
4. State verification: Redis contains expected keys and data structures

## **Next Steps**

After Phase 1 completion, proceed to **Phase 2: "Single Agent"** which adds:
- Agent pup implementation
- Basic claim execution
- Git workspace integration
