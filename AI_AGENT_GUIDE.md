# **AI Agent Navigation Guide: What to Read When**

**Purpose**: Navigation guide for AI agents to efficiently find relevant documentation  
**Scope**: Essential - read first for any Sett development task  
**Estimated tokens**: ~500 tokens  
**Read when**: Starting any development task, unsure which documents to consult

## **Quick Start: Essential Reading**

**Always start here** (required for all tasks):
1. **`PROJECT_CONTEXT.md`** (~1,500 tokens) - Project overview, philosophy, architecture concepts
2. **This file** (~500 tokens) - Navigation guidance for specific tasks

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

## **Reading Priority Guide**

### **Essential (always read)**
- `PROJECT_CONTEXT.md` - Required context for all development
- `AI_AGENT_GUIDE.md` - This navigation guide
- `QUICK_REFERENCE.md` - Key patterns and structures

### **Task-Specific (read as needed)**
- Component specifications (~3,000 tokens each)
- Feature design documents (~1,500 tokens each)  
- Development process documentation (~2,000 tokens)

### **Reference (consult when needed)**
- Complete system specification (~5,700 tokens)
- Feature design template (~3,500 tokens)
- Phase-specific documentation

## **Context Window Management**

**Recommended approach**:
1. **Start minimal**: Read only essential documents first
2. **Add task-specific**: Include only documents relevant to your current task
3. **Reference on-demand**: Consult detailed specifications only when needed
4. **Use quick references**: Prefer summary documents over full specifications

**Token estimates** are provided for each document to help manage context window usage.

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

## **When Context Window is Limited**

If context window is constrained, prioritize in this order:
1. `PROJECT_CONTEXT.md` (required context)
2. `QUICK_REFERENCE.md` (essential patterns) 
3. Most relevant component specification
4. Consult full system specification only for specific details

## **Getting Help**

When documentation is unclear or insufficient:
1. **Check cross-references** between documents
2. **Look for examples** in phase-specific READMEs
3. **Refer to quick references** for common patterns
4. **Ask specific questions** about ambiguous requirements