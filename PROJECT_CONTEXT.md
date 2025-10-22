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
Artefacts are immutable. Every decision and agent interaction is recorded on the blackboard, providing a complete, auditable history of the workflow. This makes Holt particularly valuable for regulated industries, compliance workflows, and any environment where AI transparency and accountability are business-critical or legally required.

### **ARM64-first design**
Development and deployment are optimized for ARM64, with AMD64 as a fully supported, compatible target.

### **Principle of least privilege**
Agents run in non-root containers with the minimal set of privileges required to perform their function.

## **What Makes Holt Different**

### **Container-native by design**
Unlike Python-based frameworks, Holt orchestrates agents whose tools are **any command-line tool that can be packaged into a container**. This enables automation of tasks that rely on compilers, CLIs, and infrastructure tools (git, docker, kubectl).

### **Event-driven architecture**
The system uses Redis Pub/Sub for efficient, non-polling communication between components. Agents watch for claims and bid on them, creating a robust, event-driven workflow.

### **Immutable audit trail**
Every artefact is immutable. To handle iteration and feedback, agents create new artefacts that are part of a logical thread, creating a clear historical chain without violating immutability.

### **Human-in-the-loop by design**
The system is explicitly designed for human oversight and intervention, with Question/Answer artefacts and CLI commands for human interaction. This architecture ensures compliance with regulations requiring human review of AI decisions and provides the control mechanisms needed in high-stakes environments.

### **Git-centric workflow**
Holt assumes and requires a clean Git repository. Code artefacts are git commit hashes, and agents are responsible for Git interactions, making the entire workflow version-controlled. The specific branching and commit strategy is detailed in `agent-pup.md`.

## **Key Architectural Concepts**

### **The Blackboard**
A Redis-based shared state system where all components interact via well-defined data structures. It serves as a lightweight ledger storing metadata and pointers, not large data blobs. **Critically for compliance**: every interaction is logged with timestamps, creating an immutable audit trail that meets regulatory requirements for AI transparency and accountability.

### **Artefacts**
Immutable data objects representing work products. They have:
- **structural_type**: Role in orchestration (Standard, Review, Question, Answer, Failure, Terminal)
- **type**: User-defined, domain-specific string (e.g., "DesignSpec", "CodeCommit")
- **payload**: Main content (often a git commit hash for code)
- **logical_id**: Groups versions of the same logical artefact

### **Claims**
Records of the Orchestrator's decisions about specific Artefacts. Claims go through phases:
1. **Review phase**: Parallel review by multiple agents
2. **Parallel phase**: Concurrent work by multiple agents
3. **Exclusive phase**: Single agent gets exclusive access

### **The Agent Pup**
A lightweight binary that runs as the entrypoint in every agent container. It:
- Watches for claims and bids on them
- Assembles historical context from the blackboard
- Executes the agent's specific tool via a command script
- Posts results back to the blackboard
- Operates concurrently to remain responsive

### **Full Consensus Model (V1)**
The orchestrator waits until it receives a bid from every known agent before proceeding with the grant process. This V1 model prioritizes determinism and debuggability over performance, ensuring predictable workflows in early development. Future versions are planned to incorporate timeout or quorum-based mechanisms for greater scalability.

### **Agent Scaling (Controller-Worker Pattern)**
For agents that need to run multiple instances concurrently (configured with `replicas > 1` in `holt.yml`), Holt uses a **controller-worker pattern**. A single, persistent "controller" agent is responsible for bidding on claims. When a claim is won, the orchestrator launches ephemeral "worker" agents to execute the work in parallel. This avoids race conditions while enabling horizontal scaling.

## **Core Workflow**

1. **Bootstrap**: User runs `holt forage --goal "Create a REST API"` 
2. **Initial Artefact**: CLI creates a GoalDefined artefact on the blackboard
3. **Claim Creation**: Orchestrator sees the artefact and creates a corresponding claim
4. **Bidding**: All agents evaluate the claim and submit bids ('review', 'claim', 'exclusive', 'ignore')
5. **Phased Execution**: Orchestrator grants claims in review → parallel → exclusive phases
6. **Work Execution**: Agent pups execute their tools and create new artefacts
7. **Iteration**: New artefacts trigger new claims, continuing the workflow
8. **Termination**: Workflow ends when an agent creates a Terminal artefact

## **Technology Stack**

### **Core Technologies**
- **Go**: Single module with multiple binaries (orchestrator, CLI, pup)
- **Redis**: Blackboard state storage and Pub/Sub messaging
- **Docker**: Agent containerization and lifecycle management
- **Git**: Version control integration and workspace management

### **Agent Technologies**
Agents can use any technology that can be containerized:
- LLM APIs (OpenAI, Anthropic, local models)
- Command-line tools (compilers, linters, test runners)
- Infrastructure tools (kubectl, terraform, etc.)

## **Project Structure**

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

## **Documentation Architecture**

The design documentation follows a clear component-based structure optimized for AI agent comprehension:

* **`holt-system-specification.md`** - Complete system overview, architecture, and shared components (blackboard, CLI, configuration)
* **`holt-orchestrator-component.md`** - Focused specification for the orchestrator component's logic and behavior
* **`agent-pup.md`** - Focused specification for the agent pup component's architecture and execution model
* **`holt-feature-design-template.md`** - Systematic template for designing new features with comprehensive analysis framework

This separation ensures each document has a single, clear purpose and minimal cognitive load while maintaining necessary cross-references for component integration.

## **Development Approach: Phased Delivery**

### **Phase 1: "Heartbeat"** - Core Infrastructure
Prove the blackboard architecture works with basic orchestrator and CLI.

### **Phase 2: "Single Agent"** - Basic Execution  
One agent can claim and execute work with full Git integration.

### **Phase 3: "Coordination"** - Multi-Agent Workflow
Review → Parallel → Exclusive phases working with multiple agent types.

### **Phase 4: "Human-in-the-Loop"** - Production Ready
Question/Answer system with complete operational features.

## **Key Design Decisions & Rationale**

### **Why Redis?**
Battle-tested, excellent Pub/Sub support, simple data structures, high performance.

### **Why immutable artefacts?**
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

Holt aims to be the **de facto orchestration platform** for AI-powered workflows in **any environment where auditability, control, and compliance are critical**. While initially focused on software engineering, Holt's immutable audit trails and human-in-the-loop design make it uniquely suited for regulated industries struggling to safely adopt AI.

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
- **Preserves the immutable audit trail** at all costs.

For the complete process, see `DEVELOPMENT_PROCESS.md`.

### **Core Implementation Principles**

When implementing features, adhere to these principles:
1. **Contracts First**: Well-defined interfaces between components are crucial.
2. **Start with the Blackboard**: It is the foundation of the entire system.
3. **Build Incrementally**: Follow the phased delivery plan to minimize risk.
4. **Document Extensively**: The system's complexity demands clarity.

This project represents a unique approach to AI agent orchestration that prioritizes practicality, auditability, and real-world engineering needs over academic novelty.