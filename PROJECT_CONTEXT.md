# **Sett Project Context: Purpose, Philosophy & Vision**

## **What is Sett?**

Sett is a **container-native AI agent orchestrator** designed to manage a clan of specialized, tool-equipped AI agents for automating complex software engineering tasks. It is **not** an LLM-chaining library—it is an orchestration engine for real-world toolchains that software professionals use every day.

## **Core Philosophy & Guiding Principles**

### **Pragmatism over novelty (YAGNI)**
We prioritise using existing, battle-hardened tools rather than building our own. This principle applies at all levels:
* Core components: We use Docker for containers and Redis for state because they are excellent. Sett's core is an orchestrator, not a database or container runtime.
* Internal logic: We prefer wrapping an existing, stable tool over reimplementing its functionality. For example, the sett logs command is a thin, user-friendly wrapper around docker logs, not a custom logging pipeline.

### **Zero-configuration, progressively enhanced**
The experience must be seamless out of the box. A developer should be able to get a basic sett running with a single command. Smart defaults cover 90% of use cases, while advanced features are available for those who need them.

### **Small, single-purpose components**
Each element—the orchestrator, the CLI, the agent cub—has a clear, well-defined job and does that one thing excellently. Complexity is managed by composing simple parts.

### **Auditability as a core feature**
Artefacts are immutable. Every decision and agent interaction is recorded on the blackboard, providing a complete, auditable history of the workflow. This makes Sett particularly valuable for regulated industries, compliance workflows, and any environment where AI transparency and accountability are business-critical or legally required.

### **ARM64-first design**
Development and deployment are optimized for ARM64, with AMD64 as a fully supported, compatible target.

### **Principle of least privilege**
Agents run in non-root containers with the minimal set of privileges required to perform their function.

## **What Makes Sett Different**

### **Container-native by design**
Unlike Python-based frameworks, Sett orchestrates agents whose tools are **any command-line tool that can be packaged into a container**. This enables automation of tasks that rely on compilers, CLIs, and infrastructure tools (git, docker, kubectl).

### **Event-driven architecture**
The system uses Redis Pub/Sub for efficient, non-polling communication between components. Agents watch for claims and bid on them, creating a robust, event-driven workflow.

### **Immutable audit trail**
Every artifact is immutable. To handle iteration and feedback, agents create new artifacts that are part of a logical thread, creating a clear historical chain without violating immutability.

### **Human-in-the-loop by design**
The system is explicitly designed for human oversight and intervention, with Question/Answer artifacts and CLI commands for human interaction. This architecture ensures compliance with regulations requiring human review of AI decisions and provides the control mechanisms needed in high-stakes environments.

### **Git-centric workflow**
Sett assumes and requires a clean Git repository. Code artifacts are git commit hashes, and agents are responsible for Git interactions, making the entire workflow version-controlled.

## **Key Architectural Concepts**

### **The Blackboard**
A Redis-based shared state system where all components interact via well-defined data structures. It serves as a lightweight ledger storing metadata and pointers, not large data blobs. **Critically for compliance**: every interaction is logged with timestamps, creating an immutable audit trail that meets regulatory requirements for AI transparency and accountability.

### **Artifacts**
Immutable data objects representing work products. They have:
- **structural_type**: Role in orchestration (Standard, Review, Question, Answer, Failure, Terminal)
- **type**: User-defined, domain-specific string (e.g., "DesignSpec", "CodeCommit")
- **payload**: Main content (often a git commit hash for code)
- **logical_id**: Groups versions of the same logical artifact

### **Claims**
Records of the Orchestrator's decisions about specific Artifacts. Claims go through phases:
1. **Review phase**: Parallel review by multiple agents
2. **Parallel phase**: Concurrent work by multiple agents
3. **Exclusive phase**: Single agent gets exclusive access

### **The Agent Cub**
A lightweight binary that runs as the entrypoint in every agent container. It:
- Watches for claims and bids on them
- Assembles historical context from the blackboard
- Executes the agent's specific tool via a command script
- Posts results back to the blackboard
- Operates concurrently to remain responsive

### **Full Consensus Model (V1)**
The orchestrator waits until it receives a bid from every known agent before proceeding with the grant process. This ensures deterministic, debuggable workflows.

## **Core Workflow**

1. **Bootstrap**: User runs `sett forage --goal "Create a REST API"` 
2. **Initial Artifact**: CLI creates a GoalDefined artifact on the blackboard
3. **Claim Creation**: Orchestrator sees the artifact and creates a corresponding claim
4. **Bidding**: All agents evaluate the claim and submit bids ('review', 'claim', 'exclusive', 'ignore')
5. **Phased Execution**: Orchestrator grants claims in review → parallel → exclusive phases
6. **Work Execution**: Agent cubs execute their tools and create new artifacts
7. **Iteration**: New artifacts trigger new claims, continuing the workflow
8. **Termination**: Workflow ends when an agent creates a Terminal artifact

## **Technology Stack**

### **Core Technologies**
- **Go**: Single module with multiple binaries (orchestrator, CLI, cub)
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
sett/
├── cmd/             # Binaries: sett, orchestrator, cub
├── pkg/             # Shared public packages (blackboard types)
├── internal/        # Private implementation details
├── agents/          # Example agent definitions
├── design/          # Design documents and specifications
│   ├── features/                         # Feature design documents by phase
│   │   ├── phase-1-heartbeat/           # Core infrastructure features
│   │   ├── phase-2-single-agent/       # Basic execution features
│   │   ├── phase-3-coordination/       # Multi-agent workflow features
│   │   └── phase-4-human-loop/         # Production-ready features
│   ├── sett-system-specification.md      # Complete system architecture
│   ├── sett-orchestrator-component.md    # Orchestrator component design
│   ├── agent-cub.md                      # Agent cub component design
│   └── sett-feature-design-template.md   # Systematic development template
└── Makefile
```

## **Documentation Architecture**

The design documentation follows a clear component-based structure optimized for AI agent comprehension:

* **`sett-system-specification.md`** - Complete system overview, architecture, and shared components (blackboard, CLI, configuration)
* **`sett-orchestrator-component.md`** - Focused specification for the orchestrator component's logic and behavior
* **`agent-cub.md`** - Focused specification for the agent cub component's architecture and execution model
* **`sett-feature-design-template.md`** - Systematic template for designing new features with comprehensive analysis framework

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

### **Why immutable artifacts?**
Provides complete audit trail and prevents race conditions in concurrent environments.

### **Why container-native?**
Enables orchestration of any tool that can be containerized, not just Python functions.

### **Why Git-centric?**
Provides version control, enables deterministic workspaces, and leverages existing developer workflows.

### **Why event-driven?**
Ensures maximum efficiency—agents are never too busy to evaluate new opportunities.

## **Success Criteria**

A successful Sett implementation should:
1. **Enable zero-configuration startup** - `sett init && sett up` creates a working system
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

Sett aims to be the **de facto orchestration platform** for AI-powered workflows in **any environment where auditability, control, and compliance are critical**. While initially focused on software engineering, Sett's immutable audit trails and human-in-the-loop design make it uniquely suited for regulated industries struggling to safely adopt AI.

By combining the reliability of containerization with the flexibility of AI agents, Sett enables organizations to automate complex tasks while maintaining **full visibility, control, and regulatory compliance**. This makes it invaluable for:

- **Regulated industries** (finance, healthcare, government) requiring traceable AI decisions
- **Compliance workflows** where every AI action must be documented and auditable
- **Security-sensitive environments** needing controlled AI automation with human oversight
- **Any organization** where AI transparency and accountability are business-critical

## **For Implementation Teams**

### **Development Methodology**

Sett uses a **systematic feature design approach** to ensure quality, consistency, and architectural alignment. Every feature must be designed using the standardized template (`design/sett-feature-design-template.md`) which provides:

- **Comprehensive analysis framework** covering all system components
- **Error-first design** with failure mode and edge case identification
- **Performance and resource planning** from the design phase
- **AI-specific implementation guidance** for systematic development
- **Principle compliance verification** to maintain architectural consistency

This template-driven approach is particularly critical for AI agent development, ensuring robust, auditable features that integrate seamlessly with Sett's architecture.

### **Feature Development Lifecycle**

Sett follows a systematic **three-stage feature development process** designed for iterative collaboration between humans and AI agents. This process ensures quality, consistency, and architectural alignment across all feature development.

#### **Stage 1: Feature Design (Human-AI Collaboration)**

**Purpose**: Create a comprehensive, unambiguous feature specification

**Process**:
1. **Initiate Design**: Start with `design/sett-feature-design-template.md` as the foundation
2. **Iterative Refinement**: 
   - Human provides initial requirements and context
   - AI agent fills out template sections systematically
   - Multiple rounds of discussion and clarification
   - Focus on completeness, clarity, and architectural consistency
3. **Cross-Component Analysis**: Validate impact on all system components (Orchestrator, Cub, CLI, Blackboard)
4. **Dependency Verification**: Ensure prerequisites from earlier phases are satisfied
5. **Design Approval**: Human review and final approval of completed design document

**Quality Gates**:
- All template sections completed with specific, actionable content
- Success criteria are measurable and testable
- Component impact analysis covers every system element
- No architectural contradictions or violations
- Implementation plan is detailed and feasible

**Deliverable**: Approved feature design document stored in `design/features/phase-X/feature-name.md`

#### **Stage 2: Implementation (AI Agent Execution)**

**Purpose**: Systematically implement the approved feature design

**Process**:
1. **Implementation Handoff**: AI agent receives approved design document
2. **Systematic Development**: 
   - Follow implementation steps from design document section 3.2
   - Implement tests before or alongside code (TDD approach)
   - Validate against success criteria from section 1.3 continuously
   - Handle error scenarios identified in section 6
3. **Quality Assurance**:
   - Run all tests defined in section 3.4
   - Verify performance requirements from section 3.3
   - Complete all Definition of Done items from section 5
4. **Integration Testing**: Ensure feature works with existing system components

**Quality Gates**:
- All implementation steps completed successfully
- All tests passing (unit, integration, E2E)
- Performance requirements met and verified
- Error handling implemented and tested
- Definition of Done checklist 100% complete

**Deliverable**: Fully implemented feature with passing tests and documentation

#### **Stage 3: Integration & Validation (Human-AI Verification)**

**Purpose**: Validate feature integration and prepare for delivery

**Process**:
1. **Code Review**: Human review of implementation against design
2. **System Integration**: Full system testing with the new feature
3. **Documentation Update**: Update system documentation if needed
4. **Phase Validation**: Confirm feature contributes to phase success criteria
5. **Handoff Preparation**: Prepare for integration with subsequent features

**Quality Gates**:
- Code quality meets project standards
- System tests pass with new feature integrated
- No regressions in existing functionality
- Phase success criteria remain achievable
- Documentation accurately reflects current state

**Deliverable**: Production-ready feature integrated into the main system

#### **File Organization for Feature Designs**

Feature design documents are organized by delivery phase to maintain clear progression and dependency management:

```
design/features/
├── phase-1-heartbeat/              # Core Infrastructure
│   ├── redis-blackboard-foundation.md
│   ├── basic-orchestrator-engine.md
│   └── cli-lifecycle-commands.md
├── phase-2-single-agent/           # Basic Execution
│   ├── agent-cub-implementation.md
│   ├── git-workspace-integration.md
│   └── basic-claim-execution.md
├── phase-3-coordination/           # Multi-Agent Workflow
│   ├── consensus-bidding-model.md
│   ├── phased-execution-system.md
│   └── controller-worker-pattern.md
└── phase-4-human-loop/             # Production Ready
    ├── question-answer-system.md
    ├── monitoring-health-checks.md
    └── production-readiness.md
```

#### **AI Agent Guidelines for Feature Development**

**During Design Stage**:
- Ask clarifying questions when requirements are ambiguous
- Propose concrete alternatives when trade-offs exist
- Always consider error cases and edge scenarios
- Validate design against existing system architecture
- Identify dependencies on other features or components

**During Implementation Stage**:
- Follow the approved design document exactly
- Implement comprehensive error handling from the start
- Write tests before or alongside implementation
- Validate assumptions with working code
- Report blockers or design issues immediately

**During Integration Stage**:
- Test thoroughly in clean environments
- Document any deviations from the original design
- Verify all quality gates are satisfied
- Communicate integration requirements clearly

This structured approach ensures every feature is well-designed, properly implemented, and seamlessly integrated while maintaining the high standards required for a production-ready system.

### **Implementation Guidelines**

When implementing Sett:
1. **Start with the blackboard** - It's the foundation everything else builds on
2. **Use the feature design template** - Never skip the systematic design phase
3. **Focus on the contracts** - Well-defined interfaces between components are crucial
4. **Design for failure first** - Error handling and edge cases are not afterthoughts
5. **Emphasize testing** - The distributed nature requires comprehensive test coverage
6. **Build incrementally** - Follow the phased approach to minimize risk
7. **Document extensively** - The system's complexity demands clear documentation
8. **Maintain auditability** - Every feature must preserve the immutable audit trail

### **Quality Assurance**

The systematic design template ensures that every feature:
- **Aligns with Sett's guiding principles** (YAGNI, auditability, single-purpose components)
- **Considers all architectural components** (blackboard, orchestrator, cub, CLI)
- **Plans for scale and performance** from the design phase
- **Handles errors and edge cases** robustly
- **Maintains backward compatibility** and integration safety
- **Includes comprehensive testing** across unit, integration, and E2E dimensions

This methodology is essential for maintaining system integrity as Sett scales and evolves, particularly in regulated environments where reliability and auditability are paramount.

This project represents a unique approach to AI agent orchestration that prioritizes practicality, auditability, and real-world engineering needs over academic novelty.