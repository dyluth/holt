# **The Agent Pup: Design & Specification**

**Purpose**: Agent pup component architecture, operational modes, and tool contracts  
**Scope**: Component-specific - read when implementing agent functionality  
**Read when**: Implementing agent logic, container execution, bidding systems

## **1. Core Purpose**

The Agent Pup is the brain and nervous system of every agent in Holt. It is a lightweight, standalone binary (`pup`) that runs as the entrypoint inside an agent's container.

Its fundamental purpose is to bridge the gap between the stateless, tool-equipped container and the stateful, shared blackboard. It manages the agent's entire lifecycle, from bidding on work to executing tasks and publishing results. For LLM-driven agents, it is responsible for all communication with the model, acting as the intelligent layer that translates blackboard state into high-context prompts.

## **2. Key Responsibilities**

The `pup` binary is responsible for:

*   **Initialisation:** Reading configuration from environment variables to identify its role and connect to the correct Redis instance.
*   **Blackboard Communication:** Acting as the sole client to the blackboard for all reads and writes.
*   **Claim Lifecycle Management:** Watching for new claims, evaluating them, and submitting bids.
*   **Work Acquisition:** Monitoring for granted claims to know when to begin execution.
*   **Context Assembly:** Traversing the artefact dependency graph on the blackboard to build a rich, historical context for any given task.
*   **Tool Execution:** Executing the tools available within its container based on instructions from the LLM or a deterministic workflow.
*   **Artefact Creation:** Formatting the results of its work into a new, valid Artefact and posting it to the blackboard.
*   **Error Handling:** Reporting execution failures by creating `Failure` artefacts.

## **3. The `pup` Lifecycle & Operational Modes**

The `pup` is a concurrent application that runs two primary processes in parallel as goroutines: a **Claim Watcher** and a **Work Executor**. This ensures the agent remains responsive to new work opportunities even while executing a long-running task.

However, to support different scaling models, the `pup` can activate these goroutines selectively based on its configuration. This results in three distinct operational modes.

### **3.1. Standard Mode (Default)**

*   **Activation**: Default mode for any agent that is **not** configured with `mode: controller` in `holt.yml`.
*   **Active Components**: Runs both the **Claim Watcher** and **Work Executor** loops concurrently.
*   **Behavior**: The `pup` is fully autonomous. It watches for claims, bids on them, and executes any work it is granted.
*   **Use Case**: Simple, single-replica agents that perform all functions in one container.

### **3.2. Controller Mode (`mode: controller`)**

*   **Activation**: When an agent is defined with `mode: controller` in `holt.yml`.
*   **Active Components**: Runs **only the Claim Watcher loop**.
*   **Behavior**: The `pup` acts as the dedicated brain for a fleet of workers. It evaluates all new claims and submits bids but **never** executes work itself. When a claim is granted, the Orchestrator is responsible for launching an ephemeral worker `pup`.
*   **Use Case**: The central bidding and coordination component for a scalable, multi-worker agent.

### **3.3. Worker Mode (Ephemeral)**

*   **Activation**: Launched by the Orchestrator with the `--execute-claim <claim_id>` command-line flag.
*   **Active Components**: Runs **only the Work Executor loop**.
*   **Behavior**: The `pup` is single-purpose. It immediately executes the specific claim it was assigned, publishes the resulting artefact, and then exits. It does not watch for other claims or perform any bidding.
*   **Use Case**: The ephemeral, scalable execution units for a controller-based agent.

## **4. The Context Assembly Algorithm**

This is the core of the `pup`'s intelligence layer. It makes the agent appear stateful by providing it with a deep historical context for any given task.

**Algorithm:**

1.  **Start Traversal:** Begin with the `source_artefacts` from the Artefact the agent is working on.
2.  **Initialise Data Structures:** Create a queue of artefact IDs to visit and a map to store the final, de-duplicated context.
3.  **Walk the Graph (Breadth-First Search):**
    *   For each ID in the queue:
        *   Fetch the full Artefact from the blackboard.
        *   Using its `logical_id`, query the blackboard's thread tracking structure to find the artefact in that logical thread with the highest `version` number. This **shortcut** ensures the context always contains the most recent version of any piece of work.
        *   Add this latest-version artefact to the final context map (keyed by `logical_id` to prevent duplicates).
        *   Add all of its `source_artefacts` to the queue for the next level of traversal.
4.  **Assemble Prompt:** The collected artefacts in the context map are serialized into the `context_chain` field of the JSON object passed to the agent's tool script.

## **5. Dynamic Bidding with `bid_script` (M3.8)**

To create intelligent and dynamic agents, the `pup` can delegate its bidding logic to an external script.

*   **Configuration**: An agent's `holt.yml` entry can contain a `bid_script` property, which is a shell command to be executed.
*   **Execution Flow**:
    1.  The Claim Watcher receives a new claim.
    2.  If `bid_script` is configured, the `pup` executes it.
    3.  The `pup` passes the full `Claim` object as a JSON string to the script's `stdin`.
    4.  The script performs its logic (e.g., calls an LLM, checks a file, evaluates the claim payload) and prints its desired bid (`ignore`, `review`, `claim`, or `exclusive`) to `stdout`.
    5.  The `pup` reads the bid from `stdout` and submits it to the Orchestrator.
*   **Default Behavior**: If `bid_script` is not defined, the `pup` defaults to the legacy behavior of bidding `exclusive` on all claims it sees.

## **6. Configuration and Initialisation**

The Orchestrator is responsible for providing the `pup` with its identity and environment via standard container mechanisms.

**Environment Variables:**

*   **`HOLT_INSTANCE_NAME`**: The name of the holt instance (e.g., `my-first-holt`).
*   **`HOLT_AGENT_ROLE`**: The agent's unique role from `holt.yml` (e.g., `go-coder-agent`). This is its identity for bidding.
*   **`REDIS_URL`**: The connection string for the blackboard.
*   **Additional environment variables**: As defined in the agent's `holt.yml` `environment` section.

**Mounted Files and Workspace:**

*   The agent's container has the project's working directory mounted as the default workspace. The mount mode (`ro` or `rw`) is specified in the agent's `workspace` configuration.

## **7. Tool Execution Contract**

The `pup`'s interaction with agent-specific tools is defined by a simple, robust contract. The `pup` binary is the container's entrypoint and acts as a generic runner for a **single, mandatory command** defined in the agent's `holt.yml` configuration.

### **Input to the Script (passed via `stdin`)**

The `pup` passes a single JSON object to the script's `stdin`. This object contains the full historical context for the task.

**Complete Input Schema:**

```json
{
  "claim_type": "exclusive",
  "target_artefact": { ... },
  "context_chain": [ ... ],
  "additional_context": [
    {
      "id": "review-artefact-uuid-456",
      "structural_type": "Review",
      "payload": {"feedback": "The function is missing error handling."},
      "produced_by_role": "code-reviewer",
      ...
    }
  ]
}
```

**Field Descriptions:**

*   **`claim_type`**: A string indicating the type of work granted: `review`, `claim`, or `exclusive`.
*   **`target_artefact`**: The full JSON object of the artefact that the claim was created for.
*   **`context_chain`**: A JSON array of the latest version of every artefact in the historical dependency chain.
*   **`additional_context`**: An array of full Artefact objects. When a claim is part of a rework cycle (triggered by the **Automated Feedback Loop**), this array contains the `Review` artefacts that rejected the previous work, providing the agent with the specific feedback it needs to address.

### **Output from the Script (read from `stdout`)**

The `pup` expects the script to produce a single JSON object on `stdout`. This object defines the Artefact to be created.

**Example Output (for creating a `Question`):**

```json
{
  "structural_type": "Question",
  "type": "ClarificationNeeded",
  "payload": "Is it acceptable to use a third-party library for this task?",
  "summary": "Seeking clarification on implementation constraints."
}
```

This contract gives the agent's script full control over the workflow, allowing it to produce any type of `Artefact` as a result of its work.

## **8. Health Checks and Fault Tolerance**

*   **Signal Handling**: The `pup` handles `SIGINT` and `SIGTERM` gracefully, allowing in-progress work to finish before exiting.
*   **Health Checks**: The `pup` exposes a `GET /healthz` endpoint that returns `200 OK` if connected to Redis and `503 Service Unavailable` otherwise.
*   **Network Resilience**: All external network calls (to Redis or LLM APIs) implement a robust exponential backoff retry policy.
*   **I/O Handling**: If the tool script exits with a non-zero status code or produces invalid JSON, the `pup` creates a `Failure` artefact containing the script's `stdout` and `stderr` to aid debugging.