# Sett Troubleshooting Guide

**Target Audience:** Developers encountering issues with Sett workflows

**Scope:** Common problems, causes, solutions, and debugging commands

---

## Table of Contents

1. [Sett Won't Start](#sett-wont-start)
2. [Agent Won't Execute](#agent-wont-execute)
3. [Git Workspace Errors](#git-workspace-errors)
4. [Blackboard State Issues](#blackboard-state-issues)
5. [Docker & Container Problems](#docker--container-problems)
6. [Performance Issues](#performance-issues)
7. [Debugging Commands](#debugging-commands)

---

## Sett Won't Start

### Error: "sett.yml not found or invalid"

**Symptoms:**
```
❌ sett.yml not found or invalid
   No configuration file found in the current directory.
```

**Cause:** No `sett.yml` file in current directory, or file has syntax errors.

**Solution:**
```bash
# Initialize new project
sett init

# Or verify sett.yml exists
ls -la sett.yml

# Check YAML syntax
cat sett.yml
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
sett up --force
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
sett logs redis

# Restart Sett instance
sett down
sett up

# Check for port conflicts
netstat -an | grep 6379
```

**Debug Commands:**
```bash
# List all containers for this instance
docker ps -a --filter "name=sett-"

# Inspect Redis container
docker inspect sett-{instance}-redis

# Test Redis connectivity
docker exec sett-{instance}-redis redis-cli PING
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
sett down --name default-1

# Or use different name
sett up --name my-instance

# Or list and clean up
sett list
sett down --name <old-instance>
```

---

### Error: "workspace already in use by instance X"

**Symptoms:**
```
❌ workspace already in use by instance default-1
   Another Sett instance is running in this directory.
```

**Cause:** Another Sett instance is already running in this Git repository.

**Solution:**
```bash
# Check running instances
sett list

# Stop the instance using this workspace
sett down

# Or run in different directory
cd ../other-project && sett up
```

---

## Agent Won't Execute

### Agent Container Not Starting

**Symptoms:**
- `docker ps` doesn't show agent container
- `sett logs <agent>` shows "container not found"

**Cause:** Docker image not built, configuration error in sett.yml, or Docker daemon issue.

**Solution:**
```bash
# Verify image exists
docker images | grep <agent-name>

# Build agent image
docker build -t <agent-name>:latest -f agents/<agent-name>/Dockerfile .

# Check sett.yml configuration
cat sett.yml | grep -A 5 agents:

# Restart instance
sett down && sett up
```

**Debug Commands:**
```bash
# Check Docker daemon status
docker info

# View agent container status (including stopped)
docker ps -a --filter "name=agent"

# Inspect agent container
docker inspect sett-{instance}-agent-{agent-name}

# Check container logs
docker logs sett-{instance}-agent-{agent-name}
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
sett logs <agent-name>

# Look for lines like:
# "Received claim event"
# "Submitting bid: exclusive"
# "Executing work for claim"

# Verify agent container is healthy
docker exec sett-{instance}-agent-{agent-name} wget -O- http://localhost:8080/healthz
```

**Debug Commands:**
```bash
# Check blackboard for claims
sett hoard

# Query Redis directly for bids
docker exec sett-{instance}-redis redis-cli HGETALL sett:{instance}:claim:{claim-id}:bids

# Check orchestrator logs
sett logs orchestrator
```

---

### Agent Executes But Creates Failure Artefact

**Symptoms:**
```bash
sett hoard
# Shows Failure artefact instead of expected result
```

**Cause:** Agent tool script error, invalid output JSON, or git commit validation failed.

**Solution:**
```bash
# Check agent logs for stderr output
sett logs <agent-name>

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
sett hoard | grep -A 10 "Failure"

# Check artefact payload for error details
docker exec sett-{instance}-redis redis-cli HGET sett:{instance}:artefact:{id} payload
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
docker inspect sett-{instance}-agent-{agent-name} | grep -A 10 Mounts
```

**Debug Commands:**
```bash
# Check git history in workspace
git log --oneline -20

# Verify workspace is mounted correctly
docker exec sett-{instance}-agent-{agent-name} ls -la /workspace

# Check git config in container
docker exec sett-{instance}-agent-{agent-name} git config --list
```

---

## Git Workspace Errors

### Error: "not a Git repository"

**Symptoms:**
```
❌ not a Git repository
   Sett requires a Git repository to manage workflows.
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

# Then initialize Sett
sett init
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
# Verify workspace mode in sett.yml
cat sett.yml
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
sett down && sett up
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
sett list

# Check git log for agent commits
git log --oneline --author="Sett"

# Verify mounts
docker inspect sett-{instance}-agent-{agent-name} | grep -A 10 "Mounts"

# Restart with clean state
sett down
git status  # Should be clean
sett up
```

---

## Blackboard State Issues

### Artefacts Not Appearing

**Symptoms:**
```bash
sett hoard
# Shows empty or unexpected results
```

**Cause:** Redis data cleared, wrong instance name, or forage command failed.

**Solution:**
```bash
# Verify instance name
sett list

# Check for specific instance
sett hoard --name <instance-name>

# Verify Redis contains data
docker exec sett-{instance}-redis redis-cli KEYS "sett:*"

# Check orchestrator logs
sett logs orchestrator
```

**Debug Commands:**
```bash
# List all artefacts in Redis
docker exec sett-{instance}-redis redis-cli KEYS "sett:{instance}:artefact:*"

# Get specific artefact
docker exec sett-{instance}-redis redis-cli HGETALL "sett:{instance}:artefact:{uuid}"

# Count artefacts
docker exec sett-{instance}-redis redis-cli KEYS "sett:{instance}:artefact:*" | wc -l
```

---

### Claims Stuck in "pending" State

**Symptoms:**
Claim never progresses from `pending_exclusive` to `complete`.

**Cause:** Agent not bidding, agent crashed, or orchestrator stalled.

**Solution:**
```bash
# Check claim status
docker exec sett-{instance}-redis redis-cli HGET sett:{instance}:claim:{uuid} status

# Check if bids were submitted
docker exec sett-{instance}-redis redis-cli HGETALL sett:{instance}:claim:{uuid}:bids

# Verify orchestrator is running
sett logs orchestrator

# Verify agent is running
sett logs <agent-name>

# Restart if needed
sett down && sett up
```

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

**Cause:** Another service using Redis default port or multiple Sett instances.

**Solution:**
```bash
# Find what's using the port
lsof -i :6379

# Stop conflicting service
# Or let Sett auto-assign different port (it does this automatically)

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

# Remove old Sett containers
docker ps -a | grep sett-
docker rm $(docker ps -a -q --filter "name=sett-")
```

---

### Container Health Check Failures

**Symptoms:**
```
Container sett-{instance}-orchestrator is unhealthy
```

**Cause:** Redis connection lost, application crash, or startup timeout.

**Solution:**
```bash
# Check container logs
docker logs sett-{instance}-orchestrator

# Check health endpoint
docker exec sett-{instance}-orchestrator wget -O- http://localhost:8080/healthz

# Restart container
docker restart sett-{instance}-orchestrator

# Or restart entire instance
sett down && sett up
```

---

## Performance Issues

### Slow Startup Time

**Symptoms:**
`sett up` takes > 10 seconds.

**Cause:** Images not cached, slow network, or resource constraints.

**Solution:**
```bash
# Pre-build images
docker build -t example-agent:latest -f agents/example-agent/Dockerfile .

# Pull base images ahead of time
docker pull redis:7-alpine
docker pull golang:1.24-alpine

# Check Docker resources (Docker Desktop)
# Settings → Resources → increase CPU/Memory
```

---

### Slow Agent Execution

**Symptoms:**
Agent takes > 5 seconds to produce artefact.

**Cause:** LLM API latency, complex processing, or resource constraints.

**Solution:**
```bash
# Check agent logs for timing
sett logs <agent-name>

# Monitor container resources
docker stats sett-{instance}-agent-{agent-name}

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
sett list

# View all artefacts
sett hoard

# View agent logs
sett logs <agent-name>

# View orchestrator logs
sett logs orchestrator

# Check Git status
git status

# Check Docker containers
docker ps -a
```

### Advanced Docker Debugging

```bash
# Execute shell in agent container
docker exec -it sett-{instance}-agent-{agent-name} /bin/sh

# Check environment variables
docker exec sett-{instance}-agent-{agent-name} env

# Inspect container configuration
docker inspect sett-{instance}-agent-{agent-name}

# View container resource usage
docker stats --no-stream

# Check Docker networks
docker network ls
docker network inspect sett-{instance}
```

### Redis Debugging

```bash
# Connect to Redis CLI
docker exec -it sett-{instance}-redis redis-cli

# Inside Redis CLI:
# List all keys
KEYS sett:*

# Get artefact
HGETALL sett:{instance}:artefact:{uuid}

# Get claim
HGETALL sett:{instance}:claim:{uuid}

# Get bids
HGETALL sett:{instance}:claim:{uuid}:bids

# Count artefacts
KEYS sett:{instance}:artefact:* | wc -l

# Monitor real-time activity
MONITOR
```

### Git Debugging

```bash
# View commit history
git log --oneline --all --graph

# Find commits by Sett agents
git log --oneline --grep="sett-agent"

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
docker exec sett-{instance}-orchestrator ping redis

# Test DNS resolution
docker exec sett-{instance}-agent-{agent-name} nslookup redis

# Check network connectivity
docker exec sett-{instance}-agent-{agent-name} wget -O- http://redis:6379
```

---

## Getting Help

If you've tried the solutions above and still have issues:

1. **Check logs systematically:**
   ```bash
   sett logs orchestrator > orch.log
   sett logs <agent-name> > agent.log
   docker logs sett-{instance}-redis > redis.log
   ```

2. **Gather diagnostic info:**
   ```bash
   sett list
   docker ps -a
   git status
   docker version
   ```

3. **Create minimal reproduction:**
   - Fresh Git repo
   - Minimal sett.yml
   - Simple test agent
   - Document exact steps

4. **Report issue:**
   - GitHub: https://github.com/anthropics/sett/issues
   - Include logs, configuration, and reproduction steps

---

## Quick Reference

| Problem | First Command to Run |
|---------|---------------------|
| Sett won't start | `git status && cat sett.yml` |
| Agent not executing | `sett logs <agent-name>` |
| Missing artefacts | `sett hoard && docker ps` |
| Git errors | `git status && ls -la .git` |
| Container issues | `docker ps -a \| grep sett-` |
| Redis problems | `docker logs sett-{instance}-redis` |
| Permission errors | `ls -la && docker inspect <container>` |
| Performance issues | `docker stats` |

---

**Next Steps:**
- [Agent Development Guide](./agent-development.md)
- [Project Context](../PROJECT_CONTEXT.md)
- [System Specification](../design/sett-system-specification.md)
