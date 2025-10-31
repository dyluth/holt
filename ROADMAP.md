# Holt Project Roadmap

**Purpose**: To provide a clear overview of the project's phased delivery plan, key objectives, and future direction. This document is the single source of truth for the Holt roadmap.

---

## The Phased Approach

Holt is being developed through a series of well-defined phases, each delivering a significant leap in capabilities. This approach ensures that the platform is built on a solid foundation, with each new feature set expanding upon a stable core.

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

---

## Future Enhancements

Beyond the core six-phase roadmap, a number of long-term, enterprise-focused features are being considered. These are captured in our living document for future enhancements.

For a detailed look at these ideas, see **[design/future-enhancements.md](./design/future-enhancements.md)**.

---

## How to Contribute

We welcome contributions to all areas of the project, from the core components to the development of new example agents. The best way to get started is to review our **[CONTRIBUTING.md](./CONTRIBUTING.md)** guide, which outlines our unique AI-assisted development process.

If you are interested in contributing to a specific future feature, please open an issue on our GitHub repository to start a discussion with the core team.