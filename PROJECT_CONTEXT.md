# **Holt Project Context: Purpose, Philosophy & Vision**

**Purpose**: Essential project overview and architectural foundation  
**Scope**: Essential - required reading for all development tasks  
**Estimated tokens**: ~1,500 tokens  
**Read when**: Starting any Holt development work, need project context

## **What is Holt?**

Holt is a **container-native AI agent orchestrator** designed to manage a clan of specialized, tool-equipped AI agents for automating complex software engineering tasks. It is **not** an LLM-chaining library—it is an orchestration engine for real-world toolchains that software professionals use every day.

## **Core Philosophy & Guiding Principles**

### **Pragmatism over novelty (YAGNI)**
We prioritise using existing, battle-hardened tools rather than building our own. This principle applies at all levels:
* Core components: We use Docker for containers and Redis for state because they are excellent. Holt's core is an orchestrator, not a database or container runtime.
* Internal logic: We prefer wrapping an existing, stable tool over reimplementing its functionality. For example, the holt logs command is a thin, user-friendly wrapper around docker logs, not a custom logging pipeline.

### **Zero-configuration, progressively enhanced**
The experience must be seamless out of the box. A developer should be able to get a basic holt running with a single command. Smart defaults cover 90% of use cases, while advanced features are available for those who need them.

### **Small, single-purpose components**
Each element—the orchestrator, the CLI, the agent pup—has a clear, well-defined job and does that one thing excellently. Complexity is managed by composing simple parts.

### **Auditability as a core feature**
Artefacts are treated as write-once records by the Holt platform. Every decision and agent interaction is recorded on the blackboard, providing a complete, auditable history of the workflow. This makes Holt particularly valuable for regulated industries, compliance workflows, and any environment where AI transparency and accountability are business-critical or legally required.

### **ARM64-first design**
Development and deployment are optimized for ARM64, with AMD64 as a fully supported, compatible target.

### **Principle of least privilege**
Agents run in non-root containers with the minimal set of privileges required to perform their function.

## What Makes Holt Different: Enterprise-Ready by Design

Holt is not just a pattern; it is a complete system that provides enterprise-grade features out-of-the-box. Its simple, first-principles architecture is precisely what enables this powerful, built-in functionality.

### Complete & Chronological Auditability
Holt provides a complete, unchangeable audit trail for the entire lifecycle of a workflow. This is not an add-on; it is a core property of the system.

- **Append-Only Ledger:** Every action, decision, and artefact is recorded as an immutable entry on the central blackboard.
- **Queryable History:** The `holt hoard` command provides an out-of-the-box tool to inspect and query this audit trail, allowing developers and compliance officers to trace any workflow from start to finish.
- **Git-Native Versioning:** For code-related tasks, every change is tied to a Git commit hash, integrating the audit trail with an industry-standard version control system.

### Powerful, Built-in Observability
Holt provides integrated tools for monitoring and debugging your AI agents and workflows in real time.

- **Automated Health Checks:** Every agent runs with a mandatory, built-in health check, allowing the orchestrator to immediately detect and report on agents that have crashed or become unresponsive.
- **Real-time Monitoring:** The `holt watch` command provides a live stream of all system events. With powerful filtering by **time, agent, and artefact type**, you can zero in on the exact information you need.
- **Machine-Readable Output:** The `watch` and `hoard` commands can output events as line-delimited JSON (`jsonl`), allowing you to seamlessly pipe Holt's observability data into external logging and monitoring systems like Splunk, Datadog, or the ELK stack.
- **Targeted Debugging:** The `holt logs <agent-name>` command lets you instantly access the logs for any specific agent, streamlining the debugging process.

### Flexible, Container-Native Deployment
Holt's architecture is designed for a seamless transition from local development to production deployment on any standard container platform.

- **Local Development:** The `holt up` command provides a simple, `docker-compose`-like experience for running a complete Holt instance on your local machine.
- **Production-Ready Architecture:** Because every agent and Holt component is a container, a `holt.yml` configuration serves as a blueprint for production. This stack can be deployed to any major orchestrator, including **Kubernetes, Amazon ECS, or Google Cloud Run**.
- **Future: First-Class Kubernetes Native:** The Phase 6 roadmap will make Holt a first-class citizen in Kubernetes. This includes a **Holt Operator** for managing the entire lifecycle, native integration with Kubernetes health and logging, and built-in **Prometheus metrics endpoints** for seamless integration with your existing monitoring infrastructure.

## Key Architectural Concepts

### The Blackboard
A Redis-based shared state system where all components interact via well-defined data structures. It serves as a lightweight ledger storing metadata and pointers, not large data blobs. **Critically for compliance**: every interaction is logged with timestamps, creating a chronological audit trail that meets regulatory requirements for AI transparency and accountability.

### Artefacts
Append-only data objects representing work products. They have:
- **structural_type**: Role in orchestration (Standard, Review, Question, Answer, Failure, Terminal)
- **type**: User-defined, domain-specific string (e.g., "DesignSpec", "CodeCommit")
- **payload**: Main content (often a git commit hash for code)
- **logical_id**: Groups versions of the same logical artefact

### Claims
Records of the Orchestrator's decisions about specific Artefacts. Claims go through phases:
1. **Review phase**: Parallel review by multiple agents
2. **Parallel phase**: Concurrent work by multiple agents
3. **Exclusive phase**: Single agent gets exclusive access

### The Agent Pup
A lightweight binary that runs as the entrypoint in every agent container. It:
- Watches for claims and bids on them
- Assembles historical context from the blackboard
- Executes the agent's specific tool via a command script
- Posts results back to the blackboard
- Operates concurrently to remain responsive

### Full Consensus Model (V1)
The orchestrator waits until it receives a bid from every known agent before proceeding with the grant process. This V1 model prioritizes determinism and debuggability over performance, ensuring predictable workflows in early development. Future versions are planned to incorporate timeout or quorum-based mechanisms for greater scalability.

### Agent Scaling (Controller-Worker Pattern)
For agents that need to run multiple instances concurrently (configured with `replicas > 1` in `holt.yml`), Holt uses a **controller-worker pattern**. A single, persistent "controller" agent is responsible for bidding on claims. When a claim is won, the orchestrator launches ephemeral "worker" agents to execute the work in parallel. This avoids race conditions while enabling horizontal scaling.

## Core Workflow

1. **Bootstrap**: User runs `holt forage --goal "Create a REST API"` 
2. **Initial Artefact**: CLI creates a GoalDefined artefact on the blackboard
3. **Claim Creation**: Orchestrator sees the artefact and creates a corresponding claim
4. **Bidding**: All agents evaluate the claim and submit bids ('review', 'claim', 'exclusive', 'ignore')
5. **Phased Execution**: Orchestrator grants claims in review → parallel → exclusive phases
6. **Work Execution**: Agent pups execute their tools and create new artefacts
7. **Iteration**: New artefacts trigger new claims, continuing the workflow
8. **Termination**: Workflow ends when an agent creates a Terminal artefact

## Technology Stack

### Core Technologies
- **Go**: Single module with multiple binaries (orchestrator, CLI, pup)
- **Redis**: Blackboard state storage and Pub/Sub messaging
- **Docker**: Agent containerization and lifecycle management
- **Git**: Version control integration and workspace management

### Agent Technologies
Agents can use any technology that can be containerized:
- LLM APIs (OpenAI, Anthropic, local models)
- Command-line tools (compilers, linters, test runners)
- Infrastructure tools (kubectl, terraform, etc.)

## Project Structure

```
holt/
├── cmd/             # Binaries: holt, orchestrator, pup
├── pkg/             # Shared public packages (blackboard types)
├── internal/        # Private implementation details
├── agents/          # Example agent definitions
├── design/          # Design documents and specifications
│   ├── features/                         # Feature design documents by phase
│   │   ├── phase-1-heartbeat/           # Phase 1: Core infrastructure
│   │   ├── phase-2-single-agent/        # Phase 2: Basic execution
│   │   ├── phase-3-coordination/        # Phase 3: Multi-agent coordination
│   │   └── phase-4-human-loop/          # Phase 4: Human-in-the-loop
│   ├── holt-system-specification.md      # Complete system architecture
│   ├── holt-orchestrator-component.md    # Orchestrator component design
│   ├── agent-pup.md                      # Agent pup component design
│   └── holt-feature-design-template.md   # Systematic development template
└── Makefile
```

## Documentation Architecture

The design documentation follows a clear component-based structure optimized for AI agent comprehension:

* **`holt-system-specification.md`** - Complete system overview, architecture, and shared components (blackboard, CLI, configuration)
* **`holt-orchestrator-component.md`** - Focused specification for the orchestrator component's logic and behavior
* **`agent-pup.md`** - Focused specification for the agent pup component's architecture and execution model
* **`holt-feature-design-template.md`** - Systematic template for designing new features with comprehensive analysis framework

This separation ensures each document has a single, clear purpose and minimal cognitive load while maintaining necessary cross-references for component integration.

## Development Approach: Phased Delivery

Holt is being developed through a series of well-defined phases, each delivering a significant leap in capabilities. The project's status is tracked against this roadmap.

### Phase 1: "Heartbeat" ✅
*Goal: Prove the core blackboard architecture works with basic orchestrator and CLI functionality.*
- **Features:** Redis blackboard with Pub/Sub, CLI for instance management, basic orchestrator claim engine.

### Phase 2: "Single Agent" ✅
*Goal: Enable a single agent to perform a complete, useful task.*
- **Features:** Agent `pup` implementation, claim bidding, Git workspace integration, and context assembly.

### Phase 3: "Coordination" 🚧
*Goal: Orchestrate multiple, specialized agents in a collaborative workflow.*
- **Features:** Multi-stage pipelines (review → parallel → exclusive), controller-worker scaling pattern, consensus bidding, automated feedback loops, and powerful CLI observability features.

### Phase 4: "Human-in-the-Loop" 📋
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

## **Key Design Decisions & Rationale**

### **Why Redis?**
Battle-tested, excellent Pub/Sub support, simple data structures, high performance.

### **Why a Write-Once Architecture?**
Provides complete audit trail and prevents race conditions in concurrent environments.

### **Why container-native?**
Enables orchestration of any tool that can be containerized, not just Python functions.

### **Why Git-centric?**
Provides version control, enables deterministic workspaces, and leverages existing developer workflows.

### **Why event-driven?**
Ensures maximum efficiency—agents are never too busy to evaluate new opportunities.

## **Success Criteria**

A successful Holt implementation should:
1. **Enable zero-configuration startup** - `holt init && holt up` creates a working system
2. **Provide complete auditability** - Every decision and change is traceable, meeting regulatory requirements
3. **Support complex workflows** - Multi-agent coordination with mandatory human oversight points
4. **Be production-ready** - Robust error handling, health checks, monitoring suitable for regulated environments
5. **Scale efficiently** - Handle multiple concurrent agents and workloads while maintaining audit integrity
6. **Ensure compliance readiness** - Audit trails, human controls, and transparency features that satisfy regulatory frameworks

## **Target Users**

### **Software Engineering & DevOps**
- **Software engineers** seeking to automate complex, multi-step development tasks
- **DevOps teams** wanting to orchestrate infrastructure and deployment workflows  
- **Engineering managers** needing auditable, controllable automation
- **AI researchers** requiring a robust platform for multi-agent coordination

### **Regulated Industries & Compliance**
- **Financial services** requiring auditable AI workflows for risk assessment, compliance reporting, and regulatory submissions
- **Healthcare organizations** needing traceable AI-assisted processes for clinical documentation, research protocols, and regulatory compliance
- **Government agencies** seeking controllable AI automation with full audit trails for policy analysis, document processing, and decision support
- **Legal firms** requiring documented AI workflows for contract analysis, due diligence, and regulatory research
- **Manufacturing & aerospace** needing auditable AI processes for quality assurance, safety protocols, and regulatory documentation
- **Energy & utilities** seeking traceable AI workflows for compliance reporting, safety assessments, and environmental monitoring

### **Cross-Industry Applications**
- **Compliance officers** in any industry requiring full audit trails for AI-assisted processes
- **Risk management teams** needing controllable, traceable AI workflows
- **Quality assurance professionals** requiring documented AI processes with human oversight
- **Audit teams** seeking transparent, auditable AI automation systems

## **Vision Statement**

Holt aims to be the **de facto orchestration platform** for AI-powered workflows in **any environment where auditability, control, and compliance are critical**. While initially focused on software engineering, Holt's chronological audit trails and human-in-the-loop design make it uniquely suited for regulated industries struggling to safely adopt AI.

By combining the reliability of containerization with the flexibility of AI agents, Holt enables organizations to automate complex tasks while maintaining **full visibility, control, and regulatory compliance**. This makes it invaluable for:

- **Regulated industries** (finance, healthcare, government) requiring traceable AI decisions
- **Compliance workflows** where every AI action must be documented and auditable
- **Security-sensitive environments** needing controlled AI automation with human oversight
- **Any organization** where AI transparency and accountability are business-critical

## **For Implementation Teams**

### **Development Methodology & Quality Assurance**

Holt uses a **systematic, template-driven feature design process** to ensure quality, consistency, and architectural alignment. This methodology *is* the core of our quality assurance strategy.

Every feature **must** be designed using the standardized template (`design/holt-feature-design-template.md`). This is not optional. The template enforces a comprehensive analysis that ensures every feature:
- **Aligns with Holt's guiding principles** (YAGNI, auditability, etc.).
- **Considers all architectural components** (blackboard, orchestrator, pup, CLI).
- **Is designed for failure first**, with robust handling of errors and edge cases.
- **Maintains backward compatibility** and integration safety.
- **Includes a comprehensive testing plan** (unit, integration, E2E).
- **Preserves the chronological audit trail** at all costs.

For the complete process, see `DEVELOPMENT_PROCESS.md`.

### **Core Implementation Principles**

When implementing features, adhere to these principles:
1. **Contracts First**: Well-defined interfaces between components are crucial.
2. **Start with the Blackboard**: It is the foundation of the entire system.
3. **Build Incrementally**: Follow the phased delivery plan to minimize risk.
4. **Document Extensively**: The system's complexity demands clarity.

This project represents a unique approach to AI agent orchestration that prioritizes practicality, auditability, and real-world engineering needs over academic novelty.