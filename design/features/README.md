# **Sett Feature Design Documents**

This directory contains feature design documents organized by delivery phase. Each feature must be designed using the systematic template (`../sett-feature-design-template.md`) before implementation.

## **Directory Structure**

* **`phase-1-heartbeat/`** - Core Infrastructure features that prove the blackboard architecture works
* **`phase-2-single-agent/`** - Basic execution features enabling one agent to claim and execute work  
* **`phase-3-coordination/`** - Multi-agent workflow features with review→parallel→exclusive phases
* **`phase-4-human-loop/`** - Production-ready features with human oversight and operational capabilities

## **Feature Development Process**

1. **Design Stage**: Start with template, iterate with human-AI collaboration until approved
2. **Implementation Stage**: AI agent systematically implements approved design
3. **Integration Stage**: Human-AI validation and system integration

See `../../PROJECT_CONTEXT.md` for complete process documentation.

## **Naming Convention**

Feature design files should be named descriptively:
- Use kebab-case: `redis-blackboard-foundation.md`
- Focus on the primary capability: `consensus-bidding-model.md`
- Avoid abbreviations unless universally understood

## **Quality Standards**

Every feature design must:
- Complete all sections of the template with specific, actionable content
- Define measurable success criteria and comprehensive testing strategy
- Analyze impact on all system components (Orchestrator, Cub, CLI, Blackboard)
- Include error handling and edge case analysis
- Align with Sett's guiding principles and architectural consistency

## **Phase Dependencies**

Features must respect phase dependencies:
- Phase 2 features depend on Phase 1 completion
- Phase 3 features depend on Phase 2 completion  
- Phase 4 features depend on Phase 3 completion

Cross-phase dependencies should be explicitly documented in the design's section 2 (Component Impact Analysis).