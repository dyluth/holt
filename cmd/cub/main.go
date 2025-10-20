package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dyluth/sett/internal/cub"
	"github.com/dyluth/sett/pkg/blackboard"
	"github.com/redis/go-redis/v9"
)

func main() {
	// Exit with appropriate code
	os.Exit(run())
}

// run contains the main logic and returns an exit code.
// This separation makes the logic testable and ensures deferred functions run.
func run() int {
	// M3.4: Parse command-line flags
	executeClaimID := flag.String("execute-claim", "", "Execute specific claim ID and exit (worker mode)")
	flag.Parse()

	// Load configuration from environment variables
	config, err := cub.LoadConfig()
	if err != nil {
		log.Printf("[ERROR] Configuration error: %v", err)
		return 1
	}

	// M3.4: Mode decision tree
	// 1. If SETT_MODE=controller → controller mode (bidder-only)
	// 2. Else if --execute-claim <id> → worker mode (execute-only)
	// 3. Else → traditional mode (standard agent)
	settMode := os.Getenv("SETT_MODE")

	if settMode == "controller" {
		log.Printf("[INFO] Agent cub starting in CONTROLLER mode (bidder-only) for agent='%s' instance='%s'", config.AgentName, config.InstanceName)
		return runControllerMode(config)
	} else if *executeClaimID != "" {
		log.Printf("[INFO] Agent cub starting in WORKER mode (execute-only) for claim='%s' agent='%s' instance='%s'", *executeClaimID, config.AgentName, config.InstanceName)
		return runWorkerMode(config, *executeClaimID)
	} else {
		log.Printf("[INFO] Agent cub starting in TRADITIONAL mode for agent='%s' instance='%s'", config.AgentName, config.InstanceName)
		return runTraditionalMode(config)
	}
}

// runTraditionalMode runs the standard agent mode (M3.3 and earlier behavior)
func runTraditionalMode(config *cub.Config) int {

	ctx := context.Background()

	// Parse Redis URL
	redisOpts, err := redis.ParseURL(config.RedisURL)
	if err != nil {
		log.Printf("[ERROR] Invalid REDIS_URL: %v", err)
		return 1
	}

	// Create blackboard client
	bbClient, err := blackboard.NewClient(redisOpts, config.InstanceName)
	if err != nil {
		log.Printf("[ERROR] Failed to create blackboard client: %v", err)
		return 1
	}
	defer func() {
		log.Printf("[DEBUG] Closing blackboard client...")
		if err := bbClient.Close(); err != nil {
			log.Printf("[ERROR] Error closing blackboard client: %v", err)
		}
	}()

	// Verify Redis connection
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	if err := bbClient.Ping(pingCtx); err != nil {
		cancel()
		log.Printf("[ERROR] Failed to connect to Redis: %v", err)
		return 1
	}
	cancel()
	log.Printf("[INFO] Connected to Redis")

	// Create health server
	healthServer := cub.NewHealthServer(bbClient, 8080)

	// Start health server
	if err := healthServer.Start(); err != nil {
		log.Printf("[ERROR] Failed to start health server: %v", err)
		return 1
	}
	log.Printf("[INFO] Health server started on :8080")

	// Create engine
	engine := cub.New(config, bbClient)

	// Set up context for graceful shutdown
	engineCtx, engineCancel := context.WithCancel(context.Background())
	defer engineCancel()

	// Set up signal handling for SIGINT and SIGTERM
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start engine in background goroutine
	engineDone := make(chan error, 1)
	go func() {
		engineDone <- engine.Start(engineCtx)
	}()

	// Wait for shutdown signal or engine error
	select {
	case sig := <-sigChan:
		log.Printf("[INFO] Received signal: %v", sig)
	case err := <-engineDone:
		if err != nil {
			log.Printf("[ERROR] Engine error: %v", err)
			return 1
		}
		// Engine exited normally (shouldn't happen in normal operation)
		log.Printf("[INFO] Engine exited")
		return 0
	}

	// Graceful shutdown sequence

	// 1. Cancel engine context to signal goroutines to stop
	log.Printf("[INFO] Initiating graceful shutdown...")
	engineCancel()

	// 2. Shutdown health server with timeout
	healthShutdownCtx, healthShutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer healthShutdownCancel()

	if err := healthServer.Shutdown(healthShutdownCtx); err != nil {
		log.Printf("[ERROR] Health server shutdown error: %v", err)
		// Continue with shutdown despite error
	}

	// 3. Wait for engine to complete shutdown (with timeout)
	engineShutdownTimer := time.NewTimer(5 * time.Second)
	defer engineShutdownTimer.Stop()

	select {
	case err := <-engineDone:
		if err != nil {
			log.Printf("[ERROR] Engine shutdown error: %v", err)
			return 1
		}
		log.Printf("[INFO] Engine shutdown complete")

	case <-engineShutdownTimer.C:
		log.Printf("[ERROR] Engine shutdown timeout - forcing exit")
		return 1
	}

	// 4. Redis client closed via defer

	log.Printf("[INFO] Cub shutdown complete")
	return 0
}

// runControllerMode runs the controller (bidder-only) mode
// M3.4: Controller containers only bid on claims, never execute work
func runControllerMode(config *cub.Config) int {
	ctx := context.Background()

	// Parse Redis URL
	redisOpts, err := redis.ParseURL(config.RedisURL)
	if err != nil {
		log.Printf("[ERROR] Invalid REDIS_URL: %v", err)
		return 1
	}

	// Create blackboard client
	bbClient, err := blackboard.NewClient(redisOpts, config.InstanceName)
	if err != nil {
		log.Printf("[ERROR] Failed to create blackboard client: %v", err)
		return 1
	}
	defer func() {
		log.Printf("[DEBUG] Closing blackboard client...")
		if err := bbClient.Close(); err != nil {
			log.Printf("[ERROR] Error closing blackboard client: %v", err)
		}
	}()

	// Verify Redis connection
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	if err := bbClient.Ping(pingCtx); err != nil {
		cancel()
		log.Printf("[ERROR] Failed to connect to Redis: %v", err)
		return 1
	}
	cancel()
	log.Printf("[INFO] Connected to Redis")

	// Create health server
	healthServer := cub.NewHealthServer(bbClient, 8080)

	// Start health server
	if err := healthServer.Start(); err != nil {
		log.Printf("[ERROR] Failed to start health server: %v", err)
		return 1
	}
	defer healthServer.Shutdown(context.Background())
	log.Printf("[INFO] Health server started on :8080")

	// Set up context for graceful shutdown
	controllerCtx, controllerCancel := context.WithCancel(ctx)
	defer controllerCancel()

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run controller in background goroutine
	controllerDone := make(chan error, 1)
	go func() {
		controllerDone <- cub.RunControllerMode(controllerCtx, config, bbClient)
	}()

	// Wait for shutdown signal or controller error
	select {
	case sig := <-sigChan:
		log.Printf("[INFO] Received signal: %v", sig)
	case err := <-controllerDone:
		if err != nil {
			log.Printf("[ERROR] Controller error: %v", err)
			return 1
		}
		log.Printf("[INFO] Controller exited")
		return 0
	}

	// Graceful shutdown
	log.Printf("[INFO] Initiating graceful shutdown...")
	controllerCancel()

	shutdownTimer := time.NewTimer(5 * time.Second)
	defer shutdownTimer.Stop()

	select {
	case err := <-controllerDone:
		if err != nil {
			log.Printf("[ERROR] Controller shutdown error: %v", err)
			return 1
		}
		log.Printf("[INFO] Controller shutdown complete")
		return 0

	case <-shutdownTimer.C:
		log.Printf("[ERROR] Controller shutdown timeout - forcing exit")
		return 1
	}
}

// runWorkerMode runs the worker (execute-only) mode
// M3.4: Worker containers execute a specific claim and exit immediately
func runWorkerMode(config *cub.Config, claimID string) int {
	ctx := context.Background()

	// Parse Redis URL
	redisOpts, err := redis.ParseURL(config.RedisURL)
	if err != nil {
		log.Printf("[ERROR] Invalid REDIS_URL: %v", err)
		return 1
	}

	// Create blackboard client
	bbClient, err := blackboard.NewClient(redisOpts, config.InstanceName)
	if err != nil {
		log.Printf("[ERROR] Failed to create blackboard client: %v", err)
		return 1
	}
	defer func() {
		log.Printf("[DEBUG] Closing blackboard client...")
		if err := bbClient.Close(); err != nil {
			log.Printf("[ERROR] Error closing blackboard client: %v", err)
		}
	}()

	// Verify Redis connection
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	if err := bbClient.Ping(pingCtx); err != nil {
		cancel()
		log.Printf("[ERROR] Failed to connect to Redis: %v", err)
		return 1
	}
	cancel()
	log.Printf("[INFO] Connected to Redis")

	// Execute the claim
	if err := cub.RunWorkerMode(ctx, config, bbClient, claimID); err != nil {
		log.Printf("[ERROR] Worker execution failed: %v", err)
		return 1
	}

	log.Printf("[INFO] Worker completed successfully")
	return 0
}
