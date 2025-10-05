# **Phase 1: "Heartbeat" - Core Infrastructure**

**Goal**: Prove the blackboard architecture works with basic orchestrator and CLI functionality.

## **Phase Success Criteria**

- `sett forage --goal "hello world"` creates initial artifact
- Orchestrator creates corresponding claim
- System state visible via Redis CLI
- All core data structures implemented and functional

## **Implementation Milestones**

📋 **See [MILESTONES.md](./MILESTONES.md)** for the complete breakdown of 6 implementable milestones, dependencies, and implementation order.

## **Key Features for This Phase**

1. **Redis Blackboard Foundation**
   - Complete key schemas and data structures
   - Pub/Sub channel implementation
   - Thread tracking with Redis ZSET

2. **Basic Orchestrator Engine** 
   - Artifact watching via Redis Pub/Sub
   - Claim creation and management
   - Basic event-driven architecture

3. **CLI Lifecycle Commands**
   - `sett up`, `sett down`, `sett list`, `sett forage`
   - Project initialization and teardown
   - Basic workflow initiation

## **Implementation Constraints**

- No agent execution yet (Phase 2 dependency)
- Focus on data structures and basic orchestration
- Minimal error handling (expanded in later phases)
- No human-in-the-loop features (Phase 4)

## **Testing Requirements**

- Unit tests for all blackboard operations
- Integration tests with real Redis instance
- CLI command functional testing
- State verification via Redis CLI

## **Dependencies**

This is the foundation phase - no external dependencies on other phases.

## **Deliverables**

- Working Redis blackboard with complete schemas
- Functional orchestrator watching for artifacts
- CLI commands for sett lifecycle management
- Initial artifact creation workflow