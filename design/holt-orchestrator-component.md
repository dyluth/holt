# **The Holt Orchestrator Component: Design & Specification**

**Purpose**: Orchestrator component logic, algorithms, and implementation details  
**Scope**: Component-specific - read when implementing orchestrator features  
**Read when**: Implementing orchestrator logic, claim management, event handling

## **1. Core purpose**

The Orchestrator is the central coordination engine of Holt. It is a lightweight, event-driven component that serves as a non-intelligent traffic cop, managing the lifecycle of Claims and coordinating agent work without making domain-specific decisions.

Its fundamental purpose is to:
- Watch for new Artefacts on the blackboard.
- Create corresponding Claims based on those Artefacts.
- Coordinate a consensus-based bidding process with all registered agents.
- Manage the phased execution of granted Claims (Review, Parallel, Exclusive).
- Handle automated feedback loops for iterative work.
- Launch and manage ephemeral worker containers for scalable agents.
- Recover from crashes or restarts without losing workflow state.

## **2. System Bootstrapping & State Recovery**

The Orchestrator is a stateful component that persists its critical state to Redis, allowing it to resume operations after a restart.

### **2.1. Initial Workflow Trigger**

A workflow begins when an external actor (typically the `holt forage` CLI command) creates an initial `GoalDefined` artefact on the blackboard. The Orchestrator, which subscribes to all artefact creation events, detects this new artefact and begins the orchestration process by creating the first claim.

### **2.2. Startup & Restart Resilience (M3.5)**

Upon starting, the orchestrator performs a recovery sequence to ensure seamless continuation of in-flight work:

1.  **Stale Lock Detection**: It checks for a stale instance lock in Redis. If a lock from a previous, crashed orchestrator is found (older than 30 seconds), it takes over the lock.
2.  **Orphan Worker Cleanup**: It queries the Docker API for any worker containers from its previous run (identifiable by a unique `run_id` label) and removes them.
3.  **Active Claim Recovery**: It scans Redis for all claims in a pending state (`pending_review`, `pending_parallel`, `pending_exclusive`, `pending_assignment`).
4.  **State Reconstruction**: For each active claim, it reconstructs its in-memory tracking object from the state persisted in the claim's Redis hash.
5.  **Grant Re-triggering**: If a claim was granted but the agent never produced an output (because the orchestrator crashed), the grant is re-triggered. For controller-worker agents, a new worker is launched; for traditional agents, the grant notification is re-published.
6.  **Grant Queue Recovery**: It reloads the persistent grant queue from the Redis ZSET and will resume granting claims as worker slots become available.

This process ensures that orchestrator restarts are transparent to the overall workflow.

## **3. Core Orchestration Logic**

The orchestrator's main loop is event-driven, reacting to the creation of new artefacts on the blackboard.

### **3.1. Claim Creation**

1.  **Artefact Monitoring**: The orchestrator subscribes to the `artefact_events` channel.
2.  **Event Processing**: Upon receiving a notification for a new artefact, it fetches the full artefact data.
3.  **Claim Filtering**: It **does not** create claims for `Terminal`, `Failure`, or `Review` structural types, as these represent the *end* of a work step, not the beginning.
4.  **Idempotency**: It checks if a claim for the given artefact ID already exists. If so, it ignores the duplicate event.
5.  **Claim Creation**: For a new, valid `Standard` or `Answer` artefact, it creates a new Claim object with status `pending_consensus` and persists it to the blackboard.
6.  **Agent Notification**: It publishes the new claim ID to the `claim_events` channel to notify all agents of the new work opportunity.

### **3.2. Full Consensus Bidding (M3.1)**

After creating a claim, the orchestrator enters a consensus loop to gather bids from all agents defined in `holt.yml`.

1.  **Agent Registry**: At startup, the orchestrator loads the list of all agent roles from `holt.yml`.
2.  **Bid Collection**: It waits until a bid has been received from every registered agent. It polls Redis every 100ms to check the bid status.
3.  **Deterministic Tie-Breaking**: If multiple agents bid `exclusive`, the orchestrator deterministically chooses the winner by sorting the agent roles alphabetically and selecting the first one.
4.  **No Timeouts (V1 Design)**: The V1 consensus model waits indefinitely for all bids. A non-responsive agent will halt the workflow for that claim, which must be resolved manually. This is a deliberate choice to prioritize determinism in early phases.

### **3.3. Phased Claim Execution (M3.2)**

Once consensus is reached, the orchestrator transitions the claim through a strict three-phase lifecycle.

1.  **Phase Determination**: Based on the collected bids, the orchestrator determines the starting phase. If there are `review` bids, it starts in `pending_review`. If not, it skips to `pending_parallel` (if there are `claim` bids) or directly to `pending_exclusive`.

2.  **Review Phase (`pending_review`)**:
    *   **Grant**: All agents that bid `review` are granted access. The orchestrator updates the claim's `granted_review_agents` field and persists it.
    *   **Execution**: The orchestrator waits for all granted reviewers to produce `Review` artefacts.
    *   **Completion**: Once all reviews are received, it inspects their payloads. Any payload other than an empty JSON object (`{}`) or empty array (`[]`) is considered a rejection.
    *   **Outcome**:
        *   **Rejection (Single Veto)**: If even one review contains feedback, the **Automated Feedback Loop (M3.3)** is triggered (see section 3.4).
        *   **Approval**: If all reviews are approvals, the orchestrator transitions the claim to the next phase.

3.  **Parallel Phase (`pending_parallel`)**:
    *   **Grant**: All agents that bid `claim` are granted access.
    *   **Execution**: The orchestrator waits for all granted parallel agents to produce their output artefacts.
    *   **Completion**: Once all artefacts are received, it transitions the claim to the exclusive phase.

4.  **Exclusive Phase (`pending_exclusive`)**:
    *   **Grant**: The single, deterministically chosen winner of the `exclusive` bids is granted the claim.
    *   **Execution**: The orchestrator waits for the agent to produce its output artefact.
    *   **Completion**: When the artefact is received, the claim is marked as `complete`.

### **3.4. Automated Feedback Loop (M3.3)**

Instead of simply terminating a claim upon review rejection, the orchestrator initiates an automated rework cycle.

1.  **Rejection Detection**: The orchestrator detects one or more `Review` artefacts with feedback.
2.  **Iteration Limit Check**: It checks the `version` of the rejected artefact against the `orchestrator.max_review_iterations` limit from `holt.yml`. If the limit is exceeded, it creates a `Failure` artefact and terminates the workflow.
3.  **Feedback Claim Creation**: If the limit is not reached, it creates a **new claim** with status `pending_assignment`.
4.  **Direct Assignment**: This new claim is directly assigned to the role that produced the original, rejected artefact, bypassing the bidding process.
5.  **Context Injection**: The claim includes the IDs of all feedback-containing `Review` artefacts in its `additional_context_ids` field, ensuring the original agent receives the feedback.

### **3.5. Controller-Worker Scaling (M3.4)**

The orchestrator has special logic for handling agents configured in `mode: controller`.

1.  **Controller Detection**: When granting a claim, the orchestrator checks if the winning agent role is a controller.
2.  **Concurrency Check**: It checks its in-memory tracker to see how many workers are currently running for that role. If the number of active workers is at the `max_concurrent` limit, the grant is **paused**.
3.  **Persistent Queue (M3.5)**: If a grant is paused, the claim ID is added to a persistent, role-specific FIFO queue in Redis (a ZSET scored by timestamp).
4.  **Worker Launch**: If a slot is available, the orchestrator launches a new, ephemeral worker container via the Docker API. The worker is started with the command `pup --execute-claim <claim_id>`.
5.  **Lifecycle Monitoring**: The orchestrator monitors the worker container. When the container exits, it inspects the exit code.
6.  **Cleanup & Failure Handling**: If the exit code is 0, the process is complete. If non-zero, the orchestrator creates a `Failure` artefact containing the worker's logs and terminates the original claim. In both cases, the orchestrator removes the exited worker container.

## **4. State Persistence & Resilience (M3.5)**

*   **Continuous Persistence**: All significant state changes (phase transitions, grants, queueing) are immediately written to the corresponding `Claim` hash in Redis. The in-memory state is only a cache.
*   **Heartbeat Lock**: The orchestrator maintains a lock key in Redis (`holt:{instance}:lock`) with a 60-second TTL, which it refreshes every 10 seconds. This acts as a heartbeat.
*   **Recovery on Startup**: As described in Section 2.2, the orchestrator performs a full recovery sequence on startup, ensuring no work is lost and no resources are orphaned.

## **5. Health Checks and Monitoring**

*   **Health Endpoint**: The orchestrator exposes a `GET /healthz` endpoint. It returns `200 OK` if it can connect to Redis and `503 Service Unavailable` otherwise.
*   **Structured Logging**: All significant events (claim creation, phase transitions, grants, worker launches, errors) are logged as structured JSON to stdout, enabling integration with log aggregation systems.