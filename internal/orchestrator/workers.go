package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/dyluth/holt/internal/config"
	dockerpkg "github.com/dyluth/holt/internal/docker"
	"github.com/dyluth/holt/pkg/blackboard"
	"github.com/google/uuid"
)

// WorkerState tracks an active worker container
// M3.4: Workers are ephemeral containers launched on-demand to execute granted claims
type WorkerState struct {
	ContainerID   string    // Docker container ID
	ContainerName string    // holt-{instance}-{agent}-worker-{claim-short-id}
	ClaimID       string    // Claim being executed
	Role          string    // Agent role
	AgentName     string    // Original agent name (e.g., "coder-controller")
	LaunchedAt    time.Time // When worker was launched
	Status        string    // "created", "running", "exited"
	ExitCode      int       // Container exit code (when exited)
}

// WorkerManager handles worker lifecycle management for the orchestrator
// M3.4: Manages Docker container creation, monitoring, and cleanup for workers
// M3.5: Supports queue resumption callback for grant queue management
type WorkerManager struct {
	dockerClient       *client.Client
	instanceName       string
	workspacePath      string
	networkName        string
	redisContainerName string

	activeWorkers map[string]*WorkerState // key: container_id
	workersByRole map[string]int          // key: role, value: active worker count
	workerLock    sync.RWMutex

	// M3.5: Callback invoked when worker slot opens (for grant queue resumption)
	onWorkerSlotAvailable func(ctx context.Context, role string)
}

// NewWorkerManager creates a new worker manager
func NewWorkerManager(dockerClient *client.Client, instanceName, workspacePath string) *WorkerManager {
	return &WorkerManager{
		dockerClient:       dockerClient,
		instanceName:       instanceName,
		workspacePath:      workspacePath,
		networkName:        dockerpkg.NetworkName(instanceName),
		redisContainerName: dockerpkg.RedisContainerName(instanceName),
		activeWorkers:      make(map[string]*WorkerState),
		workersByRole:      make(map[string]int),
	}
}

// SetWorkerSlotAvailableCallback sets the callback for queue resumption (M3.5).
// Called by orchestrator engine to enable grant queue resumption when workers complete.
func (wm *WorkerManager) SetWorkerSlotAvailableCallback(callback func(ctx context.Context, role string)) {
	wm.onWorkerSlotAvailable = callback
}

// CleanupOrphanedWorkers removes worker containers from previous orchestrator runs (M3.5).
// Identifies orphans by checking for containers with holt labels but not in activeWorkers map.
func (wm *WorkerManager) CleanupOrphanedWorkers(ctx context.Context) error {
	log.Printf("[Orchestrator] Scanning for orphaned worker containers...")

	// List all containers with holt instance label
	containerFilters := filters.NewArgs()
	containerFilters.Add("label", fmt.Sprintf("holt.instance=%s", wm.instanceName))

	containers, err := wm.dockerClient.ContainerList(ctx, types.ContainerListOptions{
		All:     true, // Include stopped containers
		Filters: containerFilters,
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	orphanCount := 0

	for _, container := range containers {
		// Check if container is in activeWorkers
		wm.workerLock.RLock()
		_, isTracked := wm.activeWorkers[container.ID]
		wm.workerLock.RUnlock()

		if !isTracked {
			// Orphaned container - remove it
			log.Printf("[Orchestrator] Removing orphaned worker container: %s (ID: %s)", container.Names, container.ID[:12])

			wm.logEvent("orphan_worker_cleanup", map[string]interface{}{
				"container_id":   container.ID[:12],
				"container_name": container.Names,
			})

			if err := wm.dockerClient.ContainerRemove(ctx, container.ID, types.ContainerRemoveOptions{
				Force: true,
			}); err != nil {
				log.Printf("[Orchestrator] Warning: Failed to remove orphaned container %s: %v", container.ID[:12], err)
				// Continue with other containers
			} else {
				orphanCount++
			}
		}
	}

	if orphanCount > 0 {
		log.Printf("[Orchestrator] Cleaned up %d orphaned worker containers", orphanCount)
	} else {
		log.Printf("[Orchestrator] No orphaned worker containers found")
	}

	return nil
}

// LaunchWorker creates and starts an ephemeral worker container
// M3.4: Workers are launched when a controller wins a grant
// M3.7: agentRole parameter is the agent key from holt.yml (which IS the role)
// M3.9: Resolves worker image ID and stores in claim for audit trail
func (wm *WorkerManager) LaunchWorker(ctx context.Context, claim *blackboard.Claim, agentRole string, agent config.Agent, bbClient *blackboard.Client) error {
	// M3.7: Use centralized WorkerContainerName function
	containerName := dockerpkg.WorkerContainerName(wm.instanceName, agentRole, claim.ID)

	// M3.9: Resolve worker image ID for audit trail
	imageID, err := wm.resolveWorkerImageID(ctx, agent.Worker.Image)
	if err != nil {
		return fmt.Errorf("failed to resolve worker image ID: %w", err)
	}

	// M3.9: Store image ID in claim
	claim.GrantedAgentImageID = imageID
	if err := bbClient.UpdateClaim(ctx, claim); err != nil {
		log.Printf("[Orchestrator] Warning: Failed to update claim with worker image ID: %v", err)
		// Non-fatal - continue with worker launch
	}

	wm.logEvent("worker_launching", map[string]interface{}{
		"container_name": containerName,
		"claim_id":       claim.ID,
		"role":           agentRole,
		"agent_name":     agentRole, // M3.7: Agent name = role
		"image_id":       wm.truncateImageID(imageID), // M3.9
	})

	// Build Docker container config
	redisURL := fmt.Sprintf("redis://%s:6379", wm.redisContainerName)

	containerConfig := &container.Config{
		Image: agent.Worker.Image,
		// M3.4: Worker is launched with --execute-claim flag
		// Note: Image has ENTRYPOINT ["/app/pup"], so Cmd only contains arguments
		Cmd: []string{"--execute-claim", claim.ID},
		// M3.7: ONLY HOLT_AGENT_NAME is set (to the role), HOLT_AGENT_ROLE removed
		Env: []string{
			fmt.Sprintf("HOLT_INSTANCE_NAME=%s", wm.instanceName),
			fmt.Sprintf("HOLT_AGENT_NAME=%s", agentRole),
			fmt.Sprintf("REDIS_URL=%s", redisURL),
			fmt.Sprintf("HOLT_BIDDING_STRATEGY=%s", agent.BiddingStrategy),
			// NOTE: No HOLT_MODE for workers - the --execute-claim flag is sufficient
		},
		Labels: dockerpkg.BuildLabels(wm.instanceName, uuid.New().String(), wm.workspacePath, "worker"),
	}

	// Add HOLT_AGENT_COMMAND environment variable if configured
	if len(agent.Worker.Command) > 0 {
		commandJSON, err := json.Marshal(agent.Worker.Command)
		if err != nil {
			return fmt.Errorf("failed to marshal worker command to JSON: %w", err)
		}
		containerConfig.Env = append(containerConfig.Env, fmt.Sprintf("HOLT_AGENT_COMMAND=%s", commandJSON))
	}

	// Add HOLT_AGENT_BID_SCRIPT as JSON array
	if len(agent.BidScript) > 0 {
		bidScriptJSON, err := json.Marshal(agent.BidScript)
		if err != nil {
			return fmt.Errorf("failed to marshal agent bid script to JSON: %w", err)
		}
		containerConfig.Env = append(containerConfig.Env, fmt.Sprintf("HOLT_AGENT_BID_SCRIPT=%s", bidScriptJSON))
	}

	// Build host config
	hostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode(wm.networkName),
		AutoRemove:  false, // We manage cleanup explicitly for better tracking
	}

	// Add workspace mount if configured
	if agent.Worker.Workspace != nil && agent.Worker.Workspace.Mode != "" {
		mountType := mount.Mount{
			Type:     mount.TypeBind,
			Source:   wm.workspacePath,
			Target:   "/workspace",
			ReadOnly: (agent.Worker.Workspace.Mode == "ro"),
		}
		hostConfig.Mounts = []mount.Mount{mountType}
	}

	// Create container
	resp, err := wm.dockerClient.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, containerName)
	if err != nil {
		return fmt.Errorf("failed to create worker container: %w", err)
	}

	// Start container
	if err := wm.dockerClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		// Cleanup on start failure
		wm.dockerClient.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{Force: true})
		return fmt.Errorf("failed to start worker container: %w", err)
	}

	// Track worker state
	// M3.7: agentRole = agentName (both are the role)
	wm.workerLock.Lock()
	workerState := &WorkerState{
		ContainerID:   resp.ID,
		ContainerName: containerName,
		ClaimID:       claim.ID,
		Role:          agentRole,
		AgentName:     agentRole,
		LaunchedAt:    time.Now(),
		Status:        "running",
	}
	wm.activeWorkers[resp.ID] = workerState
	wm.workersByRole[agentRole]++
	wm.workerLock.Unlock()

	wm.logEvent("worker_launched", map[string]interface{}{
		"container_id":   resp.ID,
		"container_name": containerName,
		"claim_id":       claim.ID,
		"role":           agentRole,
	})

	// Start monitoring worker in background
	go wm.monitorWorker(ctx, resp.ID, bbClient)

	return nil
}

// monitorWorker watches a worker container and handles completion/failure
// M3.4: Monitors worker exit and creates Failure artefacts on non-zero exit codes
func (wm *WorkerManager) monitorWorker(ctx context.Context, containerID string, bbClient *blackboard.Client) {
	wm.workerLock.RLock()
	worker := wm.activeWorkers[containerID]
	wm.workerLock.RUnlock()

	if worker == nil {
		log.Printf("[Orchestrator] Worker %s not found in tracking state", containerID)
		return
	}

	// Wait for container to exit
	statusCh, errCh := wm.dockerClient.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)

	select {
	case err := <-errCh:
		log.Printf("[Orchestrator] Error waiting for worker %s: %v", containerID, err)
		wm.handleWorkerError(ctx, worker, err, bbClient)

	case status := <-statusCh:
		log.Printf("[Orchestrator] Worker %s exited with code %d", containerID, status.StatusCode)
		wm.handleWorkerExit(ctx, worker, int(status.StatusCode), bbClient)
	}

	// M3.5: Cleanup and trigger queue resumption
	role := wm.cleanupWorker(ctx, containerID)

	// M3.5: Notify orchestrator that a worker slot is available for this role
	if role != "" && wm.onWorkerSlotAvailable != nil {
		wm.onWorkerSlotAvailable(ctx, role)
	}
}

// handleWorkerExit processes worker completion or failure
// M3.4: Creates Failure artefact on non-zero exit code
func (wm *WorkerManager) handleWorkerExit(ctx context.Context, worker *WorkerState, exitCode int, bbClient *blackboard.Client) {
	if exitCode != 0 {
		// Worker failed - create Failure artefact
		wm.logEvent("worker_failed", map[string]interface{}{
			"container_id": worker.ContainerID,
			"claim_id":     worker.ClaimID,
			"exit_code":    exitCode,
		})

		// Get container logs for failure details
		logs := wm.getWorkerLogs(ctx, worker.ContainerID)

		// Create Failure artefact
		failurePayload := fmt.Sprintf("Worker container exited with code %d\n\nLogs:\n%s", exitCode, logs)
		failure := &blackboard.Artefact{
			ID:              uuid.New().String(),
			LogicalID:       uuid.New().String(),
			Version:         1,
			StructuralType:  blackboard.StructuralTypeFailure,
			Type:            "WorkerFailure",
			Payload:         failurePayload,
			SourceArtefacts: []string{},
			ProducedByRole:  worker.Role,
		}

		if err := bbClient.CreateArtefact(ctx, failure); err != nil {
			log.Printf("[Orchestrator] Failed to create Failure artefact: %v", err)
		}

		// Terminate claim
		claim, err := bbClient.GetClaim(ctx, worker.ClaimID)
		if err == nil {
			claim.Status = blackboard.ClaimStatusTerminated
			claim.TerminationReason = fmt.Sprintf("Worker failed with exit code %d", exitCode)
			bbClient.UpdateClaim(ctx, claim)
		}
	} else {
		// Worker succeeded
		wm.logEvent("worker_completed", map[string]interface{}{
			"container_id": worker.ContainerID,
			"claim_id":     worker.ClaimID,
		})
	}
}

// handleWorkerError handles Docker API errors while waiting for worker
func (wm *WorkerManager) handleWorkerError(ctx context.Context, worker *WorkerState, err error, bbClient *blackboard.Client) {
	wm.logEvent("worker_error", map[string]interface{}{
		"container_id": worker.ContainerID,
		"claim_id":     worker.ClaimID,
		"error":        err.Error(),
	})

	// Create Failure artefact
	failurePayload := fmt.Sprintf("Worker container monitoring error: %v", err)
	failure := &blackboard.Artefact{
		ID:              uuid.New().String(),
		LogicalID:       uuid.New().String(),
		Version:         1,
		StructuralType:  blackboard.StructuralTypeFailure,
		Type:            "WorkerError",
		Payload:         failurePayload,
		SourceArtefacts: []string{},
		ProducedByRole:  worker.Role,
	}

	if createErr := bbClient.CreateArtefact(ctx, failure); createErr != nil {
		log.Printf("[Orchestrator] Failed to create Failure artefact: %v", createErr)
	}

	// Terminate claim
	claim, getErr := bbClient.GetClaim(ctx, worker.ClaimID)
	if getErr == nil {
		claim.Status = blackboard.ClaimStatusTerminated
		claim.TerminationReason = fmt.Sprintf("Worker monitoring error: %v", err)
		bbClient.UpdateClaim(ctx, claim)
	}
}

// cleanupWorker removes worker from tracking and Docker
// M3.4: Decrements worker count and removes container
// M3.5: Returns the worker's role so orchestrator can resume queued claims
func (wm *WorkerManager) cleanupWorker(ctx context.Context, containerID string) string {
	wm.workerLock.Lock()
	worker := wm.activeWorkers[containerID]
	if worker != nil {
		delete(wm.activeWorkers, containerID)
		wm.workersByRole[worker.Role]--
	}
	wm.workerLock.Unlock()

	// Brief delay before container removal to allow external observers (like E2E tests)
	// to detect the exited state before cleanup
	time.Sleep(2 * time.Second)

	// Remove container
	wm.dockerClient.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{
		Force: true,
	})

	var role string
	if worker != nil {
		role = worker.Role
		wm.logEvent("worker_cleanup", map[string]interface{}{
			"container_id": containerID,
			"role":         role,
		})
	}

	return role
}

// getWorkerLogs retrieves container logs for failure debugging
// M3.4: Returns last 100 lines of worker logs
func (wm *WorkerManager) getWorkerLogs(ctx context.Context, containerID string) string {
	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       "100", // Last 100 lines
	}

	reader, err := wm.dockerClient.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return fmt.Sprintf("(failed to retrieve logs: %v)", err)
	}
	defer reader.Close()

	logs, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Sprintf("(failed to read logs: %v)", err)
	}

	return string(logs)
}

// IsAtWorkerLimit checks if role has reached max concurrent workers
// M3.4: Used by grant decision logic to pause granting
func (wm *WorkerManager) IsAtWorkerLimit(role string, maxConcurrent int) bool {
	wm.workerLock.RLock()
	defer wm.workerLock.RUnlock()

	activeCount := wm.workersByRole[role]
	return activeCount >= maxConcurrent
}

// logEvent logs structured orchestrator events
func (wm *WorkerManager) logEvent(event string, data map[string]interface{}) {
	log.Printf("[Orchestrator] event=%s %v", event, data)
}

// resolveWorkerImageID resolves a worker image tag to its Docker image digest (M3.9).
// Returns full sha256:... digest if available, falls back to image ID.
func (wm *WorkerManager) resolveWorkerImageID(ctx context.Context, imageTag string) (string, error) {
	imageInfo, _, err := wm.dockerClient.ImageInspectWithRaw(ctx, imageTag)
	if err != nil {
		return "", fmt.Errorf("failed to inspect worker image: %w", err)
	}

	// Prefer RepoDigests (contains registry path + sha256)
	if len(imageInfo.RepoDigests) > 0 {
		return imageInfo.RepoDigests[0], nil
	}

	// Fallback to image ID (local builds without registry)
	if imageInfo.ID != "" {
		return imageInfo.ID, nil
	}

	return "", fmt.Errorf("worker image has no digest or ID")
}

// truncateImageID shortens an image ID/digest for display (M3.9).
func (wm *WorkerManager) truncateImageID(imageID string) string {
	if len(imageID) > 7 && imageID[:7] == "sha256:" {
		hash := imageID[7:]
		if len(hash) >= 12 {
			return hash[:12]
		}
		return hash
	}
	if len(imageID) >= 12 {
		return imageID[:12]
	}
	return imageID
}
