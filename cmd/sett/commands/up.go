package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/dyluth/sett/internal/config"
	dockerpkg "github.com/dyluth/sett/internal/docker"
	"github.com/dyluth/sett/internal/git"
	"github.com/dyluth/sett/internal/instance"
	"github.com/dyluth/sett/internal/printer"
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
	upCmd.Flags().StringVarP(&upInstanceName, "name", "n", "", "Instance name (auto-generated if omitted)")
	upCmd.Flags().BoolVarP(&upForce, "force", "f", false, "Bypass workspace collision check")
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
		return printer.Error(
			"sett.yml not found or invalid",
			"No configuration file found in the current directory.",
			[]string{
				"Initialize your project first:\n  sett init",
				"Then retry: sett up",
			},
		)
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
		return printer.Error(
			fmt.Sprintf("instance '%s' already exists", targetInstanceName),
			"Found existing containers with this instance name.",
			[]string{
				fmt.Sprintf("Stop the existing instance: sett down --name %s", targetInstanceName),
				"Choose a different name: sett up --name other-name",
			},
		)
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
			return printer.ErrorWithContext(
				"workspace in use",
				fmt.Sprintf("Another instance '%s' is already running on this workspace:", collision.InstanceName),
				map[string]string{
					"Workspace": collision.WorkspacePath,
					"Instance":  collision.InstanceName,
				},
				[]string{
					fmt.Sprintf("Stop the other instance: sett down --name %s", collision.InstanceName),
					"Use --force to bypass this check (not recommended)",
				},
			)
		}
	}

	// Phase 5: Resource Creation
	runID := uuid.New().String()
	if err := createInstance(ctx, cli, cfg, targetInstanceName, runID, workspacePath); err != nil {
		// Attempt rollback on failure
		printer.Info("\nResource creation failed. Rolling back...\n")
		if rollbackErr := rollbackInstance(ctx, cli, targetInstanceName); rollbackErr != nil {
			printer.Warning("rollback encountered errors: %v\n", rollbackErr)
		}
		return fmt.Errorf("failed to create instance: %w", err)
	}

	// Success message
	printUpSuccess(targetInstanceName, workspacePath, cfg)

	return nil
}

func validateEnvironment() error {
	// Check Git context
	checker := git.NewChecker()
	if err := checker.ValidateGitContext(); err != nil {
		return printer.Error(
			"not a Git repository",
			"Sett requires initialization from within a Git repository.",
			[]string{"Run these commands in order:\n  1. git init\n  2. sett init\n  3. sett up"},
		)
	}

	return nil
}

func createInstance(ctx context.Context, cli *client.Client, cfg *config.SettConfig, instanceName, runID, workspacePath string) error {
	// Step 1: Validate all agent images exist
	if err := validateAgentImages(ctx, cli, cfg); err != nil {
		return err
	}

	// Step 2: Allocate Redis port
	redisPort, err := instance.FindNextAvailablePort(ctx, cli)
	if err != nil {
		return fmt.Errorf("failed to allocate Redis port: %w", err)
	}

	printer.Success("Allocated Redis port: %d\n", redisPort)

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

	printer.Success("Created network: %s\n", networkName)

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

	printer.Success("Started Redis container: %s (port %d)\n", redisName, redisPort)

	// Step 4: Verify orchestrator image exists
	orchestratorImage := "sett-orchestrator:latest"
	if err := verifyOrchestratorImage(ctx, cli, orchestratorImage); err != nil {
		return err
	}

	// Step 5: Start Orchestrator container with pre-built image
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

	printer.Success("Started orchestrator container: %s\n", orchestratorName)

	// Step 6: Launch agent containers
	if err := launchAgentContainers(ctx, cli, cfg, instanceName, runID, workspacePath, networkName, redisName); err != nil {
		return fmt.Errorf("failed to launch agent containers: %w", err)
	}

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
		printer.Info("  Stopping %s...\n", c.Names[0])
		_ = cli.ContainerStop(ctx, c.ID, container.StopOptions{Timeout: &timeout})

		printer.Info("  Removing %s...\n", c.Names[0])
		if err := cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true}); err != nil {
			printer.Warning("failed to remove %s: %v\n", c.Names[0], err)
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
		printer.Info("  Removing network %s...\n", net.Name)
		if err := cli.NetworkRemove(ctx, net.ID); err != nil {
			printer.Warning("failed to remove network %s: %v\n", net.Name, err)
		}
	}

	return nil
}

func printUpSuccess(instanceName, workspacePath string, cfg *config.SettConfig) {
	printer.Success("\nInstance '%s' started successfully\n\n", instanceName)
	printer.Info("Containers:\n")
	printer.Info("  • %s (running)\n", dockerpkg.RedisContainerName(instanceName))
	printer.Info("  • %s (running)\n", dockerpkg.OrchestratorContainerName(instanceName))

	// List agent containers
	for agentName := range cfg.Agents {
		printer.Info("  • %s (running)\n", dockerpkg.AgentContainerName(instanceName, agentName))
	}

	printer.Info("\n")
	printer.Info("Network:\n")
	printer.Info("  • %s\n", dockerpkg.NetworkName(instanceName))
	printer.Info("\n")
	printer.Info("Workspace: %s\n", workspacePath)
	printer.Info("\n")
	printer.Info("Next steps:\n")
	printer.Info("  1. Run 'sett forage --goal \"your goal\"' to start a workflow\n")
	if len(cfg.Agents) > 0 {
		// Get first agent name for example
		var firstAgent string
		for name := range cfg.Agents {
			firstAgent = name
			break
		}
		printer.Info("  2. Run 'sett logs %s' to view agent logs\n", firstAgent)
		printer.Info("  3. Run 'sett down --name %s' when finished\n", instanceName)
	} else {
		printer.Info("  2. Run 'sett list' to view all instances\n")
		printer.Info("  3. Run 'sett down --name %s' when finished\n", instanceName)
	}
}

func verifyOrchestratorImage(ctx context.Context, cli *client.Client, imageName string) error {
	// Check if the image exists locally
	images, err := cli.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list Docker images: %w", err)
	}

	// Look for the orchestrator image
	for _, image := range images {
		for _, tag := range image.RepoTags {
			if tag == imageName {
				printer.Success("Found orchestrator image: %s\n", imageName)
				return nil
			}
		}
	}

	// Image not found - return helpful error
	return printer.Error(
		fmt.Sprintf("orchestrator image '%s' not found", imageName),
		"",
		[]string{"Please run 'make docker-orchestrator' to build it first."},
	)
}

func validateAgentImages(ctx context.Context, cli *client.Client, cfg *config.SettConfig) error {
	if len(cfg.Agents) == 0 {
		return nil
	}

	// Get list of all local images
	images, err := cli.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list Docker images: %w", err)
	}

	// Build set of available image tags
	availableImages := make(map[string]bool)
	for _, image := range images {
		for _, tag := range image.RepoTags {
			availableImages[tag] = true
		}
	}

	// Validate each agent image exists
	var missingImages []string
	for agentName, agent := range cfg.Agents {
		if !availableImages[agent.Image] {
			missingImages = append(missingImages, fmt.Sprintf("%s (for agent '%s')", agent.Image, agentName))
		}
	}

	if len(missingImages) > 0 {
		return printer.Error(
			"agent images not found",
			fmt.Sprintf("The following agent images are not available locally:\n  - %s",
				missingImages[0]),
			[]string{
				"Build the agent images first:",
				"  cd agents/<agent-dir>",
				"  docker build -t <image-name> .",
				"",
				"Then retry: sett up",
			},
		)
	}

	printer.Success("Validated %d agent image(s)\n", len(cfg.Agents))
	return nil
}

func launchAgentContainers(ctx context.Context, cli *client.Client, cfg *config.SettConfig, instanceName, runID, workspacePath, networkName, redisName string) error {
	if len(cfg.Agents) == 0 {
		return nil
	}

	for agentName, agent := range cfg.Agents {
		if err := launchAgentContainer(ctx, cli, instanceName, runID, workspacePath, networkName, redisName, agentName, agent); err != nil {
			return fmt.Errorf("failed to launch agent '%s': %w", agentName, err)
		}
	}

	return nil
}

func launchAgentContainer(ctx context.Context, cli *client.Client, instanceName, runID, workspacePath, networkName, redisName, agentName string, agent config.Agent) error {
	containerName := dockerpkg.AgentContainerName(instanceName, agentName)
	labels := dockerpkg.BuildLabels(instanceName, runID, workspacePath, "agent")
	labels[dockerpkg.LabelAgentName] = agentName

	// Determine workspace mode (default to ro)
	workspaceMode := "ro"
	if agent.Workspace != nil && agent.Workspace.Mode != "" {
		workspaceMode = agent.Workspace.Mode
	}

	// Build environment variables
	redisURL := fmt.Sprintf("redis://%s:6379", redisName)
	env := []string{
		fmt.Sprintf("SETT_INSTANCE_NAME=%s", instanceName),
		fmt.Sprintf("SETT_AGENT_NAME=%s", agentName),
		fmt.Sprintf("SETT_AGENT_ROLE=%s", agent.Role),
		fmt.Sprintf("REDIS_URL=%s", redisURL),
	}

	// Add SETT_AGENT_COMMAND as JSON array
	if len(agent.Command) > 0 {
		commandJSON, err := json.Marshal(agent.Command)
		if err != nil {
			return fmt.Errorf("failed to marshal agent command to JSON: %w", err)
		}
		env = append(env, fmt.Sprintf("SETT_AGENT_COMMAND=%s", commandJSON))
	}

	// Add custom environment variables from config
	if len(agent.Environment) > 0 {
		env = append(env, agent.Environment...)
	}

	// Create container
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image:  agent.Image,
		Labels: labels,
		Env:    env,
		Cmd:    agent.Command,
	}, &container.HostConfig{
		NetworkMode: container.NetworkMode(networkName),
		Binds: []string{
			fmt.Sprintf("%s:/workspace:%s", workspacePath, workspaceMode),
		},
	}, nil, nil, containerName)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	printer.Success("Started agent container: %s\n", containerName)
	return nil
}
