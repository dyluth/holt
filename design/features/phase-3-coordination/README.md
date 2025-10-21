# **Phase 3: "Coordination" - Multi-Agent Workflow**

**Goal**: Review → Parallel → Exclusive phases working with multiple agent types.

## **Implementation Status**

### **M3.1: Multiple Agents & Enhanced Bidding System** ✅ **COMPLETE**
- Full consensus bidding model implemented
- Deterministic alphabetical tie-breaking for exclusive bids
- Agent registry management
- Grant notifications with claim tracking

### **M3.2: Review & Parallel Phase Execution** ✅ **COMPLETE**
- Three-phase claim execution (review → parallel → exclusive)
- Review phase with approval detection (JSON parsing)
- Single veto review logic (any rejection terminates claim)
- Parallel phase with multiple agent coordination
- Phase skipping for backward compatibility
- Atomic phase transitions with race condition prevention
- Unique role constraint enforcement
- Self-review support for review agents
- **Status**: Core functionality complete, tested, and backward compatible

### **M3.3: Automated Feedback Loop** ✅ **COMPLETE**
- Automatic review-based claim reassignment to original producer
- Feedback claims bypass bidding (pending_assignment status)
- Context assembly includes Review artefacts for agent feedback
- Automatic version management (Pup increments version transparently)
- Configurable iteration limits (`orchestrator.max_review_iterations`)
- Graceful termination with Failure artefacts for max iterations/missing agents
- Complete audit trail with termination reasons
- **Status**: Fully implemented with comprehensive test coverage

### **M3.4: Controller-Worker Pattern for Scaling** ✅ **COMPLETE**
- Controller-worker architecture for horizontal scaling
- Single persistent controller per role (eliminates bidding race conditions)
- Ephemeral workers launched on-demand by orchestrator
- Worker lifecycle management (launch → monitor → cleanup)
- Configurable concurrency limits (`worker.max_concurrent`)
- Stateless grant pausing when at max_concurrent limit
- Worker failure detection with Failure artefact creation
- Docker socket mounting for orchestrator container management
- Mode detection: HOLT_MODE=controller → controller, --execute-claim → worker
- **Status**: Fully implemented with E2E tests and backward compatibility

### **M3.5: Orchestrator Restart Resilience** ✅ **COMPLETE**
- Full orchestrator restart resilience with state recovery
- Phase state persisted to Redis on every transition
- Persistent grant queue with FIFO ordering (Redis ZSET)
- Automatic recovery of active claims on startup
- Re-triggering of incomplete grants (worker/traditional agents)
- Orphaned worker container cleanup
- Stale lock detection prevents race conditions
- **Status**: Production-ready restart resilience fully implemented

## **Phase Success Criteria**

- ✅ Complex workflow with review feedback loop (M3.3 complete)
- ✅ Multiple agents working in parallel (M3.2 complete)
- ✅ Controller-worker pattern for scaling (M3.4 complete)
- ⚠️ Basic error handling (no timeouts yet - deferred to M3.6)

## **Key Features Implemented**

1. **Consensus Bidding Model** (M3.1)
   - ✅ Full consensus waiting for all agent bids
   - ✅ Bid collection and validation
   - ✅ Agent registry management
   - ✅ Deterministic workflow execution

2. **Phased Execution System** (M3.2)
   - ✅ Review phase with feedback detection
   - ✅ Parallel phase coordination
   - ✅ Exclusive phase execution
   - ✅ Phase transition logic
   - ✅ Phase skipping (backward compatibility)

3. **Controller-Worker Pattern** (M3.4)
   - ✅ Scalable agent architecture with mode: "controller"
   - ✅ Controller mode (bidder-only) and worker mode (execute-only)
   - ✅ Ephemeral worker container management
   - ✅ Race condition elimination via single controller per role
   - ✅ Configurable concurrency limits (max_concurrent)
   - ✅ Automatic worker cleanup and failure handling

## **Implementation Constraints**

- No human interaction yet (Phase 4 dependency)
- Focus on agent coordination and workflow phases
- Enhanced error handling and failure recovery
- Production-level reliability required

## **Testing Requirements**

- ✅ Multi-agent coordination testing (M3.2 E2E tests implemented)
- ✅ Phase transition validation (unit and E2E tests)
- ⚠️ Failure scenario testing (basic coverage, timeouts deferred to M3.4)
- 🔜 Controller-worker pattern verification (deferred to M3.6)
- 🔜 Load testing with multiple agents (deferred - see Performance Testing note below)

### **Running M3.2 E2E Tests**

**IMPORTANT**: M3.2 E2E tests require rebuilding the orchestrator Docker image with the new phase execution code:

```bash
# 1. Rebuild orchestrator with M3.2 code (REQUIRED)
make docker-orchestrator

# 2. Run E2E tests
go test -tags=integration ./cmd/holt/commands -run TestE2E_Phase3 -v
```

The E2E tests will automatically build the required agent images (example-reviewer-agent, example-parallel-agent, example-git-agent) during test execution.

### **Performance Testing Note**

**Status**: Deferred (not critical for M3.2)

**Current Performance**: All M3.2 operations are sufficiently fast for typical workflows:
- Phase transitions: <100ms per transition
- Review payload parsing: <1ms per review
- Consensus bidding: <3 seconds for 3 agents
- In-memory phase state: negligible overhead

**Future Work**: Performance testing with multiple concurrent claims should be conducted when:
1. Production usage reveals performance bottlenecks
2. Controller-worker pattern (M3.6) is implemented for horizontal scaling
3. System is deployed in high-throughput environments (>100 concurrent claims)

**Recommendation**: Current implementation is optimized for correctness and auditability. Performance is adequate for development and small-to-medium production workloads. Large-scale performance testing should be conducted as part of M3.6 or Phase 4.

## **Dependencies**

- **Phase 1**: Functional blackboard and orchestrator
- **Phase 2**: Working single-agent execution
- Multiple agent types for testing

## **Deliverables**

- Full consensus bidding implementation
- Three-phase claim execution (review/parallel/exclusive)
- Controller-worker scaling pattern
- Comprehensive multi-agent workflows

---

## **M3.2 Known Limitations & Constraints**

The following limitations are **by design** in M3.2 and should be addressed in future milestones:

### **1. In-Memory Phase State (No Restart Resilience)**

**Limitation**: Phase tracking state is kept in-memory only, not persisted to Redis.

**Impact**:
- If the orchestrator restarts, all phase state is lost
- Claims in active phases (pending_review, pending_parallel, pending_exclusive) become stuck
- Manual intervention required to terminate stuck claims and restart workflows

**Monitoring Detection**: Claims in pending_* status for >30 minutes indicate stuck state.

**Future Resolution**: M3.3+ should persist phase state to Redis for restart resilience.

### **2. Unique Role Constraint (Breaking Change)**

**Limitation**: All agents must have unique roles in Phase 3.

**Impact**:
- Cannot have multiple agents with the same role (e.g., two "Coder" agents)
- M3.1 configs with duplicate roles will fail validation at `holt up` time
- Clear error message: `duplicate agent role 'X' found (agents 'A' and 'B'): all agents must have unique roles in Phase 3`

**Rationale**: Enables reliable artefact attribution using `produced_by_role` field for phase completion tracking.

**Future Resolution**: Phase 4+ could use `produced_by_agent` field to support duplicate roles.

### **3. No Automated Feedback Loop** ✅ **RESOLVED IN M3.3**

**Previous Limitation**: Review rejection terminated the claim; no automatic retry with feedback incorporated.

**M3.3 Resolution**:
- Review rejection now automatically creates feedback claims
- Original agent is reassigned with review feedback in context
- Automatic version management (v1→v2→v3) with iteration limits
- Complete audit trail of feedback loop iterations

### **4. No Runtime Failure Detection or Timeouts**

**Limitation**: Orchestrator waits indefinitely for granted agents to produce artefacts.

**Impact**:
- If a granted agent crashes, hangs, or is taking too long, the claim remains stuck
- No automatic timeout detection
- No automatic failure artefact creation
- Requires monitoring and manual intervention to detect and fix stuck claims

**Operational Workaround**: Monitor claims in active phases; manually terminate if agents fail.

**Future Resolution**: M3.3+ should implement configurable timeouts and failure detection.

### **5. No Parallel Agent Coordination Hints**

**Limitation**: Parallel agents are assumed to perform non-conflicting work.

**Impact**:
- No mechanism to coordinate which parallel agent works on which part of the task
- User responsible for designing agents that don't conflict (e.g., test agent + documentation agent)
- If parallel agents modify the same files, merge conflicts may occur

**Best Practice**: Design parallel agents to work on orthogonal concerns (testing vs docs vs linting).

**Future Resolution**: Phase 4+ LLM-based coordination could provide work hints to parallel agents.

### **6. Deterministic Phase Transitions Only**

**Limitation**: Phase transitions are based purely on artefact presence/content, not LLM reasoning.

**Impact**:
- Review approval is based on JSON parsing only (`{}` or `[]` = approval)
- No semantic understanding of review feedback
- Phase progression is mechanical, not intelligent

**Rationale**: Keeps M3.2 simple and deterministic; LLM reasoning is Phase 4 concern.

**Future Resolution**: Phase 4 could use LLM to interpret review feedback and make intelligent phase decisions.

---

## **Future Milestones (Immediate Priorities)**

The following requirements have been identified as immediate priorities for Phase 3 completion. These should be designed and implemented as the next milestones after M3.2.



### **M3.3: Automated Feedback Loop** ✅ **COMPLETE**

**Implemented**: M3.3 provides automated review-based iteration with version management.

**Implementation Summary**:
- Review rejection automatically creates feedback claims assigned to original producer
- Feedback claims bypass bidding via `pending_assignment` status
- Context assembly includes Review artefacts for agent feedback
- Pup automatically manages versioning (logical_id preservation, version increment)
- Configurable iteration limits prevent infinite loops (`orchestrator.max_review_iterations`)
- Graceful termination with Failure artefacts and clear termination reasons
- Complete audit trail of all feedback iterations

**Documentation**: See `design/features/phase-3-coordination/M3.3-automated-feedback-loop.md` for full design and implementation details.


### **M3.4: Controller-Worker Pattern for Scaling** ✅ **COMPLETE**

**Implementation Summary**: Controller-worker architecture fully operational.

**Delivered Features**:
- ✅ Configuration schema with `mode: "controller"` and nested `worker:` block
- ✅ Validation with default max_concurrent=1
- ✅ Controller mode (bidder-only) - never executes work
- ✅ Worker mode (execute-only) - launched with `--execute-claim <claim_id>`
- ✅ WorkerManager for orchestrator with Docker client integration
- ✅ Worker lifecycle: LaunchWorker() → monitorWorker() → cleanupWorker()
- ✅ Concurrency limit enforcement with stateless pause mechanism
- ✅ Failure artefact creation on worker exit code ≠ 0
- ✅ Docker socket mounting: /var/run/docker.sock:/var/run/docker.sock
- ✅ HOLT_MODE environment variable for controller identification
- ✅ Full backward compatibility (traditional agents unaffected)
- ✅ Comprehensive E2E tests (basic flow, concurrency, backward compat)

**Key Design Decisions**:
- Explicit `mode: "controller"` in holt.yml for clarity
- Command-line claim delivery: `pup --execute-claim <claim_id>`
- Orchestrator owns worker lifecycle (centralized control)
- No automatic retries (M3.4 scope limit)
- Stateless grant pausing (persistent queue deferred to M3.5)

**Documentation**: See `design/features/phase-3-coordination/M3.4-controller-worker-pattern.md`



### **M3.5: Orchestrator Restart Resilience** ✅ **COMPLETE**

**Implementation Summary**: Full orchestrator restart resilience with comprehensive state recovery.

**Delivered Features**:
- ✅ Phase state persisted to Redis (Claim.PhaseState field)
- ✅ Persistent grant queue using Redis ZSET (FIFO ordering by timestamp)
- ✅ RecoverState() called on orchestrator startup
- ✅ Scan Redis for active claims (pending_review, pending_parallel, pending_exclusive, pending_assignment)
- ✅ Reconstruct in-memory PhaseState from persisted data
- ✅ Validate granted agents still exist in config
- ✅ Re-trigger grants for claims missing artefacts (both workers and traditional agents)
- ✅ Orphaned worker cleanup (CleanupOrphanedWorkers)
- ✅ Grant queue recovery with automatic resumption
- ✅ Stale lock detection in `holt up` (30s threshold)
- ✅ Graceful failure handling (terminate claims with clear reasons)

**Key Design Decisions**:
- Claim schema extended with PhaseState, GrantQueue, and grant tracking fields
- ZSET-based grant queue for FIFO ordering
- Clean upgrade model (no M3.4→M3.5 migration logic required)
- Re-grant to same agents (not re-consensus) for determinism
- Always re-trigger if artefact missing (no time-based heuristics)

**Success Criteria**: All met
- ✓ Orchestrator restart preserves phase state
- ✓ Claims continue progressing after restart
- ✓ No manual intervention required
- ✓ Persistent grant queue survives restarts
- ✓ Orphaned workers cleaned up automatically
- ✓ Stale locks detected and handled

**Documentation**: See `design/features/phase-3-coordination/M3.5-orchestrator-restart-resilience.md`

### **M3.6: Runtime Failure Detection & Timeouts** 🔜 **HIGH PRIORITY**

**Requirement**: The orchestrator needs a mechanism to detect when a granted agent has crashed, hung, or is taking too long to produce an artefact.

**Current Limitation**: Orchestrator waits indefinitely; crashed agents leave claims stuck.

**Proposed Behavior**:
- Configurable timeout per phase (e.g., review: 5 minutes, parallel: 10 minutes, exclusive: 30 minutes)
- When timeout is exceeded, orchestrator terminates the claim
- Creates a Failure artefact with timeout details
- Logs clear timeout event for operational monitoring

**Implementation Considerations**:
- Add timeout configuration to holt.yml (per-agent or per-phase)
- Track grant time in phase state
- Periodic check for timed-out grants
- Graceful handling of agents that complete after timeout

**Success Criteria**:
- Claims do not remain stuck indefinitely
- Timeouts create Failure artefacts with clear error messages
- Configurable timeout values per agent or phase


