package pup

import (
	"os"
	"testing"
)

func TestLoadConfig_Success(t *testing.T) {
	// Set up valid environment
	os.Setenv("HOLT_INSTANCE_NAME", "test-instance")
	os.Setenv("HOLT_AGENT_NAME", "test-agent")
	os.Setenv("HOLT_AGENT_ROLE", "coder")
	os.Setenv("REDIS_URL", "redis://localhost:6379")
	os.Setenv("HOLT_AGENT_COMMAND", `["/app/run.sh"]`)
	os.Setenv("HOLT_BIDDING_STRATEGY", "exclusive") // M3.1: Required
	defer func() {
		os.Unsetenv("HOLT_INSTANCE_NAME")
		os.Unsetenv("HOLT_AGENT_NAME")
		os.Unsetenv("HOLT_AGENT_ROLE")
		os.Unsetenv("REDIS_URL")
		os.Unsetenv("HOLT_AGENT_COMMAND")
		os.Unsetenv("HOLT_BIDDING_STRATEGY")
	}()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if cfg.InstanceName != "test-instance" {
		t.Errorf("Expected InstanceName='test-instance', got '%s'", cfg.InstanceName)
	}

	if cfg.AgentName != "test-agent" {
		t.Errorf("Expected AgentName='test-agent', got '%s'", cfg.AgentName)
	}

	if cfg.AgentRole != "coder" {
		t.Errorf("Expected AgentRole='coder', got '%s'", cfg.AgentRole)
	}

	if cfg.RedisURL != "redis://localhost:6379" {
		t.Errorf("Expected RedisURL='redis://localhost:6379', got '%s'", cfg.RedisURL)
	}

	if len(cfg.Command) != 1 || cfg.Command[0] != "/app/run.sh" {
		t.Errorf("Expected Command=['/app/run.sh'], got %v", cfg.Command)
	}
}

func TestLoadConfig_MissingInstanceName(t *testing.T) {
	// Set up environment with missing HOLT_INSTANCE_NAME
	os.Unsetenv("HOLT_INSTANCE_NAME")
	os.Setenv("HOLT_AGENT_NAME", "test-agent")
	os.Setenv("HOLT_AGENT_ROLE", "coder")
	os.Setenv("REDIS_URL", "redis://localhost:6379")
	defer func() {
		os.Unsetenv("HOLT_AGENT_NAME")
		os.Unsetenv("HOLT_AGENT_ROLE")
		os.Unsetenv("REDIS_URL")
	}()

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("Expected error for missing HOLT_INSTANCE_NAME, got nil")
	}

	expected := "HOLT_INSTANCE_NAME environment variable is required"
	if err.Error() != expected {
		t.Errorf("Expected error '%s', got '%s'", expected, err.Error())
	}
}

func TestLoadConfig_MissingAgentName(t *testing.T) {
	// Set up environment with missing HOLT_AGENT_NAME
	os.Setenv("HOLT_INSTANCE_NAME", "test-instance")
	os.Unsetenv("HOLT_AGENT_NAME")
	os.Setenv("HOLT_AGENT_ROLE", "coder")
	os.Setenv("REDIS_URL", "redis://localhost:6379")
	defer func() {
		os.Unsetenv("HOLT_INSTANCE_NAME")
		os.Unsetenv("HOLT_AGENT_ROLE")
		os.Unsetenv("REDIS_URL")
	}()

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("Expected error for missing HOLT_AGENT_NAME, got nil")
	}

	expected := "HOLT_AGENT_NAME environment variable is required"
	if err.Error() != expected {
		t.Errorf("Expected error '%s', got '%s'", expected, err.Error())
	}
}

func TestLoadConfig_MissingAgentRole(t *testing.T) {
	// Set up environment with missing HOLT_AGENT_ROLE
	os.Setenv("HOLT_INSTANCE_NAME", "test-instance")
	os.Setenv("HOLT_AGENT_NAME", "test-agent")
	os.Unsetenv("HOLT_AGENT_ROLE")
	os.Setenv("REDIS_URL", "redis://localhost:6379")
	defer func() {
		os.Unsetenv("HOLT_INSTANCE_NAME")
		os.Unsetenv("HOLT_AGENT_NAME")
		os.Unsetenv("REDIS_URL")
	}()

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("Expected error for missing HOLT_AGENT_ROLE, got nil")
	}

	expected := "HOLT_AGENT_ROLE environment variable is required"
	if err.Error() != expected {
		t.Errorf("Expected error '%s', got '%s'", expected, err.Error())
	}
}

func TestLoadConfig_MissingRedisURL(t *testing.T) {
	// Set up environment with missing REDIS_URL
	os.Setenv("HOLT_INSTANCE_NAME", "test-instance")
	os.Setenv("HOLT_AGENT_NAME", "test-agent")
	os.Setenv("HOLT_AGENT_ROLE", "coder")
	os.Unsetenv("REDIS_URL")
	defer func() {
		os.Unsetenv("HOLT_INSTANCE_NAME")
		os.Unsetenv("HOLT_AGENT_NAME")
		os.Unsetenv("HOLT_AGENT_ROLE")
	}()

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("Expected error for missing REDIS_URL, got nil")
	}

	expected := "REDIS_URL environment variable is required"
	if err.Error() != expected {
		t.Errorf("Expected error '%s', got '%s'", expected, err.Error())
	}
}

func TestLoadConfig_EmptyInstanceName(t *testing.T) {
	// Set up environment with empty HOLT_INSTANCE_NAME
	os.Setenv("HOLT_INSTANCE_NAME", "")
	os.Setenv("HOLT_AGENT_NAME", "test-agent")
	os.Setenv("HOLT_AGENT_ROLE", "coder")
	os.Setenv("REDIS_URL", "redis://localhost:6379")
	defer func() {
		os.Unsetenv("HOLT_INSTANCE_NAME")
		os.Unsetenv("HOLT_AGENT_NAME")
		os.Unsetenv("HOLT_AGENT_ROLE")
		os.Unsetenv("REDIS_URL")
	}()

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("Expected error for empty HOLT_INSTANCE_NAME, got nil")
	}

	expected := "HOLT_INSTANCE_NAME environment variable is required"
	if err.Error() != expected {
		t.Errorf("Expected error '%s', got '%s'", expected, err.Error())
	}
}

func TestLoadConfig_EmptyAgentName(t *testing.T) {
	// Set up environment with empty HOLT_AGENT_NAME
	os.Setenv("HOLT_INSTANCE_NAME", "test-instance")
	os.Setenv("HOLT_AGENT_NAME", "")
	os.Setenv("HOLT_AGENT_ROLE", "coder")
	os.Setenv("REDIS_URL", "redis://localhost:6379")
	defer func() {
		os.Unsetenv("HOLT_INSTANCE_NAME")
		os.Unsetenv("HOLT_AGENT_NAME")
		os.Unsetenv("HOLT_AGENT_ROLE")
		os.Unsetenv("REDIS_URL")
	}()

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("Expected error for empty HOLT_AGENT_NAME, got nil")
	}

	expected := "HOLT_AGENT_NAME environment variable is required"
	if err.Error() != expected {
		t.Errorf("Expected error '%s', got '%s'", expected, err.Error())
	}
}

func TestLoadConfig_EmptyAgentRole(t *testing.T) {
	// Set up environment with empty HOLT_AGENT_ROLE
	os.Setenv("HOLT_INSTANCE_NAME", "test-instance")
	os.Setenv("HOLT_AGENT_NAME", "test-agent")
	os.Setenv("HOLT_AGENT_ROLE", "")
	os.Setenv("REDIS_URL", "redis://localhost:6379")
	defer func() {
		os.Unsetenv("HOLT_INSTANCE_NAME")
		os.Unsetenv("HOLT_AGENT_NAME")
		os.Unsetenv("HOLT_AGENT_ROLE")
		os.Unsetenv("REDIS_URL")
	}()

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("Expected error for empty HOLT_AGENT_ROLE, got nil")
	}

	expected := "HOLT_AGENT_ROLE environment variable is required"
	if err.Error() != expected {
		t.Errorf("Expected error '%s', got '%s'", expected, err.Error())
	}
}

func TestLoadConfig_EmptyRedisURL(t *testing.T) {
	// Set up environment with empty REDIS_URL
	os.Setenv("HOLT_INSTANCE_NAME", "test-instance")
	os.Setenv("HOLT_AGENT_NAME", "test-agent")
	os.Setenv("HOLT_AGENT_ROLE", "coder")
	os.Setenv("REDIS_URL", "")
	defer func() {
		os.Unsetenv("HOLT_INSTANCE_NAME")
		os.Unsetenv("HOLT_AGENT_NAME")
		os.Unsetenv("HOLT_AGENT_ROLE")
		os.Unsetenv("REDIS_URL")
	}()

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("Expected error for empty REDIS_URL, got nil")
	}

	expected := "REDIS_URL environment variable is required"
	if err.Error() != expected {
		t.Errorf("Expected error '%s', got '%s'", expected, err.Error())
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &Config{
		InstanceName:    "test-instance",
		AgentName:       "test-agent",
		AgentRole:       "coder",
		RedisURL:        "redis://localhost:6379",
		Command:         []string{"/app/run.sh"},
		BiddingStrategy: "exclusive", // M3.1: Required
	}

	err := cfg.Validate()
	if err != nil {
		t.Fatalf("Expected no error for valid config, got: %v", err)
	}
}

func TestValidate_InvalidConfig(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *Config
		expectedErr string
	}{
		{
			name: "empty instance name",
			cfg: &Config{
				InstanceName: "",
				AgentName:    "test-agent",
				AgentRole:    "coder",
				RedisURL:     "redis://localhost:6379",
			},
			expectedErr: "HOLT_INSTANCE_NAME environment variable is required",
		},
		{
			name: "empty agent name",
			cfg: &Config{
				InstanceName: "test-instance",
				AgentName:    "",
				AgentRole:    "coder",
				RedisURL:     "redis://localhost:6379",
			},
			expectedErr: "HOLT_AGENT_NAME environment variable is required",
		},
		{
			name: "empty agent role",
			cfg: &Config{
				InstanceName: "test-instance",
				AgentName:    "test-agent",
				AgentRole:    "",
				RedisURL:     "redis://localhost:6379",
			},
			expectedErr: "HOLT_AGENT_ROLE environment variable is required",
		},
		{
			name: "empty redis URL",
			cfg: &Config{
				InstanceName: "test-instance",
				AgentName:    "test-agent",
				AgentRole:    "coder",
				RedisURL:     "",
			},
			expectedErr: "REDIS_URL environment variable is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if err == nil {
				t.Fatal("Expected validation error, got nil")
			}
			if err.Error() != tt.expectedErr {
				t.Errorf("Expected error '%s', got '%s'", tt.expectedErr, err.Error())
			}
		})
	}
}
