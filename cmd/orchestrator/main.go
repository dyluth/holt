package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/dyluth/sett/internal/config"
	"github.com/dyluth/sett/internal/orchestrator"
	"github.com/dyluth/sett/pkg/blackboard"
	"github.com/redis/go-redis/v9"
)

func main() {
	// 1. Load environment variables
	instanceName := os.Getenv("SETT_INSTANCE_NAME")
	redisURL := os.Getenv("REDIS_URL")

	if instanceName == "" || redisURL == "" {
		fmt.Fprintf(os.Stderr, "Error: SETT_INSTANCE_NAME and REDIS_URL must be set\n")
		os.Exit(1)
	}

	// 2. Parse Redis URL
	redisOpts, err := redis.ParseURL(redisURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Invalid REDIS_URL: %v\n", err)
		os.Exit(1)
	}

	// 3. Create blackboard client
	client, err := blackboard.NewClient(redisOpts, instanceName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to create blackboard client: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	// 4. Verify Redis connectivity
	ctx := context.Background()
	if err := client.Ping(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Redis not accessible: %v\n", err)
		os.Exit(1)
	}

	// 5. Load sett.yml configuration from workspace
	cfg, err := config.Load("/workspace/sett.yml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to load sett.yml: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Orchestrator starting for instance '%s' with %d agents\n", instanceName, len(cfg.Agents))

	// 6. Create orchestrator engine with config
	engine := orchestrator.NewEngine(client, instanceName, cfg)

	// 7. Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	// 8. Start orchestrator in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- engine.Run(ctx)
	}()

	// 9. Wait for shutdown signal or error
	select {
	case sig := <-sigCh:
		fmt.Printf("Received signal %v, shutting down gracefully...\n", sig)
		cancel()
		// Wait for engine to finish
		<-errCh
	case err := <-errCh:
		if err != nil {
			fmt.Fprintf(os.Stderr, "Orchestrator error: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("Orchestrator stopped")
}
