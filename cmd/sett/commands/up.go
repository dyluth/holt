package commands

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/dyluth/sett/internal/config"
	dockerpkg "github.com/dyluth/sett/internal/docker"
	"github.com/dyluth/sett/internal/git"
	"github.com/dyluth/sett/internal/instance"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var (
	upInstanceName string
	upForce        bool
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start a Sett instance",
	Long: `Start a new Sett instance in the current Git repository.

Creates and starts:
  • Isolated Docker network
  • Redis container (blackboard storage)
  • Orchestrator container (claim coordinator)

The instance name is auto-generated (default-N) unless specified with --name.
Workspace safety checks prevent multiple instances on the same directory unless --force is used.`,
	RunE: runUp,
}

func init() {
	upCmd.Flags().StringVar(&upInstanceName, "name", "", "Instance name (auto-generated if omitted)")
	upCmd.Flags().BoolVar(&upForce, "force", false, "Bypass workspace collision check")
	rootCmd.AddCommand(upCmd)
}

func runUp(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Phase 1: Environment Validation
	if err := validateEnvironment(); err != nil {
		return err
	}

	// Phase 2: Configuration Validation
	cfg, err := config.Load("sett.yml")
	if err != nil {
		return fmt.Errorf(`sett.yml not found or invalid

No configuration file found in the current directory.

Initialize your project first:
  sett init

Then retry: sett up

Error details: %w`, err)
	}

	// Create Docker client
	cli, err := dockerpkg.NewClient(ctx)
	if err != nil {
		return err
	}
	defer cli.Close()

	// Phase 3: Instance Name Determination
	targetInstanceName := upInstanceName
	if targetInstanceName == "" {
		// Auto-generate default-N name
		targetInstanceName, err = instance.GenerateDefaultName(ctx, cli)
		if err != nil {
			return fmt.Errorf("failed to generate instance name: %w", err)
		}
	}

	// Validate instance name
	if err := instance.ValidateName(targetInstanceName); err != nil {
		return err
	}

	// Check for name collision
	nameCollision, err := instance.CheckNameCollision(ctx, cli, targetInstanceName)
	if err != nil {
		return err
	}
	if nameCollision {
		return fmt.Errorf(`instance '%s' already exists

Found existing containers with this instance name.

Either:
  1. Stop the existing instance: sett down --name %s
  2. Choose a different name: sett up --name other-name`, targetInstanceName, targetInstanceName)
	}

	// Phase 4: Workspace Safety Check
	workspacePath, err := instance.GetCanonicalWorkspacePath()
	if err != nil {
		return fmt.Errorf("failed to get workspace path: %w", err)
	}

	if !upForce {
		collision, err := instance.CheckWorkspaceCollision(ctx, cli, workspacePath, targetInstanceName)
		if err != nil {
			return fmt.Errorf("failed to check workspace collision: %w", err)
		}
		if collision != nil {
			return fmt.Errorf(`workspace in use

Another instance '%s' is already running on this workspace:
  Workspace: %s
  Instance:  %s

Either:
  1. Stop the other instance: sett down --name %s
  2. Use --force to bypass this check (not recommended)`, collision.InstanceName, collision.WorkspacePath, collision.InstanceName, collision.InstanceName)
		}
	}

	// Phase 5: Resource Creation
	runID := uuid.New().String()
	if err := createInstance(ctx, cli, cfg, targetInstanceName, runID, workspacePath); err != nil {
		// Attempt rollback on failure
		fmt.Printf("\nResource creation failed. Rolling back...\n")
		if rollbackErr := rollbackInstance(ctx, cli, targetInstanceName); rollbackErr != nil {
			fmt.Printf("Warning: rollback encountered errors: %v\n", rollbackErr)
		}
		return fmt.Errorf("failed to create instance: %w", err)
	}

	// Success message
	printUpSuccess(targetInstanceName, workspacePath)

	return nil
}

func validateEnvironment() error {
	// Check Git context
	checker := git.NewChecker()
	if err := checker.ValidateGitContext(); err != nil {
		return fmt.Errorf(`not a Git repository

Sett requires initialization from within a Git repository.

Run these commands in order:
  1. git init
  2. sett init
  3. sett up

Error: %w`, err)
	}

	return nil
}

func createInstance(ctx context.Context, cli *client.Client, cfg *config.SettConfig, instanceName, runID, workspacePath string) error {
	// Step 1: Allocate Redis port
	redisPort, err := instance.FindNextAvailablePort(ctx, cli)
	if err != nil {
		return fmt.Errorf("failed to allocate Redis port: %w", err)
	}

	fmt.Printf("✓ Allocated Redis port: %d\n", redisPort)

	// Step 2: Create isolated network
	networkName := dockerpkg.NetworkName(instanceName)
	networkLabels := dockerpkg.BuildLabels(instanceName, runID, workspacePath, "")

	_, err = cli.NetworkCreate(ctx, networkName, types.NetworkCreate{
		Driver: "bridge",
		Labels: networkLabels,
	})
	if err != nil {
		return fmt.Errorf("failed to create network '%s': %w", networkName, err)
	}

	fmt.Printf("✓ Created network: %s\n", networkName)

	// Step 3: Start Redis container with port mapping
	redisImage := "redis:7-alpine"
	if cfg.Services != nil && cfg.Services.Redis != nil && cfg.Services.Redis.Image != "" {
		redisImage = cfg.Services.Redis.Image
	}

	redisName := dockerpkg.RedisContainerName(instanceName)
	redisLabels := dockerpkg.BuildLabels(instanceName, runID, workspacePath, "redis")
	// Add Redis port label
	redisLabels[dockerpkg.LabelRedisPort] = fmt.Sprintf("%d", redisPort)

	redisResp, err := cli.ContainerCreate(ctx, &container.Config{
		Image:  redisImage,
		Labels: redisLabels,
		ExposedPorts: nat.PortSet{
			"6379/tcp": struct{}{},
		},
	}, &container.HostConfig{
		NetworkMode: container.NetworkMode(networkName),
		PortBindings: nat.PortMap{
			"6379/tcp": []nat.PortBinding{
				{
					HostIP:   "127.0.0.1",
					HostPort: fmt.Sprintf("%d", redisPort),
				},
			},
		},
	}, nil, nil, redisName)
	if err != nil {
		return fmt.Errorf("failed to create Redis container: %w", err)
	}

	if err := cli.ContainerStart(ctx, redisResp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start Redis container: %w", err)
	}

	fmt.Printf("✓ Started Redis container: %s (port %d)\n", redisName, redisPort)

	// Step 4: Build orchestrator image
	orchestratorImage := "sett-orchestrator:latest"
	fmt.Printf("Building orchestrator image...\n")
	if err := buildOrchestratorImage(ctx, cli, orchestratorImage); err != nil {
		return fmt.Errorf("failed to build orchestrator image: %w", err)
	}

	fmt.Printf("✓ Built orchestrator image: %s\n", orchestratorImage)

	// Step 5: Start Orchestrator container with real binary
	orchestratorName := dockerpkg.OrchestratorContainerName(instanceName)
	orchestratorLabels := dockerpkg.BuildLabels(instanceName, runID, workspacePath, "orchestrator")

	// Use Redis container name as hostname (Docker DNS)
	redisURL := fmt.Sprintf("redis://%s:6379", redisName)

	orchestratorResp, err := cli.ContainerCreate(ctx, &container.Config{
		Image:  orchestratorImage,
		Labels: orchestratorLabels,
		Env: []string{
			fmt.Sprintf("SETT_INSTANCE_NAME=%s", instanceName),
			fmt.Sprintf("REDIS_URL=%s", redisURL),
		},
	}, &container.HostConfig{
		NetworkMode: container.NetworkMode(networkName),
		Binds: []string{
			fmt.Sprintf("%s:/workspace:ro", workspacePath),
		},
	}, nil, nil, orchestratorName)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator container: %w", err)
	}

	if err := cli.ContainerStart(ctx, orchestratorResp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start orchestrator container: %w", err)
	}

	fmt.Printf("✓ Started orchestrator container: %s\n", orchestratorName)

	return nil
}

func rollbackInstance(ctx context.Context, cli *client.Client, instanceName string) error {
	timeout := 10

	// Find all containers for this instance
	containers, err := cli.ContainerList(ctx, container.ListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", fmt.Sprintf("%s=%s", dockerpkg.LabelInstanceName, instanceName)),
		),
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	// Stop and remove containers
	for _, c := range containers {
		fmt.Printf("  Stopping %s...\n", c.Names[0])
		_ = cli.ContainerStop(ctx, c.ID, container.StopOptions{Timeout: &timeout})

		fmt.Printf("  Removing %s...\n", c.Names[0])
		if err := cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true}); err != nil {
			fmt.Printf("  Warning: failed to remove %s: %v\n", c.Names[0], err)
		}
	}

	// Remove network
	networks, err := cli.NetworkList(ctx, types.NetworkListOptions{
		Filters: filters.NewArgs(
			filters.Arg("label", fmt.Sprintf("%s=%s", dockerpkg.LabelInstanceName, instanceName)),
		),
	})
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}

	for _, net := range networks {
		fmt.Printf("  Removing network %s...\n", net.Name)
		if err := cli.NetworkRemove(ctx, net.ID); err != nil {
			fmt.Printf("  Warning: failed to remove network %s: %v\n", net.Name, err)
		}
	}

	return nil
}

func printUpSuccess(instanceName, workspacePath string) {
	fmt.Printf("\n✓ Instance '%s' started successfully\n\n", instanceName)
	fmt.Printf("Containers:\n")
	fmt.Printf("  • %s (running)\n", dockerpkg.RedisContainerName(instanceName))
	fmt.Printf("  • %s (running)\n", dockerpkg.OrchestratorContainerName(instanceName))
	fmt.Printf("\n")
	fmt.Printf("Network:\n")
	fmt.Printf("  • %s\n", dockerpkg.NetworkName(instanceName))
	fmt.Printf("\n")
	fmt.Printf("Workspace: %s\n", workspacePath)
	fmt.Printf("\n")
	fmt.Printf("Next steps:\n")
	fmt.Printf("  1. Run 'sett forage --goal \"your goal\"' to start a workflow\n")
	fmt.Printf("  2. Run 'sett list' to view all instances\n")
	fmt.Printf("  3. Run 'sett down --name %s' when finished\n", instanceName)
}

func buildOrchestratorImage(ctx context.Context, cli *client.Client, imageName string) error {
	// Get the project root directory
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Create build context as tar archive
	buildContext, err := createBuildContext(projectRoot)
	if err != nil {
		return fmt.Errorf("failed to create build context: %w", err)
	}

	// Build image
	buildOptions := types.ImageBuildOptions{
		Tags:       []string{imageName},
		Dockerfile: "Dockerfile.orchestrator",
		Remove:     true,
		Platform:   "linux/arm64", // ARM64-first design
	}

	resp, err := cli.ImageBuild(ctx, buildContext, buildOptions)
	if err != nil {
		return fmt.Errorf("failed to build image: %w", err)
	}
	defer resp.Body.Close()

	// Read build output (discarding it, but checking for errors)
	_, err = io.Copy(io.Discard, resp.Body)
	if err != nil {
		return fmt.Errorf("error reading build output: %w", err)
	}

	return nil
}

func createBuildContext(projectRoot string) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	defer tw.Close()

	// Files and directories to include in build context
	includes := []string{
		"go.mod",
		"go.sum",
		"Dockerfile.orchestrator",
		"cmd/",
		"pkg/",
		"internal/",
	}

	for _, include := range includes {
		fullPath := filepath.Join(projectRoot, include)

		// Check if path exists
		info, err := os.Stat(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue // Skip if doesn't exist
			}
			return nil, err
		}

		if info.IsDir() {
			// Add directory recursively
			err = filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				// Skip hidden files/directories and test files
				if filepath.Base(path)[0] == '.' {
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}

				// Get relative path for tar
				relPath, err := filepath.Rel(projectRoot, path)
				if err != nil {
					return err
				}

				// Create tar header
				header, err := tar.FileInfoHeader(info, "")
				if err != nil {
					return err
				}
				header.Name = relPath

				if err := tw.WriteHeader(header); err != nil {
					return err
				}

				// If it's a file, write its contents
				if !info.IsDir() {
					file, err := os.Open(path)
					if err != nil {
						return err
					}
					defer file.Close()

					if _, err := io.Copy(tw, file); err != nil {
						return err
					}
				}

				return nil
			})
			if err != nil {
				return nil, err
			}
		} else {
			// Add single file
			file, err := os.Open(fullPath)
			if err != nil {
				return nil, err
			}
			defer file.Close()

			relPath, err := filepath.Rel(projectRoot, fullPath)
			if err != nil {
				return nil, err
			}

			header, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return nil, err
			}
			header.Name = relPath

			if err := tw.WriteHeader(header); err != nil {
				return nil, err
			}

			if _, err := io.Copy(tw, file); err != nil {
				return nil, err
			}
		}
	}

	return &buf, nil
}
