package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/docker/docker/client"
	"github.com/dyluth/holt/internal/config"
	"github.com/dyluth/holt/internal/orchestrator"
	"github.com/dyluth/holt/pkg/blackboard"
	"github.com/redis/go-redis/v9"
)

func main() {
	// 1. Load environment variables
	instanceName := os.Getenv("HOLT_INSTANCE_NAME")
	redisURL := os.Getenv("REDIS_URL")

	if instanceName == "" || redisURL == "" {
		fmt.Fprintf(os.Stderr, "Error: HOLT_INSTANCE_NAME and REDIS_URL must be set\n")
		os.Exit(1)
	}

	// 2. Parse Redis URL
	redisOpts, err := redis.ParseURL(redisURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Invalid REDIS_URL: %v\n", err)
		os.Exit(1)
	}

	// 3. Create blackboard client
	bbClient, err := blackboard.NewClient(redisOpts, instanceName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to create blackboard client: %v\n", err)
		os.Exit(1)
	}
	defer bbClient.Close()

	// 4. Verify Redis connectivity
	ctx := context.Background()
	if err := bbClient.Ping(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Redis not accessible: %v\n", err)
		os.Exit(1)
	}

	// 5. Load holt.yml configuration from workspace
	cfg, err := config.Load("/workspace/holt.yml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to load holt.yml: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Orchestrator starting for instance '%s' with %d agents\n", instanceName, len(cfg.Agents))

	// 6. Initialize Docker client for worker management (M3.4)
	// The Docker socket is mounted at /var/run/docker.sock by the CLI
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to create Docker client (worker management disabled): %v\n", err)
		// Continue without worker management - controllers will not be able to launch workers
		dockerClient = nil
	} else {
		// Verify Docker connectivity
		if _, err := dockerClient.Ping(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Docker not accessible (worker management disabled): %v\n", err)
			dockerClient.Close()
			dockerClient = nil
		} else {
			fmt.Println("Docker client initialized for worker management")
		}
	}

	// 7. Create worker manager if Docker is available
	var workerManager *orchestrator.WorkerManager = nil
	if dockerClient != nil {
		// M3.4: Get host workspace path from environment (for worker bind mounts)
		// The orchestrator container has the workspace mounted at /workspace internally,
		// but workers need to mount from the actual host path
		hostWorkspacePath := os.Getenv("HOST_WORKSPACE_PATH")
		if hostWorkspacePath == "" {
			// Fallback: try to use /workspace if not set (for backward compatibility)
			// This may fail if running in a container environment
			hostWorkspacePath = "/workspace"
			fmt.Println("Warning: HOST_WORKSPACE_PATH not set, using /workspace (may fail in containerized environment)")
		}
		workerManager = orchestrator.NewWorkerManager(dockerClient, instanceName, hostWorkspacePath)
		fmt.Println("Worker manager initialized for controller-worker pattern")
	}

	// 8. Create orchestrator engine with config
	engine := orchestrator.NewEngine(bbClient, instanceName, cfg, workerManager)

	// 9. Setup graceful shutdown
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	// 10. Start orchestrator in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- engine.Run(runCtx)
	}()

	// 11. Wait for shutdown signal or error
	select {
	case sig := <-sigCh:
		fmt.Printf("Received signal %v, shutting down gracefully...\n", sig)
		cancel()
		// Wait for engine to finish
		<-errCh
	case runErr := <-errCh:
		if runErr != nil {
			fmt.Fprintf(os.Stderr, "Orchestrator error: %v\n", runErr)
			os.Exit(1)
		}
	}

	// 12. Cleanup Docker client if initialized
	if dockerClient != nil {
		dockerClient.Close()
	}

	fmt.Println("Orchestrator stopped")
}
