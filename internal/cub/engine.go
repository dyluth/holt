package cub

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/dyluth/sett/pkg/blackboard"
)

// Engine represents the core execution logic of the agent cub.
// It manages two concurrent goroutines:
//   - Claim Watcher: Monitors for new claims and evaluates bidding opportunities (M2.2+)
//   - Work Executor: Executes granted work and posts results (M2.3+)
//
// The engine coordinates these goroutines via a work queue channel and
// handles graceful shutdown through context cancellation.
type Engine struct {
	config   *Config
	bbClient *blackboard.Client
	wg       sync.WaitGroup
}

// New creates a new agent cub engine with the provided configuration and blackboard client.
// The engine is ready to be started but does not begin execution until Start() is called.
//
// Parameters:
//   - config: Agent cub runtime configuration (instance name, agent name, etc.)
//   - bbClient: Blackboard client for Redis operations
//
// Returns a configured Engine ready to start.
func New(config *Config, bbClient *blackboard.Client) *Engine {
	return &Engine{
		config:   config,
		bbClient: bbClient,
	}
}

// Start launches the agent cub's concurrent goroutines and blocks until context cancellation.
// Creates a work queue channel and starts both the Claim Watcher and Work Executor goroutines.
//
// The method blocks until:
//   - The provided context is cancelled (normal shutdown)
//   - All goroutines complete their shutdown sequence
//
// Graceful shutdown sequence:
//  1. Context is cancelled (typically via SIGTERM signal)
//  2. Both goroutines detect cancellation via select on ctx.Done()
//  3. Goroutines exit their loops and perform cleanup
//  4. Start() returns once all goroutines complete
//
// Returns nil when shutdown completes successfully.
func (e *Engine) Start(ctx context.Context) error {
	log.Printf("[INFO] Agent cub starting for agent='%s' instance='%s'", e.config.AgentName, e.config.InstanceName)

	// Create work queue with buffer size 1
	// Buffer size 1 allows Claim Watcher to post one claim without blocking
	workQueue := make(chan *blackboard.Claim, 1)

	// Launch Claim Watcher goroutine
	e.wg.Add(1)
	go e.claimWatcher(ctx, workQueue)

	// Launch Work Executor goroutine
	e.wg.Add(1)
	go e.workExecutor(ctx, workQueue)

	// Wait for context cancellation
	<-ctx.Done()
	log.Printf("[INFO] Shutdown signal received, initiating graceful shutdown")

	// Close work queue to signal Work Executor that no more work will arrive
	close(workQueue)

	// Wait for all goroutines to complete
	e.wg.Wait()
	log.Printf("[INFO] All goroutines exited, shutdown complete")

	return nil
}

// claimWatcher monitors for new claims and evaluates bidding opportunities.
// In M2.1, this is a placeholder implementation that logs periodically to demonstrate liveness.
// Real claim watching and bidding logic will be implemented in M2.2.
//
// The goroutine runs until the context is cancelled, then exits cleanly.
// Placeholder behavior: Logs status every 30 seconds.
func (e *Engine) claimWatcher(ctx context.Context, workQueue chan *blackboard.Claim) {
	defer e.wg.Done()
	defer log.Printf("[DEBUG] Claim Watcher exited cleanly")

	log.Printf("[DEBUG] Claim Watcher starting")

	// Create ticker for periodic logging (placeholder work)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Log immediately on startup
	log.Printf("[DEBUG] Claim Watcher running (placeholder mode)")

	for {
		select {
		case <-ctx.Done():
			// Context cancelled - shutdown requested
			log.Printf("[DEBUG] Claim Watcher received shutdown signal")
			return

		case <-ticker.C:
			// Periodic heartbeat log (placeholder work in M2.1)
			log.Printf("[DEBUG] Claim Watcher running (placeholder mode)")
		}
	}
}

// workExecutor receives granted claims from the work queue and executes them.
// In M2.1, this is a placeholder implementation that logs periodically to demonstrate liveness.
// Real work execution logic will be implemented in M2.3.
//
// The goroutine runs until:
//   - The context is cancelled (shutdown signal)
//   - The work queue channel is closed (no more work will arrive)
//
// Placeholder behavior: Logs status every 30 seconds.
func (e *Engine) workExecutor(ctx context.Context, workQueue chan *blackboard.Claim) {
	defer e.wg.Done()
	defer log.Printf("[DEBUG] Work Executor exited cleanly")

	log.Printf("[DEBUG] Work Executor starting")

	// Create ticker for periodic logging (placeholder work)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Log immediately on startup
	log.Printf("[DEBUG] Work Executor ready (placeholder mode)")

	for {
		select {
		case <-ctx.Done():
			// Context cancelled - shutdown requested
			log.Printf("[DEBUG] Work Executor received shutdown signal")
			return

		case claim, ok := <-workQueue:
			if !ok {
				// Work queue closed - no more work will arrive
				log.Printf("[DEBUG] Work queue closed, Work Executor shutting down")
				return
			}

			// Placeholder work: In M2.3, this will execute the agent tool
			// For now, just log that we received a claim (won't happen in M2.1)
			log.Printf("[DEBUG] Work Executor received claim: %s (placeholder mode)", claim.ID)

		case <-ticker.C:
			// Periodic heartbeat log (placeholder work in M2.1)
			log.Printf("[DEBUG] Work Executor ready (placeholder mode)")
		}
	}
}
