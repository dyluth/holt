# AI Agent Navigation Guide

**Purpose**: A machine-readable index for AI agents to efficiently find relevant documentation and manage context windows.
**Scope**: Essential - read first for any development task.

---

## 1. Core Context Documents

For any non-trivial task, load this core set of documents to understand the project's high-level architecture, goals, and current state.

- **`README.md`** (~2,000 tokens) - Project overview, quick start, and links to key documents.
- **`ROADMAP.md`** (~500 tokens) - The official 6-phase project roadmap.
- **`PROJECT_CONTEXT.md`** (~1,500 tokens) - The project's core philosophy and architectural principles.
- **`QUICK_REFERENCE.md`** (~1,000 tokens) - Essential data structures, CLI commands, and Redis patterns.

---

## 2. Task-Specific Reading Lists

To supplement the core context, select from the lists below based on your specific task.

### **Topic: Onboarding & Contributing**
- **`CONTRIBUTING.md`** (~1,000 tokens) - How to set up the environment and contribute.
- **`DEVELOPMENT_PROCESS.md`** (~2,000 tokens) - The mandatory design-first development lifecycle.

### **Topic: Enterprise & Compliance Features**
- **`ENTERPRISE_GRADE_HOLT.md`** (~1,000 tokens) - Explains built-in security, auditability, and governance.
- **`HOLT_COMPLIANCE_GUIDE.md`** (~1,500 tokens) - Maps Holt features to HIPAA, SOC 2, and ISO 27001 controls.

### **Topic: Learning by Example**
- **`docs/HOW_TO_1_BUILD_A_SIMPLE_AGENT.md`** - Walkthrough of the basic file-creation agent.
- **`docs/HOW_TO_2_BUILD_A_MULTI_AGENT_WORKFLOW.md`** - Deep dive into the collaborative recipe-generator demo.
- **`docs/HOW_TO_3_BUILD_AN_IAC_AGENT.md`** - Guide to the advanced Terraform agent.

### **Topic: Core Architecture Deep Dive**
- **`design/holt-system-specification.md`** (~5,000 tokens) - The complete technical architecture.
- **`design/holt-orchestrator-component.md`** (~3,000 tokens) - The Orchestrator's internal logic.
- **`design/agent-pup.md`** (~3,000 tokens) - The `pup` architecture and tool execution contract.

### **Topic: Designing a New Feature**
- **`DEVELOPMENT_PROCESS.md`** (~2,000 tokens) - The three-stage development lifecycle.
- **`design/holt-feature-design-template.md`** (~3,500 tokens) - The required template for all new feature designs.

---

## 3. Common Navigation Patterns

- **Goal: "Understand the project's security and compliance features."**
  - → `ENTERPRISE_GRADE_HOLT.md`
  - → `HOLT_COMPLIANCE_GUIDE.md`

- **Goal: "Learn how to build my first agent."**
  - → `docs/HOW_TO_1_BUILD_A_SIMPLE_AGENT.md`
  - → `QUICK_REFERENCE.md` (for data structures)

- **Goal: "Design a new feature for Phase 4."**
  - → `ROADMAP.md` (to understand Phase 4 goals)
  - → `DEVELOPMENT_PROCESS.md` (to understand the process)
  - → `design/holt-feature-design-template.md` (to create the design document)

- **Goal: "Fix a bug in the Orchestrator."**
  - → `design/holt-orchestrator-component.md` (to understand the logic)
  - → `QUICK_REFERENCE.md` (for Redis patterns)
