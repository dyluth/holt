# **AI Agent Navigation Guide: What to Read When**

**Purpose**: Navigation guide for AI agents to efficiently find relevant documentation  
**Scope**: Essential - read first for any Sett development task  
**Estimated tokens**: ~500 tokens  
**Read when**: Starting any development task, unsure which documents to consult

## **Quick Start: Core Context**

**Always start here** (required for all tasks):
1. **`PROJECT_CONTEXT.md`** (~1,500 tokens) - Project overview, philosophy, architecture concepts
2. **This file** (~500 tokens) - Navigation guidance for specific tasks
3. **`QUICK_REFERENCE.md`** (~800 tokens) - Key concepts, data structures, and patterns

## **Task-Specific Reading Lists**

### **Understanding the System**
- **`design/sett-system-specification.md`** (~5,700 tokens) - Complete architecture overview
- **`QUICK_REFERENCE.md`** (~800 tokens) - Key concepts and data structures

### **Designing Features**
- **`DEVELOPMENT_PROCESS.md`** (~2,000 tokens) - Three-stage development lifecycle  
- **`design/sett-feature-design-template.md`** (~3,500 tokens) - Systematic design template
- **`design/features/phase-X/README.md`** (~300 tokens each) - Phase-specific constraints

### **Implementing Orchestrator Features**
- **`design/sett-orchestrator-component.md`** (~3,200 tokens) - Orchestrator logic and algorithms
- **`QUICK_REFERENCE.md`** (~800 tokens) - Redis patterns and event flows

### **Implementing Agent Features**  
- **`design/agent-cub.md`** (~3,300 tokens) - Agent cub architecture and contracts
- **`QUICK_REFERENCE.md`** (~800 tokens) - Tool execution patterns

### **Working with CLI/User Interface**
- **`design/sett-system-specification.md`** (sections 6-7) - CLI commands and human interaction
- **`QUICK_REFERENCE.md`** (~800 tokens) - Command reference

### **System Integration & Testing**
- **`design/sett-system-specification.md`** (sections 8-9) - Error handling and technical details
- **`DEVELOPMENT_PROCESS.md`** (Stage 3) - Integration validation process

## **Strategic Reading & Context Management**

Your goal is to build the most relevant context for your task while respecting token limits. Use this strategic approach:

**1. Start with the Core Context**
Always read the documents listed in the **`Quick Start: Core Context`** section first. They provide the foundational knowledge for any task.

**2. Select Task-Specific Documents**
Use the **`Task-Specific Reading Lists`** or **`Common Navigation Patterns`** sections to identify the specific design documents needed for your objective.

**3. Prioritize Summaries**
To save tokens, prefer reading `QUICK_REFERENCE.md` or component-specific documents before consulting the full `design/sett-system-specification.md`. The token estimates help you budget your context window.

**Example Workflow (Constrained Context):**
1.  Read `PROJECT_CONTEXT.md` and `QUICK_REFERENCE.md`.
2.  Identify and read the most relevant component document (e.g., `design/sett-orchestrator-component.md`).
3.  If you still need more detail, search within the full `design/sett-system-specification.md` for specific keywords rather than reading the whole file.

## **Common Navigation Patterns**

### **"I need to understand Sett's architecture"**
→ `PROJECT_CONTEXT.md` + `QUICK_REFERENCE.md` + `design/sett-system-specification.md`

### **"I'm designing a new feature"**  
→ `DEVELOPMENT_PROCESS.md` + `design/sett-feature-design-template.md` + relevant phase README

### **"I'm implementing orchestrator logic"**
→ `design/sett-orchestrator-component.md` + `QUICK_REFERENCE.md`

### **"I'm implementing agent functionality"**
→ `design/agent-cub.md` + `QUICK_REFERENCE.md`

### **"I need to integrate/test features"**
→ `DEVELOPMENT_PROCESS.md` (Stage 3) + relevant component specifications

## **Getting Help**

When documentation is unclear or insufficient:
1. **Check cross-references** between documents
2. **Look for examples** in phase-specific READMEs
3. **Refer to quick references** for common patterns
4. **Ask specific questions** about ambiguous requirements
