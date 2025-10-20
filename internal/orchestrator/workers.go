package orchestrator

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/dyluth/sett/internal/config"
	dockerpkg "github.com/dyluth/sett/internal/docker"
	"github.com/dyluth/sett/pkg/blackboard"
	"github.com/google/uuid"
)

// WorkerState tracks an active worker container
// M3.4: Workers are ephemeral containers launched on-demand to execute granted claims
type WorkerState struct {
	ContainerID   string    // Docker container ID
	ContainerName string    // sett-{instance}-{agent}-worker-{claim-short-id}
	ClaimID       string    // Claim being executed
	Role          string    // Agent role
	AgentName     string    // Original agent name (e.g., "coder-controller")
	LaunchedAt    time.Time // When worker was launched
	Status        string    // "created", "running", "exited"
	ExitCode      int       // Container exit code (when exited)
}

// WorkerManager handles worker lifecycle management for the orchestrator
// M3.4: Manages Docker container creation, monitoring, and cleanup for workers
type WorkerManager struct {
	dockerClient      *client.Client
	instanceName      string
	workspacePath     string
	networkName       string
	redisContainerName string

	activeWorkers map[string]*WorkerState // key: container_id
	workersByRole map[string]int          // key: role, value: active worker count
	workerLock    sync.RWMutex
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

// LaunchWorker creates and starts an ephemeral worker container
// M3.4: Workers are launched when a controller wins a grant
func (wm *WorkerManager) LaunchWorker(ctx context.Context, claim *blackboard.Claim, agentName string, agent config.Agent, bbClient *blackboard.Client) error {
	// Generate worker container name
	shortClaimID := claim.ID[:8] // First 8 chars of UUID
	containerName := fmt.Sprintf("sett-%s-%s-worker-%s", wm.instanceName, agentName, shortClaimID)

	wm.logEvent("worker_launching", map[string]interface{}{
		"container_name": containerName,
		"claim_id":       claim.ID,
		"role":           agent.Role,
		"agent_name":     agentName,
	})

	// Build Docker container config
	redisURL := fmt.Sprintf("redis://%s:6379", wm.redisContainerName)

	containerConfig := &container.Config{
		Image: agent.Worker.Image,
		// M3.4: Worker is launched with --execute-claim flag
		// Note: Image has ENTRYPOINT ["/app/cub"], so Cmd only contains arguments
		Cmd: []string{"--execute-claim", claim.ID},
		Env: []string{
			fmt.Sprintf("SETT_INSTANCE_NAME=%s", wm.instanceName),
			fmt.Sprintf("SETT_AGENT_NAME=%s", agentName),
			fmt.Sprintf("SETT_AGENT_ROLE=%s", agent.Role),
			fmt.Sprintf("REDIS_URL=%s", redisURL),
			fmt.Sprintf("SETT_BIDDING_STRATEGY=%s", agent.BiddingStrategy),
			// NOTE: No SETT_MODE for workers - the --execute-claim flag is sufficient
		},
		Labels: dockerpkg.BuildLabels(wm.instanceName, uuid.New().String(), wm.workspacePath, "worker"),
	}

	// Add SETT_AGENT_COMMAND environment variable if configured
	if len(agent.Worker.Command) > 0 {
		commandJSON := fmt.Sprintf("[\"%s\"]", agent.Worker.Command[0])
		if len(agent.Worker.Command) > 1 {
			// Build proper JSON array
			commandJSON = "["
			for i, cmd := range agent.Worker.Command {
				if i > 0 {
					commandJSON += ","
				}
				commandJSON += fmt.Sprintf("\"%s\"", cmd)
			}
			commandJSON += "]"
		}
		containerConfig.Env = append(containerConfig.Env, fmt.Sprintf("SETT_AGENT_COMMAND=%s", commandJSON))
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
	wm.workerLock.Lock()
	workerState := &WorkerState{
		ContainerID:   resp.ID,
		ContainerName: containerName,
		ClaimID:       claim.ID,
		Role:          agent.Role,
		AgentName:     agentName,
		LaunchedAt:    time.Now(),
		Status:        "running",
	}
	wm.activeWorkers[resp.ID] = workerState
	wm.workersByRole[agent.Role]++
	wm.workerLock.Unlock()

	wm.logEvent("worker_launched", map[string]interface{}{
		"container_id":   resp.ID,
		"container_name": containerName,
		"claim_id":       claim.ID,
		"role":           agent.Role,
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

	// Cleanup
	wm.cleanupWorker(ctx, containerID)
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
func (wm *WorkerManager) cleanupWorker(ctx context.Context, containerID string) {
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

	if worker != nil {
		wm.logEvent("worker_cleanup", map[string]interface{}{
			"container_id": containerID,
			"role":         worker.Role,
		})
	}
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
