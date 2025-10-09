package cub

import (
	"os"
	"testing"
)

func TestLoadConfig_Success(t *testing.T) {
	// Set up valid environment
	os.Setenv("SETT_INSTANCE_NAME", "test-instance")
	os.Setenv("SETT_AGENT_NAME", "test-agent")
	os.Setenv("REDIS_URL", "redis://localhost:6379")
	defer func() {
		os.Unsetenv("SETT_INSTANCE_NAME")
		os.Unsetenv("SETT_AGENT_NAME")
		os.Unsetenv("REDIS_URL")
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

	if cfg.RedisURL != "redis://localhost:6379" {
		t.Errorf("Expected RedisURL='redis://localhost:6379', got '%s'", cfg.RedisURL)
	}
}

func TestLoadConfig_MissingInstanceName(t *testing.T) {
	// Set up environment with missing SETT_INSTANCE_NAME
	os.Unsetenv("SETT_INSTANCE_NAME")
	os.Setenv("SETT_AGENT_NAME", "test-agent")
	os.Setenv("REDIS_URL", "redis://localhost:6379")
	defer func() {
		os.Unsetenv("SETT_AGENT_NAME")
		os.Unsetenv("REDIS_URL")
	}()

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("Expected error for missing SETT_INSTANCE_NAME, got nil")
	}

	expected := "SETT_INSTANCE_NAME environment variable is required"
	if err.Error() != expected {
		t.Errorf("Expected error '%s', got '%s'", expected, err.Error())
	}
}

func TestLoadConfig_MissingAgentName(t *testing.T) {
	// Set up environment with missing SETT_AGENT_NAME
	os.Setenv("SETT_INSTANCE_NAME", "test-instance")
	os.Unsetenv("SETT_AGENT_NAME")
	os.Setenv("REDIS_URL", "redis://localhost:6379")
	defer func() {
		os.Unsetenv("SETT_INSTANCE_NAME")
		os.Unsetenv("REDIS_URL")
	}()

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("Expected error for missing SETT_AGENT_NAME, got nil")
	}

	expected := "SETT_AGENT_NAME environment variable is required"
	if err.Error() != expected {
		t.Errorf("Expected error '%s', got '%s'", expected, err.Error())
	}
}

func TestLoadConfig_MissingRedisURL(t *testing.T) {
	// Set up environment with missing REDIS_URL
	os.Setenv("SETT_INSTANCE_NAME", "test-instance")
	os.Setenv("SETT_AGENT_NAME", "test-agent")
	os.Unsetenv("REDIS_URL")
	defer func() {
		os.Unsetenv("SETT_INSTANCE_NAME")
		os.Unsetenv("SETT_AGENT_NAME")
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
	// Set up environment with empty SETT_INSTANCE_NAME
	os.Setenv("SETT_INSTANCE_NAME", "")
	os.Setenv("SETT_AGENT_NAME", "test-agent")
	os.Setenv("REDIS_URL", "redis://localhost:6379")
	defer func() {
		os.Unsetenv("SETT_INSTANCE_NAME")
		os.Unsetenv("SETT_AGENT_NAME")
		os.Unsetenv("REDIS_URL")
	}()

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("Expected error for empty SETT_INSTANCE_NAME, got nil")
	}

	expected := "SETT_INSTANCE_NAME environment variable is required"
	if err.Error() != expected {
		t.Errorf("Expected error '%s', got '%s'", expected, err.Error())
	}
}

func TestLoadConfig_EmptyAgentName(t *testing.T) {
	// Set up environment with empty SETT_AGENT_NAME
	os.Setenv("SETT_INSTANCE_NAME", "test-instance")
	os.Setenv("SETT_AGENT_NAME", "")
	os.Setenv("REDIS_URL", "redis://localhost:6379")
	defer func() {
		os.Unsetenv("SETT_INSTANCE_NAME")
		os.Unsetenv("SETT_AGENT_NAME")
		os.Unsetenv("REDIS_URL")
	}()

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("Expected error for empty SETT_AGENT_NAME, got nil")
	}

	expected := "SETT_AGENT_NAME environment variable is required"
	if err.Error() != expected {
		t.Errorf("Expected error '%s', got '%s'", expected, err.Error())
	}
}

func TestLoadConfig_EmptyRedisURL(t *testing.T) {
	// Set up environment with empty REDIS_URL
	os.Setenv("SETT_INSTANCE_NAME", "test-instance")
	os.Setenv("SETT_AGENT_NAME", "test-agent")
	os.Setenv("REDIS_URL", "")
	defer func() {
		os.Unsetenv("SETT_INSTANCE_NAME")
		os.Unsetenv("SETT_AGENT_NAME")
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
		InstanceName: "test-instance",
		AgentName:    "test-agent",
		RedisURL:     "redis://localhost:6379",
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
				RedisURL:     "redis://localhost:6379",
			},
			expectedErr: "SETT_INSTANCE_NAME environment variable is required",
		},
		{
			name: "empty agent name",
			cfg: &Config{
				InstanceName: "test-instance",
				AgentName:    "",
				RedisURL:     "redis://localhost:6379",
			},
			expectedErr: "SETT_AGENT_NAME environment variable is required",
		},
		{
			name: "empty redis URL",
			cfg: &Config{
				InstanceName: "test-instance",
				AgentName:    "test-agent",
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
