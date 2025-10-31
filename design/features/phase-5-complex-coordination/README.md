# **Phase 5: Complex Workflow Coordination**

**Goal**: Enable the orchestration of complex, non-linear workflows involving multi-dependency synchronization ("fan-in") and conditional pathing, moving Holt from a phased execution model to a full Directed Acyclic Graph (DAG) coordination platform.

## **Phase Success Criteria**

- A "synchronizer" or "aggregator" agent can be built that waits for several distinct artefacts from different workflow branches to be completed before it begins its own work.
- The system can support conditional execution paths, where the creation of one type of artefact (e.g., `HighSeverityBugFound`) can trigger a completely different set of agents than another artefact (e.g., `TestsPassed`).
- Complex, real-world CI/CD pipelines, including build, multi-platform testing, security scanning, and conditional deployment, can be fully modeled and executed within Holt.
- The `holt watch` output and `holt hoard` audit trail can be used to clearly visualize and debug these complex, branching, and merging workflows.

## **Key Features for This Phase**

1.  **Fan-In / Synchronization Pattern:**
    *   Formalize the design pattern where an agent's bidding logic inspects the blackboard to verify that multiple, distinct prerequisite artefacts exist before submitting a `claim` or `exclusive` bid.
    *   This allows an agent to act as a synchronization point, waiting for several parallel branches of a workflow to complete.

2.  **Conditional Pathing:**
    *   Leverage dynamic bidding to a greater extent, allowing agents to create highly conditional workflows. For example, an agent could bid `exclusive` on a `TestResult` artefact only if its payload contains `"status": "failed"`, thereby creating a dedicated "failure recovery" branch in the workflow.

3.  **Advanced Context-Aware Bidding:**
    *   Agent bidding scripts will evolve to become more sophisticated, using the `context_chain` and direct blackboard queries to make decisions based on the full history and state of the workflow, not just the target artefact.

## **Implementation Constraints**

- The primary mechanism for this phase will be the implementation of more intelligent agent bidding logic, rather than significant changes to the orchestrator.
- The orchestrator must remain a stateless, non-intelligent arbiter. All workflow branching and synchronization logic must reside within the agents' bidding strategies.
- Performance of blackboard queries during bidding will become a key consideration.

## **Dependencies**

- This phase builds upon all previous phases, requiring a stable multi-agent coordination model (Phase 3) and robust human-in-the-loop capabilities (Phase 4).

---

## **M5.1 Implementation Detail: Declarative Fan-In Synchronization**

*This section contains the detailed design for the Fan-In Synchronization feature, to be broken into formal milestones upon implementation of Phase 5.*

### **Problem Statement**

Holt's phased execution model excels at linear workflows and parallel "fan-out" operations (e.g., multiple reviewers acting on one artefact). However, it lacks a formal, robust mechanism for the reverse: **"fan-in" synchronization**. There is no easy way to define an agent that should only run *after* several different, parallel workflow branches have all completed.

A naive implementation would require the synchronizing agent to contain complex, brittle, and race-condition-prone logic in its bidding script to query the blackboard and correlate disparate results. This is unsafe, as it could erroneously merge results from different workflow forks, and it violates the principle of keeping agent logic simple.

### **The Solution: Declarative Synchronization**

To solve this, "fan-in" will be a first-class feature of Holt, implemented in the Pup and declared in `holt.yml`.

#### **New `holt.yml` Configuration**

A new, optional `synchronize` block will be added to the agent definition. An agent with this block is designated as a "Synchronizer Agent."

```yaml
agents:
  Deployer:
    image: "holt-deployer-demo:latest"
    command: ["/app/run.sh"]
    # The "synchronize" block replaces bid_script and bidding_strategy
    synchronize:
      on:
        # The agent will look for this common ancestor type.
        ancestor_type: "CodeCommit"
        # And will wait until ALL of these descendant types exist
        # as direct children of that single ancestor.
        require_descendants:
          - "TestResult"
          - "LintResult"
          - "Documentation"
      # The bid to place on the final trigger artefact once all conditions are met.
      bid: "exclusive"
```

#### **The Pup's New Synchronization Logic**

When an agent pup starts, it will detect the `synchronize` block in its configuration and enter a new bidding mode. For every incoming claim, the `claimWatcher` will execute this logic:

1.  **Identify Potential Trigger:** Check if the claim's target artefact has a `type` that is listed in the `require_descendants` array. If not, `ignore` the claim immediately.

2.  **Find Common Ancestor:** If the artefact is a potential trigger, traverse **up** the artefact graph via its `source_artefacts`. Find the first ancestor whose `type` matches the `ancestor_type` from the configuration (e.g., `CodeCommit`). If no such ancestor is found, `ignore` the claim.

3.  **Verify All Dependencies (The Fan-In Check):** Once the common ancestor is found, the pup will perform a **full descendant traversal** starting from that ancestor to find all artefacts that have it in their provenance chain. It will then check if this complete set of descendants contains an artefact for **every single type** listed in the `require_descendants` array.

4.  **Make Bid Decision:**
    *   If all required descendant artefacts are present, the condition is met. The pup will submit the configured `bid` (e.g., `exclusive`) on the current claim (which was for the final artefact that completed the set).
    *   If any descendant artefact is still missing, the pup will submit `ignore`.

This logic ensures that the bid is only placed when the complete set of required inputs, all stemming from a single, common ancestor, is available.

#### **Agent Execution and Context**

When the Synchronizer agent (e.g., `Deployer`) is finally granted the claim, the pup will assemble a special, rich `context_chain` to pass to its `run.sh` script. This context will include:

*   The common `ancestor_artefact` (e.g., the `CodeCommit`).
*   The full list of all the `descendant_artefacts` that satisfied the condition (e.g., the `TestResult`, `LintResult`, and `Documentation` artefacts).

This provides the agent with all the necessary inputs to perform its aggregation or deployment task.