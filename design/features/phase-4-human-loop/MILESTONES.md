# **Phase 4: "Human-in-the-Loop" - Implementation Milestones**

**Phase Goal**: Full featured system with human oversight and production-ready operations.
Thoughts from Cam:
* there are times when one LLM might want to ask questions from the implementer LLM of an earlier Artefact (think implementer asking an implementation detail) - this is a form of late review, but done at the Claim / exlcusive step, not necessarily the review step - we need to support this additional flow.
* human in the loop 2 modes:
    - implicit human decision requests - questions that filter up to the top of the chain that the LLMs cant answer
    - breakpointing - where a human can set explicit breakpoints or step controlls - eg whenever an artefact of type X is created, enter full control to review every decision.  this is complex, so we need to think hard on how to implement this, and what is simple, and what we can build later.
    - also when in full control, the human can take action - eg add an explicit manual review artefact on something to cause it to be re-evaluated.
* a much more sophisticated demo - eg a `Go Microservice & OpenAPI Spec` where there are designs, comprehensive design reviews, implementations (including asking implementation questions back to the architect), implementation reviews & tooling that runs the tests checks code coverage etc,  then documentation creation & reviews etc, then final total review including all genarated artefacts and code changes. -  showing off extensive use of tools AND LLMS.  and including the possibility of questions escalating up to the human.
* 


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
- `holt questions [--wait]` command implementation
- `holt answer <question-id> "response"` command
- Answer artefact creation and claim unblocking
- Timeout handling for unanswered questions

**Deliverables**:
- `cmd/holt/commands/questions.go` - Questions command
- `cmd/holt/commands/answer.go` - Answer command
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
- `/healthz` endpoints for orchestrator and pups
- Structured JSON logging throughout
- Performance metrics collection
- Operational debugging commands

**Deliverables**:
- Health check HTTP servers in orchestrator and pup
- Logging framework integration
- Metrics collection infrastructure
- Debug tooling and commands

**Design Document**: TBD

---

### **M4.3: Instance Destruction**
**Status**: Not Started
**Dependencies**: M4.2
**Estimated Effort**: Medium

**Goal**: Safely destroy Holt instances and all associated data

**Scope**:
- `holt destroy --name <instance> [--force]` command implementation
- Validation that instance is stopped before destruction
- Complete removal of all Redis keys for the instance:
  - `holt:{instance_name}:artefact:*`
  - `holt:{instance_name}:claim:*`
  - `holt:{instance_name}:thread:*`
  - `holt:{instance_name}:lock`
  - Entry in `holt:instances` hash
- Confirmation prompts and safety checks
- `--force` flag to bypass confirmation (for scripting)
- Comprehensive audit logging of destruction operations

**Deliverables**:
- `cmd/holt/commands/destroy.go` - Destroy command
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
- **SHOULD** create audit artefact before destruction (future enhancement)

**Example UX**:
```bash
# Basic usage (requires confirmation)
$ holt destroy --name myproject
WARNING: This will permanently delete ALL data for instance 'myproject'
This includes:
  - All artefacts and claims
  - All thread history
  - All metadata

Type the instance name to confirm: myproject
Destroying instance 'myproject'...
✓ Removed 1,234 artefacts
✓ Removed 567 claims
✓ Removed 89 threads
✓ Removed metadata
Instance 'myproject' destroyed successfully

# Force mode (no confirmation, for scripts)
$ holt destroy --name myproject --force
Instance 'myproject' destroyed successfully

# Error: Instance still running
$ holt destroy --name myproject
Error: Cannot destroy instance 'myproject' - instance is still running
Run 'holt down --name myproject' first

# Error: Instance doesn't exist
$ holt destroy --name nonexistent
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

After Phase 4 completion, Holt v1.0 is production-ready for:
- Software engineering workflows
- Regulated industry compliance scenarios
- Enterprise deployments requiring auditability
