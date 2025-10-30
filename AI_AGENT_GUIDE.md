# **AI Agent Navigation Guide: What to Read When**

**Purpose**: Navigation guide for AI agents to efficiently find relevant documentation  
**Scope**: Essential - read first for any Holt development task  
**Estimated tokens**: ~600 tokens  
**Read when**: Starting any development task, unsure which documents to consult

## **Quick Start: Core Context**

**Always start here** (required for all tasks):
1. **`README.md`** (~2,000 tokens) - High-level project overview, enterprise features, and the official 6-phase roadmap.
2. **`PROJECT_CONTEXT.md`** (~1,500 tokens) - The project's core philosophy, architectural principles, and vision.
3. **This file** (~600 tokens) - Navigation guidance for specific tasks.
4. **`QUICK_REFERENCE.md`** (~800 tokens) - Key concepts, data structures, and command patterns.

## **Task-Specific Reading Lists**

### **Understanding the System Architecture**
- **`design/holt-system-specification.md`** (~5,700 tokens) - Complete technical architecture overview.
- **`PROJECT_CONTEXT.md`** (~1,500 tokens) - The "why" behind the architecture.

### **Understanding the Roadmap & Vision**
- **`README.md`** (Roadmap section) - The official 6-phase project roadmap.
- **`design/features/phase-X/README.md`** - Detailed goals for each specific phase.
- **`design/future-enhancements.md`** - The long-term vision for enterprise features beyond the core roadmap.

### **Designing New Features**
- **`DEVELOPMENT_PROCESS.md`** (~2,000 tokens) - The three-stage development lifecycle.  
- **`design/holt-feature-design-template.md`** (~3,500 tokens) - The template for creating new feature designs.

### **Implementing Orchestrator Features**
- **`design/holt-orchestrator-component.md`** (~3,200 tokens) - Orchestrator logic and algorithms.
- **`QUICK_REFERENCE.md`** (~800 tokens) - Redis patterns and event flows.

### **Implementing Agent Features**  
- **`design/agent-pup.md`** (~3,300 tokens) - Agent pup architecture and contracts.
- **`QUICK_REFERENCE.md`** (~800 tokens) - Tool execution patterns.

### **Working with the CLI (Observability & Commands)**
- **`design/features/phase-3-coordination/M3.10-cli-observability.md`** - Design for the powerful `watch` and `hoard` filtering and output features.
- **`QUICK_REFERENCE.md`** (~800 tokens) - Command reference.

### **System Integration & Testing**
- **`DEVELOPMENT_PROCESS.md`** (Stage 3) - Integration validation process.
- **`design/holt-system-specification.md`** (sections 8-9) - Error handling and technical details.

## **Strategic Reading & Context Management**

Your goal is to build the most relevant context for your task while respecting token limits. Use this strategic approach:

**1. Start with the Core Context**
Always read the documents listed in the **`Quick Start: Core Context`** section first. They provide the foundational knowledge for any task.

**2. Select Task-Specific Documents**
Use the **`Task-Specific Reading Lists`** or **`Common Navigation Patterns`** sections to identify the specific design documents needed for your objective.

**3. Prioritize Summaries**
To save tokens, prefer reading `QUICK_REFERENCE.md` or component-specific documents before consulting the full `design/holt-system-specification.md`. The token estimates help you budget your context window.

## **Common Navigation Patterns**

### **"I need to understand Holt's architecture"**
→ `PROJECT_CONTEXT.md` + `design/holt-system-specification.md`

### **"What is the project's roadmap?"**
→ `README.md` (Roadmap section) + `design/future-enhancements.md`

### **"I'm designing a new feature"**  
→ `DEVELOPMENT_PROCESS.md` + `design/holt-feature-design-template.md`

### **"I'm implementing CLI observability features"**
→ `design/features/phase-3-coordination/M3.10-cli-observability.md` + `QUICK_REFERENCE.md`

### **"I'm implementing agent functionality"**
→ `design/agent-pup.md` + `QUICK_REFERENCE.md`

## **Getting Help**

When documentation is unclear or insufficient:
1. **Check cross-references** between documents.
2. **Look for examples** in phase-specific READMEs or demo directories.
3. **Refer to quick references** for common patterns.
4. **Ask specific questions** about ambiguous requirements.
