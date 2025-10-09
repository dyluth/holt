package main

import (
	"context"
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
	// Load configuration from environment variables
	config, err := cub.LoadConfig()
	if err != nil {
		log.Printf("[ERROR] Configuration error: %v", err)
		return 1
	}

	log.Printf("[INFO] Agent cub initializing for agent='%s' instance='%s'", config.AgentName, config.InstanceName)

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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := bbClient.Ping(ctx); err != nil {
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
