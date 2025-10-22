# Testing M3.4 Controller-Worker Docker Socket Access

## Prerequisites

Rebuild the orchestrator image with the fixed Dockerfile:

```bash
make docker-orchestrator
```

## Manual Verification

### 1. Check Host Docker Socket

```bash
ls -l /var/run/docker.sock
```

**Expected output**:
- **Linux**: `srw-rw---- 1 root docker 0 ... /var/run/docker.sock` (GID varies: 999, 998, 121)
- **macOS**: `srw-rw---- 1 root wheel 0 ... /var/run/docker.sock` (GID 0)

### 2. Start a Test Instance

```bash
holt up --name docker-test
```

**Look for** in the output:
```
Docker socket GID: 999 (adding to orchestrator container)
```

### 3. Check Orchestrator Logs

```bash
docker logs holt-orchestrator-docker-test
```

**Expected diagnostic output**:
```
Docker socket found: mode=Srw-rw----
Docker socket ownership: uid=0, gid=999
Current process: uid=1000, gid=1000, groups=[1000 999]
✓ Docker client initialized for worker management
```

**Key check**: `groups` should include the socket's GID (e.g., `[1000 999]`)

### 4. Test Worker Launch (E2E Test)

```bash
go test -tags=integration ./cmd/holt/commands -run TestE2E_M3_4_BasicControllerWorkerFlow -v
```

**Expected**: Test should pass with worker successfully launched.

## Troubleshooting

### Issue: "permission denied" on Docker socket

**Symptoms**:
```
Warning: Docker not accessible (worker management disabled): permission denied
Current process: uid=1000, gid=1000, groups=[1000]
```

**Diagnosis**: GroupAdd not working (GID not in supplementary groups)

**Solutions**:

1. **Verify `holt up` detected the socket**:
   ```bash
   # Rebuild orchestrator
   make docker-orchestrator

   # Try starting instance again
   holt down --name docker-test
   holt up --name docker-test
   ```

2. **macOS Docker Desktop specific**:
   - Ensure Docker Desktop is running
   - Try restarting Docker Desktop
   - Check Docker Desktop settings → Resources → File Sharing

3. **Linux specific**:
   - Verify current user is in docker group:
     ```bash
     groups | grep docker
     ```
   - If not, add yourself:
     ```bash
     sudo usermod -aG docker $USER
     # Log out and back in
     ```

4. **GitHub Actions specific**:
   - Ensure `docker` service is available in workflow
   - Check runner has Docker socket mounted

### Issue: Socket GID is 0 but still fails

**macOS-specific**: Docker Desktop sometimes requires the container to run with specific permissions.

**Workaround**: If GroupAdd doesn't work on macOS, try setting the socket permissions:
```bash
# NOT RECOMMENDED, but for testing:
sudo chmod 666 /var/run/docker.sock
```

Then restart the instance.

## Platform-Specific Notes

### Linux
- Docker socket typically owned by `root:docker` (GID 999, 998, or 121)
- GroupAdd should work reliably
- User must be in docker group on host

### macOS Docker Desktop
- Socket owned by `root:wheel` (GID 0)
- Docker Desktop's VM handles permissions
- GroupAdd with GID 0 should work, but behavior depends on Docker Desktop version

### GitHub Actions
- Socket varies by runner (ubuntu-latest, etc.)
- Typically GID 121 or similar
- GroupAdd should work if docker service is running

## Success Criteria

✓ Orchestrator logs show: `✓ Docker client initialized for worker management`
✓ Process groups include socket GID: `groups=[1000 XXX]`
✓ E2E test `TestE2E_M3_4_BasicControllerWorkerFlow` passes
✓ Workers can be launched via controller agents
