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
	Server             ServerConfig  `yaml:"server"`
	SSH                SSHConfig     `yaml:"ssh"`
	Hosts              []HostConfig  `yaml:"hosts"`
	Auth               AuthConfig    `yaml:"auth,omitempty"`
	EnableEventsStream bool          `yaml:"enable_events_stream"`
	CacheTTL           time.Duration `yaml:"cache_ttl,omitempty"`
}

type ServerConfig struct {
	Port int    `yaml:"port"`
	Bind string `yaml:"bind"`
}

type SSHConfig struct {
	KeyPath               string        `yaml:"key_path"`
	ConnectTimeout        time.Duration `yaml:"connect_timeout"`
	KeepaliveInterval     time.Duration `yaml:"keepalive_interval"`
	StrictHostKeyChecking string        `yaml:"ssh_strict_host_key_checking"`
}

type HostConfig struct {
	Name    string `yaml:"name"`
	Address string `yaml:"address"`
	Port    int    `yaml:"port"`
	User    string `yaml:"user"`
	Mode    string `yaml:"mode"` // "rootful" or "rootless"
}

type AuthConfig struct {
	Enabled      bool          `yaml:"enabled"`
	Username     string        `yaml:"username,omitempty"`
	PasswordHash string        `yaml:"password_hash,omitempty"`
	SessionTTL   time.Duration `yaml:"session_ttl,omitempty"`
}

func (h HostConfig) IsRootful() bool {
	return h.Mode == "rootful"
}

func (h HostConfig) SSHAddress() string {
	port := h.Port
	if port == 0 {
		port = 22
	}
	return fmt.Sprintf("%s:%d", h.Address, port)
}

func Load(path string) (*Config, error) {
	expanded := ExpandPath(path)

	data, err := os.ReadFile(expanded)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", expanded, err)
	}

	cfg, err := LoadBytes(data)
	if err != nil {
		return nil, err
	}

	cfg.SSH.KeyPath = ExpandPath(cfg.SSH.KeyPath)
	return cfg, nil
}

func LoadBytes(data []byte) (*Config, error) {
	cfg := defaultConfig()

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	cfg.SSH.KeyPath = ExpandPath(cfg.SSH.KeyPath)
	return cfg, nil
}

func Marshal(cfg *Config) ([]byte, error) {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("serializing config: %w", err)
	}
	return data, nil
}

func defaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: 18734,
			Bind: "127.0.0.1",
		},
		SSH: SSHConfig{
			ConnectTimeout:        5 * time.Second,
			KeepaliveInterval:     30 * time.Second,
			StrictHostKeyChecking: "accept-new",
		},
		Auth: AuthConfig{
			SessionTTL: 12 * time.Hour,
		},
		EnableEventsStream: true,
		CacheTTL:           3 * time.Second,
	}
}

func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535, got %d", c.Server.Port)
	}

	if len(c.Hosts) == 0 {
		return fmt.Errorf("at least one host must be configured")
	}

	names := make(map[string]bool)
	for i, h := range c.Hosts {
		if h.Name == "" {
			return fmt.Errorf("hosts[%d].name is required", i)
		}
		if names[h.Name] {
			return fmt.Errorf("duplicate host name: %s", h.Name)
		}
		names[h.Name] = true

		if h.Address == "" {
			return fmt.Errorf("hosts[%d].address is required", i)
		}
		if h.User == "" {
			return fmt.Errorf("hosts[%d].user is required", i)
		}
		if h.Mode != "rootful" && h.Mode != "rootless" {
			return fmt.Errorf("hosts[%d].mode must be 'rootful' or 'rootless', got '%s'", i, h.Mode)
		}

		if h.Port == 0 {
			c.Hosts[i].Port = 22
		}
	}

	if c.SSH.KeyPath == "" {
		return fmt.Errorf("ssh.key_path is required")
	}

	hostKeyMode := strings.ToLower(strings.TrimSpace(c.SSH.StrictHostKeyChecking))
	if hostKeyMode == "" {
		hostKeyMode = "accept-new"
	}

	switch hostKeyMode {
	case "strict", "accept-new", "off":
		c.SSH.StrictHostKeyChecking = hostKeyMode
	default:
		return fmt.Errorf("ssh.ssh_strict_host_key_checking must be 'strict', 'accept-new', or 'off', got '%s'", c.SSH.StrictHostKeyChecking)
	}

	if c.Auth.SessionTTL <= 0 {
		c.Auth.SessionTTL = 12 * time.Hour
	}

	if c.Auth.Enabled {
		if c.Auth.Username == "" {
			return fmt.Errorf("auth.username is required when auth is enabled")
		}
		if c.Auth.PasswordHash == "" {
			return fmt.Errorf("auth.password_hash is required when auth is enabled")
		}
	}

	if c.CacheTTL <= 0 {
		c.CacheTTL = 3 * time.Second
	}

	return nil
}

func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
