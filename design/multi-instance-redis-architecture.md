# **Multi-Instance Redis Architecture**

**Purpose**: Instance isolation, naming, and uniqueness strategy for shared Redis
**Scope**: Foundation - applies to all Holt instances
**Estimated tokens**: ~1,200 tokens
**Read when**: Implementing instance lifecycle, orchestrator, or understanding isolation

---

## **Problem Statement**

Multiple Holt instances must be able to coexist safely on a single shared Redis server without interfering with each other. This requires:

1. **Complete data isolation** - No instance can access another instance's artefacts, claims, or bids
2. **Event isolation** - Pub/Sub events must only reach the intended instance
3. **Uniqueness enforcement** - No two instances can use the same name simultaneously
4. **Automatic cleanup** - Crashed instances should not leave orphaned locks
5. **Simple UX** - `holt up` without arguments should "just work"

---

## **Solution Architecture**

### **1. Namespacing Strategy**

**All Redis keys and channels are namespaced by instance name:**

```
# Global Keys (not instance-specific)
holt:instance_counter                      # Atomic counter for auto-naming
holt:instances                             # Hash of active instance metadata

# Instance-Specific Keys
holt:{instance_name}:artefact:{uuid}       # Artefact data
holt:{instance_name}:claim:{uuid}          # Claim data
holt:{instance_name}:claim:{uuid}:bids     # Bid data
holt:{instance_name}:thread:{logical_id}   # Version tracking
holt:{instance_name}:lock                  # Instance lock (TTL-based)

# Instance-Specific Pub/Sub Channels
holt:{instance_name}:artefact_events       # Artefact creation events
holt:{instance_name}:claim_events          # Claim creation events
```

**Benefits:**
- Complete isolation at the Redis level
- Simple pattern matching for debugging (`KEYS holt:myproject:*`)
- No cross-instance event delivery

**Implementation:**
- All key and channel generation functions take `instanceName` as first parameter
- Defined in `pkg/blackboard/schema.go`

---

### **2. Instance Lock Mechanism**

**Purpose**: Prevent name collisions and provide instance health tracking

**Redis Key:**
```
Key: holt:{instance_name}:lock
Type: String
Value: timestamp or orchestrator metadata (JSON)
TTL: 60 seconds
```

**Lifecycle:**

1. **On `holt up --name myproject`:**
   - Try to create lock: `SET holt:myproject:lock <value> NX EX 60`
   - If fails (key exists), instance name is already in use → error with helpful message
   - If succeeds, proceed with startup

2. **During operation (Orchestrator heartbeat):**
   - Refresh lock every 30 seconds: `SET holt:myproject:lock <value> EX 60`
   - If heartbeat fails, lock will expire in 60 seconds (auto-cleanup)

3. **On `holt down --name myproject`:**
   - Graceful shutdown: `DEL holt:myproject:lock`
   - If crash, TTL expires naturally (no orphaned locks)

**Error Handling:**
```
# Lock already exists
$ holt up --name myproject
Error: Instance name 'myproject' is already in use
Try: holt list (to see active instances)
Try: holt down --name myproject (to stop existing instance)
Try: holt up --name myproject-2 (to use a different name)
```

---

### **3. Instance Metadata Hash**

**Purpose**: Track active instances and their workspace paths to prevent conflicts

**Redis Key:**
```
Key: holt:instances
Type: Hash
Fields: {instance_name} -> JSON metadata
No field-level TTL (cleanup on holt down)
```

**Metadata Schema (JSON):**
```json
{
  "run_id": "550e8400-e29b-41d4-a716-446655440000",
  "workspace_path": "/absolute/path/to/project",
  "started_at": "2025-10-05T12:34:56Z",
  "orchestrator_pid": 12345
}
```

**Field Definitions:**
- `run_id` (UUID): Unique identifier for this instance run. Changes on every `holt up` (even with same instance name)
- `workspace_path` (string): Absolute path to the workspace directory where `holt up` was executed
- `started_at` (ISO8601): Timestamp when the instance was started
- `orchestrator_pid` (int): Process ID of the orchestrator (for debugging)

**Lifecycle:**

1. **On `holt up --name myproject`:**
   ```redis
   HSET holt:instances myproject '{"run_id":"...", "workspace_path":"...", ...}'
   ```

2. **During operation:**
   - Metadata persists (no TTL on individual hash fields)
   - Used for workspace path collision detection

3. **On `holt down --name myproject`:**
   ```redis
   HDEL holt:instances myproject
   ```

**Workspace Path Collision Detection:**

```go
func CheckWorkspaceCollision(workspacePath string, excludeInstance string) (string, error) {
    // Get all instance metadata
    instances := redisClient.HGetAll(ctx, "holt:instances").Val()

    for instanceName, metadataJSON := range instances {
        if instanceName == excludeInstance {
            continue // Skip self when checking
        }

        var metadata InstanceMetadata
        json.Unmarshal([]byte(metadataJSON), &metadata)

        if metadata.WorkspacePath == workspacePath {
            return instanceName, fmt.Errorf(
                "workspace '%s' is already in use by instance '%s'",
                workspacePath, instanceName,
            )
        }
    }

    return "", nil // No collision
}
```

**Error Handling:**
```bash
# Workspace collision without --force
$ cd /path/to/project && holt up
Error: workspace '/path/to/project' is already in use by instance 'default-1'
Use --force to override this check, or run 'holt down --name default-1' first

# Workspace collision with --force (bypasses check)
$ cd /path/to/project && holt up --force
Warning: Overriding workspace path collision check
Started instance: default-2
```

---

### **4. Instance Name Resolution**

**Explicit naming:**
```bash
$ holt up --name myproject
# Uses "myproject" if available, errors if locked
```

**Auto-increment default naming:**
```bash
$ holt up
# Algorithm:
# 1. INCR holt:instance_counter (atomic)
# 2. Use "default-{counter}" as instance name
# 3. Create lock for that name
```

**Redis Counter Key:**
```
Key: holt:instance_counter
Type: Integer
Purpose: Global atomic counter for instance numbering
No TTL: Persists across instance lifecycles
```

**Implementation (pseudocode):**
```go
func ResolveInstanceName(explicitName string) (string, error) {
    if explicitName != "" {
        // User specified name - try to use it
        if IsInstanceLocked(explicitName) {
            return "", fmt.Errorf("instance name '%s' is already in use", explicitName)
        }
        return explicitName, nil
    }

    // No name specified - atomically increment global counter
    counter, err := redisClient.Incr(ctx, "holt:instance_counter").Result()
    if err != nil {
        return "", fmt.Errorf("failed to generate instance name: %w", err)
    }

    // Use counter value for guaranteed-unique name
    instanceName := fmt.Sprintf("default-%d", counter)

    return instanceName, nil
}

func IsInstanceLocked(instanceName string) bool {
    lockKey := InstanceLockKey(instanceName)
    // Returns true if key exists (instance is running)
    exists := redisClient.Exists(ctx, lockKey).Val() > 0
    return exists
}
```

**UX Examples:**
```bash
# First instance - counter starts at 1
$ holt up
Started instance: default-1

# Second instance - counter increments to 2
$ holt up
Started instance: default-2

# Third instance - counter increments to 3
$ holt up
Started instance: default-3

# Stop first instance
$ holt down --name default-1

# Fourth instance - counter increments to 4 (never reuses old numbers)
$ holt up
Started instance: default-4

# Explicit naming - doesn't affect counter
$ holt up --name myproject
Started instance: myproject
```

---

### **5. Instance Discovery**

**`holt list` implementation:**

```bash
$ holt list
Active Holt instances:
  default      (started 5m ago)
  myproject    (started 1h ago)
  default-1    (started 2m ago)
```

**Implementation:**
```go
func ListActiveInstances() ([]InstanceInfo, error) {
    // Scan for all lock keys
    pattern := "holt:*:lock"
    lockKeys := redisClient.Keys(ctx, pattern).Val()

    instances := []InstanceInfo{}
    for _, lockKey := range lockKeys {
        // Extract instance name from key
        // "holt:myproject:lock" -> "myproject"
        instanceName := ExtractInstanceNameFromLockKey(lockKey)

        // Get lock value (contains metadata)
        lockValue := redisClient.Get(ctx, lockKey).Val()

        instances = append(instances, InstanceInfo{
            Name: instanceName,
            Metadata: lockValue,
        })
    }

    return instances, nil
}
```

---

### **6. Instance Name Validation**

**Rules:**
- Must be lowercase alphanumeric plus hyphens
- Must start with a letter
- Length: 1-63 characters
- Pattern: `^[a-z][a-z0-9-]*$`

**Rationale:**
- Compatible with DNS naming (for future k8s support)
- Safe in Redis keys (no special characters)
- Human-readable and memorable

**Implementation:**
```go
func ValidateInstanceName(name string) error {
    if len(name) == 0 || len(name) > 63 {
        return errors.New("instance name must be 1-63 characters")
    }

    matched := regexp.MustCompile(`^[a-z][a-z0-9-]*$`).MatchString(name)
    if !matched {
        return errors.New("instance name must start with a letter and contain only lowercase letters, numbers, and hyphens")
    }

    return nil
}
```

---

## **Implementation Roadmap**

### **M1.1: Redis Blackboard Foundation**
- Define core types (Artefact, Claim, Bid)
- Define instance-specific key helpers (ArtefactKey, ClaimKey, etc.)
- Define namespaced Pub/Sub channel helpers
- Add instance name parameter to all key/channel functions
- **NO global key logic** (moved to M1.4)

### **M1.2: Blackboard Client Operations**
- Redis connection management and CRUD operations
- Pub/Sub subscription uses namespaced channels
- Thread tracking ZSET operations

### **M1.4: CLI Lifecycle Management**
- Define global key constants (`holt:instance_counter`, `holt:instances`)
- Define instance-specific key helper (`holt:{name}:lock`)
- Implement `ResolveInstanceName()` algorithm (atomic counter)
- Implement `ValidateInstanceName()` validation
- Implement `CheckWorkspaceCollision()` function
- `holt up`:
  - Atomically increment counter for default naming
  - Check workspace path collision (unless `--force`)
  - Create lock on startup
  - Register instance metadata in `holt:instances` hash
- `holt down`:
  - Delete lock on shutdown
  - Remove instance from `holt:instances` hash
- `holt list`: Read `holt:instances` hash and display metadata

### **M1.5: Orchestrator Claim Engine**
- Heartbeat goroutine refreshes lock every 30s
- Graceful shutdown deletes lock and instance metadata
- Subscribe to namespaced Pub/Sub channels
- Publish to namespaced Pub/Sub channels

---

## **Testing Strategy**

### **Unit Tests**
- `ValidateInstanceName()` with valid and invalid names
- `ResolveInstanceName()` with various scenarios
- Lock key generation and parsing

### **Integration Tests**
- Multiple `holt up` commands (test auto-increment)
- `holt up` collision detection (name already in use)
- Workspace path collision detection (exact match)
- `--force` flag bypasses workspace collision
- Lock TTL expiration (crash simulation)
- `holt list` discovers all instances with metadata
- Instance metadata properly stored and removed
- Namespaced Pub/Sub (no cross-instance events)

### **E2E Tests**
```bash
# Test 1: Auto-increment naming
holt up              # Gets "default-1"
holt up              # Gets "default-2"
holt list            # Shows both with metadata
holt down            # Stops "default-2"
holt down --name default-1

# Test 2: Explicit naming
holt up --name test1
holt up --name test1 # Fails (already in use)
holt down --name test1

# Test 3: Workspace path collision
cd /path/to/project
holt up              # Gets "default-3", workspace recorded
cd /path/to/project
holt up              # Fails (workspace already in use)
holt up --force      # Succeeds (bypasses workspace check), gets "default-4"
holt down --name default-3
holt down --name default-4

# Test 4: Crash recovery
holt up --name crash-test
kill -9 <orchestrator-pid>
# Wait 65 seconds (TTL expiration)
holt up --name crash-test # Should succeed (lock expired)

# Test 5: Instance metadata
holt up --name meta-test
redis-cli HGET holt:instances meta-test
# Should show JSON with workspace_path, run_id, etc.
holt down --name meta-test
redis-cli HGET holt:instances meta-test
# Should return nil (metadata removed)
```

---

## **Security Considerations**

### **Lock Hijacking Prevention**
**Risk**: Attacker creates lock key manually to DoS instance creation

**Mitigation**:
- Lock value includes orchestrator metadata (PID, start time)
- `holt down` validates lock ownership before deletion (future enhancement)
- TTL ensures locks don't persist indefinitely

### **Instance Name Squatting**
**Risk**: Malicious actor creates many locks to prevent legitimate instances

**Mitigation**:
- Redis should be on a trusted network (not internet-facing)
- Future: Add authentication/authorization to Redis
- Monitor for excessive lock creation

### **Resource Exhaustion**
**Risk**: Too many instances overwhelm Redis

**Mitigation**:
- `ResolveInstanceName()` has upper limit (100 auto-increment attempts)
- Redis maxmemory policy should be configured
- Monitor Redis memory usage

---

## **Future Enhancements**

### **V2: Lock Ownership Verification**
```go
type LockValue struct {
    OrchestratorPID int       `json:"pid"`
    StartTime       time.Time `json:"start_time"`
    Hostname        string    `json:"hostname"`
}

// holt down validates lock ownership before deletion
```

### **V2: Quorum-Based Locking**
- Use RedLock algorithm for distributed lock safety
- Relevant when Redis is clustered or replicated

### **V2: Instance Metadata**
- Store additional info in lock value
- Expose in `holt list` (uptime, artefact count, agent status)

---

## **Principle Compliance**

✅ **YAGNI**: Uses standard Redis operations (SET NX, TTL), no complex lock library needed
✅ **Zero-config**: `holt up` works without arguments (auto-increment)
✅ **Small, single-purpose**: Lock mechanism is simple and focused
✅ **Pragmatism**: TTL-based approach is battle-tested and reliable

---

## **Summary**

The multi-instance Redis architecture provides:

1. **Complete isolation** via instance name namespacing
2. **Automatic name resolution** with atomic counter-based auto-increment
3. **Robust uniqueness** via TTL-based lock mechanism
4. **Workspace safety** preventing accidental concurrent operations on the same codebase
5. **Crash resilience** through lock expiration and metadata tracking
6. **Simple UX** that "just works" out of the box with sensible defaults

**Three-layer safety strategy:**
- **Layer 1**: `holt:instance_counter` for guaranteed-unique naming
- **Layer 2**: `holt:{name}:lock` with TTL for liveness and uniqueness
- **Layer 3**: `holt:instances` hash for workspace collision detection

This design enables safe, reliable multi-instance operation on shared Redis infrastructure while maintaining Holt's zero-configuration philosophy.
