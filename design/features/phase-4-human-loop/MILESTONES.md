# **Phase 4: "Human-in-the-Loop" - Implementation Milestones**

**Phase Goal**: Full featured system with human oversight and production-ready operations.

**Phase Success Criteria**:
- Complex workflows with human decision points
- Production-ready operational features
- Comprehensive error handling and monitoring
- Complete audit trail for regulated environments

---

## **Milestone Overview**

Phase 4 milestones focus on production readiness, human interaction, and operational safety.

### **M4.1: Question-Answer System**
**Status**: Not Started
**Dependencies**: Phase 3 completion
**Estimated Effort**: Medium

**Goal**: Enable agents to escalate questions to humans and receive answers

**Scope**:
- Question structural type handling in orchestrator
- `sett questions [--wait]` command implementation
- `sett answer <question-id> "response"` command
- Answer artifact creation and claim unblocking
- Timeout handling for unanswered questions

**Deliverables**:
- `cmd/sett/commands/questions.go` - Questions command
- `cmd/sett/commands/answer.go` - Answer command
- Orchestrator Question/Answer workflow logic
- Integration tests for human-in-the-loop scenarios

**Design Document**: TBD

---

### **M4.2: Health Checks and Monitoring**
**Status**: Not Started
**Dependencies**: M4.1
**Estimated Effort**: Medium

**Goal**: Production-ready monitoring and observability

**Scope**:
- `/healthz` endpoints for orchestrator and cubs
- Structured JSON logging throughout
- Performance metrics collection
- Operational debugging commands

**Deliverables**:
- Health check HTTP servers in orchestrator and cub
- Logging framework integration
- Metrics collection infrastructure
- Debug tooling and commands

**Design Document**: TBD

---

### **M4.3: Instance Destruction**
**Status**: Not Started
**Dependencies**: M4.2
**Estimated Effort**: Medium

**Goal**: Safely destroy Sett instances and all associated data

**Scope**:
- `sett destroy --name <instance> [--force]` command implementation
- Validation that instance is stopped before destruction
- Complete removal of all Redis keys for the instance:
  - `sett:{instance_name}:artifact:*`
  - `sett:{instance_name}:claim:*`
  - `sett:{instance_name}:thread:*`
  - `sett:{instance_name}:lock`
  - Entry in `sett:instances` hash
- Confirmation prompts and safety checks
- `--force` flag to bypass confirmation (for scripting)
- Comprehensive audit logging of destruction operations

**Deliverables**:
- `cmd/sett/commands/destroy.go` - Destroy command
- `internal/instance/destroyer.go` - Instance destruction logic
- Confirmation and safety validation
- Integration tests for destruction scenarios
- Documentation for data recovery and backups

**Design Document**: `instance-destruction.md`

**Safety Requirements**:
- **MUST** verify instance is not running (no lock exists)
- **MUST** prompt for confirmation with instance name re-entry
- **MUST** log destruction operation with timestamp and operator
- **SHOULD** support dry-run mode (`--dry-run` flag)
- **SHOULD** create audit artifact before destruction (future enhancement)

**Example UX**:
```bash
# Basic usage (requires confirmation)
$ sett destroy --name myproject
WARNING: This will permanently delete ALL data for instance 'myproject'
This includes:
  - All artifacts and claims
  - All thread history
  - All metadata

Type the instance name to confirm: myproject
Destroying instance 'myproject'...
✓ Removed 1,234 artifacts
✓ Removed 567 claims
✓ Removed 89 threads
✓ Removed metadata
Instance 'myproject' destroyed successfully

# Force mode (no confirmation, for scripts)
$ sett destroy --name myproject --force
Instance 'myproject' destroyed successfully

# Error: Instance still running
$ sett destroy --name myproject
Error: Cannot destroy instance 'myproject' - instance is still running
Run 'sett down --name myproject' first

# Error: Instance doesn't exist
$ sett destroy --name nonexistent
Error: Instance 'nonexistent' not found
```

---

### **M4.4: Production Documentation**
**Status**: Not Started
**Dependencies**: M4.3
**Estimated Effort**: Small

**Goal**: Complete production deployment and operational documentation

**Scope**:
- Deployment guides for various environments
- Operational runbooks for common scenarios
- Troubleshooting guides
- Security hardening recommendations
- Backup and recovery procedures

**Deliverables**:
- Production deployment guide
- Operational runbooks
- Troubleshooting documentation
- Security best practices guide

**Design Document**: N/A (documentation only)

---

## **Phase 4 Completion Criteria**

Phase 4 is complete when:
- ✅ Question/Answer workflow fully functional
- ✅ Health checks operational for all components
- ✅ Instance destruction works safely with audit logging
- ✅ Production documentation complete
- ✅ All Phase 4 tests passing
- ✅ Security review completed
- ✅ Performance benchmarks met

## **Next Steps**

After Phase 4 completion, Sett v1.0 is production-ready for:
- Software engineering workflows
- Regulated industry compliance scenarios
- Enterprise deployments requiring auditability
