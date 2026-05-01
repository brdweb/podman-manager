package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Manager   ManagerConfig   `yaml:"manager"`
	Agent     AgentConfig     `yaml:"agent"`
	Podman    PodmanConfig    `yaml:"podman"`
	Heartbeat HeartbeatConfig `yaml:"heartbeat"`
	Reconnect ReconnectConfig `yaml:"reconnect"`
	Log       LogConfig       `yaml:"log"`
}

type ManagerConfig struct {
	Address     string `yaml:"address"`
	TLS         bool   `yaml:"tls"`
	TLSInsecure bool   `yaml:"tls_insecure"`
}

type AgentConfig struct {
	ID         string `yaml:"id"`
	Credential string `yaml:"credential"`
	Token      string `yaml:"token"`
}

type PodmanConfig struct {
	SocketPath string        `yaml:"socket_path"`
	Timeout    time.Duration `yaml:"timeout"`
}

type HeartbeatConfig struct {
	Interval time.Duration `yaml:"interval"`
	Timeout  time.Duration `yaml:"timeout"`
}

type ReconnectConfig struct {
	InitialBackoff time.Duration `yaml:"initial_backoff"`
	MaxBackoff     time.Duration `yaml:"max_backoff"`
	Multiplier     float64       `yaml:"multiplier"`
}

type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading config file %s: %w", path, err)
	}

	cfg := defaultConfig()
	if err == nil && len(data) > 0 {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing config file: %w", err)
		}
	}

	applyEnv(cfg)

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}

func Save(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing config file %s: %w", path, err)
	}
	return nil
}

func applyEnv(cfg *Config) {
	if value := strings.TrimSpace(os.Getenv("AGENT_MANAGER_URL")); value != "" {
		cfg.Manager.Address = value
	}
	if value := strings.TrimSpace(os.Getenv("AGENT_TOKEN")); value != "" {
		cfg.Agent.Token = value
	}
	if value := strings.TrimSpace(os.Getenv("AGENT_ID")); value != "" {
		cfg.Agent.ID = value
	}
	if value := strings.TrimSpace(os.Getenv("AGENT_CREDENTIAL")); value != "" {
		cfg.Agent.Credential = value
	}
	if value := strings.TrimSpace(os.Getenv("AGENT_PODMAN_SOCKET")); value != "" {
		cfg.Podman.SocketPath = value
	}
	if value := strings.TrimSpace(os.Getenv("AGENT_LOG_LEVEL")); value != "" {
		cfg.Log.Level = value
	}
	if value := strings.TrimSpace(os.Getenv("AGENT_LOG_FORMAT")); value != "" {
		cfg.Log.Format = value
	}
	if value := strings.TrimSpace(os.Getenv("AGENT_TLS")); value != "" {
		cfg.Manager.TLS = strings.EqualFold(value, "true") || value == "1"
	}
	if value := strings.TrimSpace(os.Getenv("AGENT_TLS_INSECURE")); value != "" {
		cfg.Manager.TLSInsecure = strings.EqualFold(value, "true") || value == "1"
	}
}

func LoadBytes(data []byte) (*Config, error) {
	cfg := defaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	c.Manager.Address = strings.TrimSpace(c.Manager.Address)
	if c.Manager.Address == "" {
		return fmt.Errorf("manager.address is required")
	}

	if c.Podman.Timeout <= 0 {
		c.Podman.Timeout = 10 * time.Second
	}
	if c.Heartbeat.Interval <= 0 {
		c.Heartbeat.Interval = 15 * time.Second
	}
	if c.Heartbeat.Timeout <= 0 {
		c.Heartbeat.Timeout = 5 * time.Second
	}
	if c.Reconnect.InitialBackoff <= 0 {
		c.Reconnect.InitialBackoff = time.Second
	}
	if c.Reconnect.MaxBackoff <= 0 {
		c.Reconnect.MaxBackoff = time.Minute
	}
	if c.Reconnect.Multiplier <= 0 {
		c.Reconnect.Multiplier = 2.0
	}
	if c.Reconnect.MaxBackoff < c.Reconnect.InitialBackoff {
		return fmt.Errorf("reconnect.max_backoff must be greater than or equal to reconnect.initial_backoff")
	}

	c.Log.Level = strings.ToLower(strings.TrimSpace(c.Log.Level))
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	switch c.Log.Level {
	case "debug", "info", "warn", "warning", "error":
	default:
		return fmt.Errorf("log.level must be debug, info, warn, or error")
	}

	c.Log.Format = strings.ToLower(strings.TrimSpace(c.Log.Format))
	if c.Log.Format == "" {
		c.Log.Format = "json"
	}
	switch c.Log.Format {
	case "json", "text":
	default:
		return fmt.Errorf("log.format must be json or text")
	}

	return nil
}

func defaultConfig() *Config {
	return &Config{
		Manager: ManagerConfig{
			TLS: true,
		},
		Podman: PodmanConfig{
			Timeout: 10 * time.Second,
		},
		Heartbeat: HeartbeatConfig{
			Interval: 15 * time.Second,
			Timeout:  5 * time.Second,
		},
		Reconnect: ReconnectConfig{
			InitialBackoff: time.Second,
			MaxBackoff:     time.Minute,
			Multiplier:     2.0,
		},
		Log: LogConfig{
			Level:  "info",
			Format: "json",
		},
	}
}
