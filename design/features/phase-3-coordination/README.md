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

### **M3.3+: Future Milestones** 🔜 **PENDING DESIGN**
See "Future Milestones" section below for planned enhancements.

## **Phase Success Criteria**

- ✅ Complex workflow with review feedback loop
- ✅ Multiple agents working in parallel
- ⚠️ Basic error handling (no timeouts or auto-retry yet)
- 🔜 Controller-worker pattern for scaling (deferred)

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

3. **Controller-Worker Pattern** (Future)
   - 🔜 Scalable agent architecture (replicas > 1)
   - 🔜 Bidder-only and execute-only modes
   - 🔜 Ephemeral container management
   - 🔜 Race condition elimination

## **Implementation Constraints**

- No human interaction yet (Phase 4 dependency)
- Focus on agent coordination and workflow phases
- Enhanced error handling and failure recovery
- Production-level reliability required

## **Testing Requirements**

- Multi-agent coordination testing
- Phase transition validation
- Failure scenario testing
- Controller-worker pattern verification
- Load testing with multiple agents

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
- M3.1 configs with duplicate roles will fail validation at `sett up` time
- Clear error message: `duplicate agent role 'X' found (agents 'A' and 'B'): all agents must have unique roles in Phase 3`

**Rationale**: Enables reliable artefact attribution using `produced_by_role` field for phase completion tracking.

**Future Resolution**: Phase 4+ could use `produced_by_agent` field to support duplicate roles.

### **3. No Automated Feedback Loop**

**Limitation**: Review rejection terminates the claim; no automatic retry with feedback incorporated.

**Impact**:
- When a reviewer provides feedback, the claim status becomes `terminated`
- Workflow stops; no automatic re-assignment to the original agent with feedback
- User must manually restart the workflow with the feedback incorporated

**Future Resolution**: M3.3+ should automatically create new claims with feedback context.

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

### **M3.3: Orchestrator Restart Resilience** 🔜 **HIGH PRIORITY**

**Requirement**: The orchestrator must be able to recover the state of in-progress claims if it restarts unexpectedly.

**Current Limitation**: Phase state is in-memory only; orchestrator restart loses all tracking data.

**Proposed Behavior**:
- On startup, the orchestrator scans Redis for claims in active states (pending_review, pending_parallel, pending_exclusive)
- Reconstructs in-memory phase state by querying granted agents and received artefacts
- Resumes monitoring for phase completion
- Claims are no longer stuck after orchestrator restart

**Implementation Considerations**:
- Need to persist phase state to Redis (additional keys or claim fields)
- Need startup recovery logic to rebuild phase state map
- Need to handle edge cases (partial artefacts received, status inconsistencies)

**Success Criteria**:
- Orchestrator restart does not lose phase tracking
- Claims continue progressing through phases after restart
- No manual intervention required for stuck claims

### **M3.4: Runtime Failure Detection & Timeouts** 🔜 **HIGH PRIORITY**

**Requirement**: The orchestrator needs a mechanism to detect when a granted agent has crashed, hung, or is taking too long to produce an artefact.

**Current Limitation**: Orchestrator waits indefinitely; crashed agents leave claims stuck.

**Proposed Behavior**:
- Configurable timeout per phase (e.g., review: 5 minutes, parallel: 10 minutes, exclusive: 30 minutes)
- When timeout is exceeded, orchestrator terminates the claim
- Creates a Failure artefact with timeout details
- Logs clear timeout event for operational monitoring

**Implementation Considerations**:
- Add timeout configuration to sett.yml (per-agent or per-phase)
- Track grant time in phase state
- Periodic check for timed-out grants
- Graceful handling of agents that complete after timeout

**Success Criteria**:
- Claims do not remain stuck indefinitely
- Timeouts create Failure artefacts with clear error messages
- Configurable timeout values per agent or phase

### **M3.5: Automated Feedback Loop** 🔜 **MEDIUM PRIORITY**

**Requirement**: The system needs a mechanism to automatically re-assign work to an agent based on Review feedback from another agent.

**Current Limitation**: Review rejection terminates claim; user must manually restart workflow.

**Proposed Behavior**:
- When a claim is terminated due to negative review feedback, the orchestrator automatically creates a new claim
- New claim targets the original artefact's producer agent
- New claim's context includes both the original artefact and the Review feedback artefact
- Agent can read feedback and iterate on the work

**Implementation Considerations**:
- Detect review rejection and extract feedback artefact
- Create new claim with source_artefacts = [original_artefact, review_artefact]
- Limit iteration depth to prevent infinite loops (max 3 iterations?)
- Log feedback loop creation for audit trail

**Success Criteria**:
- Review feedback automatically triggers new claim
- Original agent receives feedback in context
- Workflow can iterate until all reviewers approve
- Iteration depth limits prevent infinite loops

### **M3.6: Controller-Worker Pattern for Scaling** 🔜 **LOW PRIORITY**

**Requirement**: Support scalable agent architecture with replicas > 1.

**Current Status**: Agent cub has bidder-only and execute-only modes, but not fully integrated.

**Proposed Behavior**:
- One persistent "controller" agent per role (bidder-only mode)
- Controller submits bids on behalf of the role
- When granted, orchestrator launches ephemeral "worker" agents (execute-only mode)
- Workers execute in parallel, exit on completion
- Eliminates race conditions in bidding while enabling horizontal scaling

**Implementation Considerations**:
- Orchestrator needs to launch worker containers dynamically
- Worker lifecycle management (create, execute, destroy)
- Worker container naming and tracking
- Resource limits for parallel workers

**Success Criteria**:
- Agents can scale horizontally with replicas > 1
- No race conditions in bidding
- Workers execute in parallel efficiently
- Clean worker cleanup after execution