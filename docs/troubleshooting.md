# Holt Troubleshooting Guide

**Target Audience:** Developers encountering issues with Holt workflows

**Scope:** Common problems, causes, solutions, and debugging commands

---

## Table of Contents

1. [Holt Won't Start](#holt-wont-start)
2. [Agent Won't Execute](#agent-wont-execute)
3. [Git Workspace Errors](#git-workspace-errors)
4. [Blackboard State Issues](#blackboard-state-issues)
   - [M3.3 Feedback Loop Issues](#error-max-review-iterations-reached-m33)
5. [Docker & Container Problems](#docker--container-problems)
6. [Performance Issues](#performance-issues)
7. [Debugging Commands](#debugging-commands)

---

## Holt Won't Start

### Error: "holt.yml not found or invalid"

**Symptoms:**
```
❌ holt.yml not found or invalid
   No configuration file found in the current directory.
```

**Cause:** No `holt.yml` file in current directory, or file has syntax errors.

**Solution:**
```bash
# Initialize new project
holt init

# Or verify holt.yml exists
ls -la holt.yml

# Check YAML syntax
cat holt.yml
```

---

### Error: "Git workspace is not clean"

**Symptoms:**
```
❌ Git workspace is not clean
   You have uncommitted changes:
   M  src/main.go
   ?? temp.txt
```

**Cause:** Uncommitted changes or untracked files in Git repository.

**Solution:**
```bash
# Option 1: Commit changes
git add .
git commit -m "Work in progress"

# Option 2: Stash temporarily
git stash

# Option 3: Force start (use with caution)
holt up --force
```

**Debug Commands:**
```bash
# Check workspace status
git status

# See what files are dirty
git status --porcelain

# View uncommitted changes
git diff
```

---

### Error: "Redis connection failed"

**Symptoms:**
```
❌ Failed to start orchestrator
   Could not connect to Redis at localhost:6379
```

**Cause:** Redis container not running, port conflict, or Docker networking issue.

**Solution:**
```bash
# Check if Redis container is running
docker ps | grep redis

# Check Redis logs
holt logs redis

# Restart Holt instance
holt down
holt up

# Check for port conflicts
netstat -an | grep 6379
```

**Debug Commands:**
```bash
# List all containers for this instance
docker ps -a --filter "name=holt-"

# Inspect Redis container
docker inspect holt-{instance}-redis

# Test Redis connectivity
docker exec holt-{instance}-redis redis-cli PING
```

---

### Error: "instance 'default-1' already exists"

**Symptoms:**
```
❌ instance 'default-1' already exists
   Found existing containers with this instance name.
```

**Cause:** Previous instance with same name still running.

**Solution:**
```bash
# Stop existing instance
holt down --name default-1

# Or use different name
holt up --name my-instance

# Or list and clean up
holt list
holt down --name <old-instance>
```

---

### Error: "workspace already in use by instance X"

**Symptoms:**
```
❌ workspace already in use by instance default-1
   Another Holt instance is running in this directory.
```

**Cause:** Another Holt instance is already running in this Git repository.

**Solution:**
```bash
# Check running instances
holt list

# Stop the instance using this workspace
holt down

# Or run in different directory
cd ../other-project && holt up
```

---

## Agent Won't Execute

### Agent Container Not Starting

**Symptoms:**
- `docker ps` doesn't show agent container
- `holt logs <agent>` shows "container not found"

**Cause:** Docker image not built, configuration error in holt.yml, or Docker daemon issue.

**Solution:**
```bash
# Verify image exists
docker images | grep <agent-name>

# Build agent image
docker build -t <agent-name>:latest -f agents/<agent-name>/Dockerfile .

# Check holt.yml configuration
cat holt.yml | grep -A 5 agents:

# Restart instance
holt down && holt up
```

**Debug Commands:**
```bash
# Check Docker daemon status
docker info

# View agent container status (including stopped)
docker ps -a --filter "name=agent"

# Inspect agent container
docker inspect holt-{instance}-agent-{agent-name}

# Check container logs
docker logs holt-{instance}-agent-{agent-name}
```

---

### Agent Receives Claim But Doesn't Execute

**Symptoms:**
- Claim created on blackboard
- Agent container running
- No artefact produced

**Cause:** Agent not bidding, bidding logic error, or consensus not reached.

**Solution:**
```bash
# Check agent logs for bidding activity
holt logs <agent-name>

# Look for lines like:
# "Received claim event"
# "Submitting bid: exclusive"
# "Executing work for claim"

# Verify agent container is healthy
docker exec holt-{instance}-agent-{agent-name} wget -O- http://localhost:8080/healthz
```

**Debug Commands:**
```bash
# Check blackboard for claims
holt hoard

# Query Redis directly for bids
docker exec holt-{instance}-redis redis-cli HGETALL holt:{instance}:claim:{claim-id}:bids

# Check orchestrator logs
holt logs orchestrator
```

---

### Agent Executes But Creates Failure Artefact

**Symptoms:**
```bash
holt hoard
# Shows Failure artefact instead of expected result
```

**Cause:** Agent tool script error, invalid output JSON, or git commit validation failed.

**Solution:**
```bash
# Check agent logs for stderr output
holt logs <agent-name>

# Look for error messages like:
# "exit code: 1"
# "JSON parse error"
# "Git commit validation failed"

# Test agent script locally
cat test-input.json | agents/<agent-name>/run.sh

# Verify script outputs valid JSON
agents/<agent-name>/run.sh < test-input.json | jq .
```

**Debug Commands:**
```bash
# Get Failure artefact details
holt hoard | grep -A 10 "Failure"

# Check artefact payload for error details
docker exec holt-{instance}-redis redis-cli HGET holt:{instance}:artefact:{id} payload
```

---

### Error: "Git commit validation failed"

**Symptoms:**
```
Failure artefact payload:
"Git commit validation failed: commit abc123 does not exist"
```

**Cause:** Agent returned CodeCommit artefact with invalid or non-existent commit hash.

**Solution:**
```bash
# Check if commit exists in workspace
git log --oneline | grep abc123

# Verify agent script commits BEFORE getting hash
# run.sh should have:
git commit -m "message"
commit_hash=$(git rev-parse HEAD)  # AFTER commit

# Not this (wrong order):
commit_hash=$(git rev-parse HEAD)  # BEFORE commit
git commit -m "message"

# Check workspace mount in container
docker inspect holt-{instance}-agent-{agent-name} | grep -A 10 Mounts
```

**Debug Commands:**
```bash
# Check git history in workspace
git log --oneline -20

# Verify workspace is mounted correctly
docker exec holt-{instance}-agent-{agent-name} ls -la /workspace

# Check git config in container
docker exec holt-{instance}-agent-{agent-name} git config --list
```

---

## Git Workspace Errors

### Error: "not a Git repository"

**Symptoms:**
```
❌ not a Git repository
   Holt requires a Git repository to manage workflows.
```

**Cause:** Current directory is not a Git repository.

**Solution:**
```bash
# Initialize Git repository
git init

# Create initial commit
echo "# Project" > README.md
git add .
git commit -m "Initial commit"

# Then initialize Holt
holt init
```

---

### Error: "permission denied" When Agent Commits

**Symptoms:**
Agent logs show:
```
error: cannot open .git/COMMIT_EDITMSG: Permission denied
```

**Cause:** Agent container user doesn't have write permissions on workspace.

**Solution:**
```bash
# Verify workspace mode in holt.yml
cat holt.yml
# Should have:
agents:
  my-agent:
    workspace:
      mode: rw  # Not "ro"

# Check workspace directory permissions
ls -la

# Ensure git directory is accessible
chmod -R 755 .git

# Restart instance
holt down && holt up
```

---

### Workspace Out of Sync

**Symptoms:**
- Files created by agent don't appear in workspace
- Git history different than expected

**Cause:** Multiple instances running, workspace mount issues, or agent not committing.

**Solution:**
```bash
# Verify only one instance running in this workspace
holt list

# Check git log for agent commits
git log --oneline --author="Holt"

# Verify mounts
docker inspect holt-{instance}-agent-{agent-name} | grep -A 10 "Mounts"

# Restart with clean state
holt down
git status  # Should be clean
holt up
```

---

## Blackboard State Issues

### Artefacts Not Appearing

**Symptoms:**
```bash
holt hoard
# Shows empty or unexpected results
```

**Cause:** Redis data cleared, wrong instance name, or forage command failed.

**Solution:**
```bash
# Verify instance name
holt list

# Check for specific instance
holt hoard --name <instance-name>

# Verify Redis contains data
docker exec holt-{instance}-redis redis-cli KEYS "holt:*"

# Check orchestrator logs
holt logs orchestrator
```

**Debug Commands:**
```bash
# List all artefacts in Redis
docker exec holt-{instance}-redis redis-cli KEYS "holt:{instance}:artefact:*"

# Get specific artefact
docker exec holt-{instance}-redis redis-cli HGETALL "holt:{instance}:artefact:{uuid}"

# Count artefacts
docker exec holt-{instance}-redis redis-cli KEYS "holt:{instance}:artefact:*" | wc -l
```

---

### Claims Stuck in "pending" State

**Symptoms:**
Claim never progresses from `pending_exclusive` to `complete`.

**Cause:** Agent not bidding, agent crashed, or orchestrator stalled.

**Solution:**
```bash
# Check claim status
docker exec holt-{instance}-redis redis-cli HGET holt:{instance}:claim:{uuid} status

# Check if bids were submitted
docker exec holt-{instance}-redis redis-cli HGETALL holt:{instance}:claim:{uuid}:bids

# Verify orchestrator is running
holt logs orchestrator

# Verify agent is running
holt logs <agent-name>

# Restart if needed
holt down && holt up
```

---

### Error: "Max review iterations reached" (M3.3+)

**Symptoms:**
```
Failure artefact payload:
"Max review iterations (3) reached for artefact abc123 (version 4)"
```

**Cause:** Feedback loop hit configured iteration limit (`orchestrator.max_review_iterations`).

**What Happened:**
1. Agent produced work (v1)
2. Reviewer rejected with feedback
3. Agent reworked (v2)
4. Reviewer rejected again
5. Process repeated until reaching max iterations
6. Orchestrator terminated with Failure artefact

**Solution:**
```bash
# Option 1: Increase iteration limit in holt.yml
cat >> holt.yml <<EOF
orchestrator:
  max_review_iterations: 5  # Increase from default 3
EOF

# Option 2: Investigate why agent and reviewer disagree
# Check review feedback in audit trail
holt hoard | grep -A 5 "Review"

# Check agent's iterations
docker exec holt-{instance}-redis redis-cli KEYS "holt:{instance}:thread:*"

# Option 3: Fix agent or reviewer logic
# - Update agent to better address feedback
# - Update reviewer criteria to be more lenient
```

**Debug Commands:**
```bash
# View iteration history for an artefact
holt hoard | grep -B 2 -A 5 "version"

# Check termination reason
docker exec holt-{instance}-redis redis-cli HGET holt:{instance}:claim:{uuid} termination_reason

# View Failure artefact details
docker exec holt-{instance}-redis redis-cli HGETALL holt:{instance}:artefact:{failure-uuid}
```

---

### Feedback Loop Not Working (M3.3+)

**Symptoms:**
- Reviewer rejects work
- Claim terminates instead of creating feedback claim
- Agent not automatically reassigned for rework

**Cause:** Missing M3.3 orchestrator image, configuration issue, or orchestrator bug.

**Solution:**
```bash
# Verify orchestrator has M3.3 code
docker exec holt-{instance}-orchestrator /app/orchestrator --version
# Should show version with M3.3 support

# Rebuild orchestrator with M3.3
make docker-orchestrator

# Restart instance
holt down && holt up

# Check orchestrator logs for feedback events
holt logs orchestrator | grep "feedback_claim"
```

**Expected Log Messages:**
```
[Orchestrator] Review rejection detected for claim abc123
[Orchestrator] Created feedback claim def456 for agent coder-agent (iteration 2)
[Orchestrator] Feedback claim def456 completed by agent coder-agent
```

**Debug Commands:**
```bash
# Check for feedback claims in Redis
docker exec holt-{instance}-redis redis-cli KEYS "holt:{instance}:claim:*"
docker exec holt-{instance}-redis redis-cli HGET holt:{instance}:claim:{uuid} status
# Look for "pending_assignment" status

# Check for termination reasons
docker exec holt-{instance}-redis redis-cli HGET holt:{instance}:claim:{uuid} termination_reason
```

---

### Version Not Incrementing (M3.3+)

**Symptoms:**
- Agent processes feedback claim
- New artefact created with version=1 instead of version=2
- Logical IDs don't match (breaks thread continuity)

**Cause:** Pup not detecting feedback claim, or missing M3.3 Pup code.

**Solution:**
```bash
# Verify agent Pup has M3.3 code
docker exec holt-{instance}-agent-{agent-name} /app/pup --version

# Rebuild agent images with M3.3 Pup
docker build -t {agent-name}:latest -f agents/{agent-name}/Dockerfile .

# Restart instance
holt down && holt up

# Verify version progression in audit trail
holt hoard | grep -A 3 "logical_id"
```

**Expected Behavior:**
```
First attempt:
  logical_id: abc-123, version: 1, type: CodeCommit

After feedback:
  logical_id: abc-123, version: 2, type: CodeCommit  <- Same logical_id, incremented version

After more feedback:
  logical_id: abc-123, version: 3, type: CodeCommit  <- Continues incrementing
```

**Debug Commands:**
```bash
# Check artefact version progression
docker exec holt-{instance}-redis redis-cli ZRANGE holt:{instance}:thread:{logical-id} 0 -1 WITHSCORES

# Verify claim has additional_context_ids (feedback claim indicator)
docker exec holt-{instance}-redis redis-cli HGET holt:{instance}:claim:{uuid} additional_context_ids

# Check Pup logs for version management
holt logs {agent-name} | grep "Creating rework artefact"
```

---

### Error: "Missing agent configuration for feedback" (M3.3+)

**Symptoms:**
```
Failure artefact payload:
"Cannot create feedback claim: agent with role 'Coder' no longer exists in configuration"
```

**Cause:** Original agent that produced work was removed from `holt.yml` before feedback loop completed.

**Solution:**
```bash
# Option 1: Re-add the agent to holt.yml
cat >> holt.yml <<EOF
agents:
  coder-agent:
    role: "Coder"
    image: "coder-agent:latest"
    command: ["/app/run.sh"]
    workspace:
      mode: rw
EOF

# Option 2: Manually terminate the stuck workflow
# (Orchestrator already created Failure artefact)
holt hoard  # Verify Failure artefact exists

# Restart with corrected configuration
holt down && holt up
```

**Prevention:**
- Don't remove agents from configuration during active workflows
- Wait for workflows to complete before changing agent configuration
- Monitor `holt hoard` before modifying `holt.yml`

---

## Docker & Container Problems

### Docker Daemon Not Running

**Symptoms:**
```
Cannot connect to the Docker daemon at unix:///var/run/docker.sock
```

**Cause:** Docker service not started.

**Solution:**
```bash
# Linux
sudo systemctl start docker

# macOS
# Start Docker Desktop application

# Verify Docker is running
docker info
```

---

### Port Conflicts

**Symptoms:**
```
Error: port 6379 is already allocated
```

**Cause:** Another service using Redis default port or multiple Holt instances.

**Solution:**
```bash
# Find what's using the port
lsof -i :6379

# Stop conflicting service
# Or let Holt auto-assign different port (it does this automatically)

# If needed, manually stop old containers
docker ps -a | grep redis
docker rm -f <container-id>
```

---

### Out of Disk Space

**Symptoms:**
```
Error: no space left on device
```

**Cause:** Docker images and containers consuming disk space.

**Solution:**
```bash
# Check disk usage
df -h

# Clean up Docker
docker system prune -a

# Remove unused images
docker images
docker rmi <unused-images>

# Remove old Holt containers
docker ps -a | grep holt-
docker rm $(docker ps -a -q --filter "name=holt-")
```

---

### Container Health Check Failures

**Symptoms:**
```
Container holt-{instance}-orchestrator is unhealthy
```

**Cause:** Redis connection lost, application crash, or startup timeout.

**Solution:**
```bash
# Check container logs
docker logs holt-{instance}-orchestrator

# Check health endpoint
docker exec holt-{instance}-orchestrator wget -O- http://localhost:8080/healthz

# Restart container
docker restart holt-{instance}-orchestrator

# Or restart entire instance
holt down && holt up
```

---

## Performance Issues

### Slow Startup Time

**Symptoms:**
`holt up` takes > 10 seconds.

**Cause:** Images not cached, slow network, or resource constraints.

**Solution:**
```bash
# Pre-build images
docker build -t example-agent:latest -f agents/example-agent/Dockerfile .

# Pull base images ahead of time
docker pull redis:7-alpine
docker pull golang:1.24-alpine

# Check Docker resources (Docker Desktop)
# Holtings → Resources → increase CPU/Memory
```

---

### Slow Agent Execution

**Symptoms:**
Agent takes > 5 seconds to produce artefact.

**Cause:** LLM API latency, complex processing, or resource constraints.

**Solution:**
```bash
# Check agent logs for timing
holt logs <agent-name>

# Monitor container resources
docker stats holt-{instance}-agent-{agent-name}

# Optimize agent script
# - Cache LLM responses
# - Reduce processing steps
# - Parallelize where possible
```

---

## Debugging Commands

### Essential Commands

```bash
# List running instances
holt list

# View all artefacts
holt hoard

# View agent logs
holt logs <agent-name>

# View orchestrator logs
holt logs orchestrator

# Check Git status
git status

# Check Docker containers
docker ps -a
```

### Advanced Docker Debugging

```bash
# Execute shell in agent container
docker exec -it holt-{instance}-agent-{agent-name} /bin/sh

# Check environment variables
docker exec holt-{instance}-agent-{agent-name} env

# Inspect container configuration
docker inspect holt-{instance}-agent-{agent-name}

# View container resource usage
docker stats --no-stream

# Check Docker networks
docker network ls
docker network inspect holt-{instance}
```

### Redis Debugging

```bash
# Connect to Redis CLI
docker exec -it holt-{instance}-redis redis-cli

# Inside Redis CLI:
# List all keys
KEYS holt:*

# Get artefact
HGETALL holt:{instance}:artefact:{uuid}

# Get claim
HGETALL holt:{instance}:claim:{uuid}

# Get bids
HGETALL holt:{instance}:claim:{uuid}:bids

# Count artefacts
KEYS holt:{instance}:artefact:* | wc -l

# Monitor real-time activity
MONITOR
```

### Git Debugging

```bash
# View commit history
git log --oneline --all --graph

# Find commits by Holt agents
git log --oneline --grep="holt-agent"

# Check current branch and status
git status

# View file at specific commit
git show <commit-hash>:<filename>

# Find which commit created a file
git log --diff-filter=A -- <filename>
```

### Network Debugging

```bash
# Test Redis connectivity from orchestrator
docker exec holt-{instance}-orchestrator ping redis

# Test DNS resolution
docker exec holt-{instance}-agent-{agent-name} nslookup redis

# Check network connectivity
docker exec holt-{instance}-agent-{agent-name} wget -O- http://redis:6379
```

---

## Getting Help

If you've tried the solutions above and still have issues:

1. **Check logs systematically:**
   ```bash
   holt logs orchestrator > orch.log
   holt logs <agent-name> > agent.log
   docker logs holt-{instance}-redis > redis.log
   ```

2. **Gather diagnostic info:**
   ```bash
   holt list
   docker ps -a
   git status
   docker version
   ```

3. **Create minimal reproduction:**
   - Fresh Git repo
   - Minimal holt.yml
   - Simple test agent
   - Document exact steps

4. **Report issue:**
   - GitHub: https://github.com/dyluth/holt/issues
   - Include logs, configuration, and reproduction steps

---

## Quick Reference

| Problem | First Command to Run |
|---------|---------------------|
| Holt won't start | `git status && cat holt.yml` |
| Agent not executing | `holt logs <agent-name>` |
| Missing artefacts | `holt hoard && docker ps` |
| Git errors | `git status && ls -la .git` |
| Container issues | `docker ps -a \| grep holt-` |
| Redis problems | `docker logs holt-{instance}-redis` |
| Permission errors | `ls -la && docker inspect <container>` |
| Performance issues | `docker stats` |
| Max iterations (M3.3) | `holt hoard \| grep Failure` |
| Feedback loop (M3.3) | `holt logs orchestrator \| grep feedback` |
| Version not incrementing (M3.3) | `holt logs <agent> \| grep "rework artefact"` |

---

**Next Steps:**
- [Agent Development Guide](./agent-development.md)
- [Project Context](../PROJECT_CONTEXT.md)
- [System Specification](../design/holt-system-specification.md)
