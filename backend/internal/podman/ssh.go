package podman

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/brdweb/podman-manager/internal/config"
)

type SSHPool struct {
	mu          sync.RWMutex
	connections map[string]*sshConn
	sshConfig   *ssh.ClientConfig
	hostConfigs map[string]config.HostConfig
	logger      *slog.Logger
}

type sshConn struct {
	mu     sync.Mutex
	client *ssh.Client
	host   config.HostConfig
	logger *slog.Logger
}

type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

func NewSSHPool(cfg *config.Config, logger *slog.Logger) (*SSHPool, error) {
	if logger == nil {
		logger = slog.Default()
	}

	keyData, err := os.ReadFile(cfg.SSH.KeyPath)
	if err != nil {
		logger.Error("failed to read SSH key", "key_path", cfg.SSH.KeyPath, "error", err)
		return nil, fmt.Errorf("reading SSH key %s: %w", cfg.SSH.KeyPath, err)
	}

	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		logger.Error("failed to parse SSH key", "key_path", cfg.SSH.KeyPath, "error", err)
		return nil, fmt.Errorf("parsing SSH key: %w", err)
	}

	sshCfg := &ssh.ClientConfig{
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         cfg.SSH.ConnectTimeout,
	}

	hostMap := make(map[string]config.HostConfig)
	for _, h := range cfg.Hosts {
		sshCfg.User = h.User
		hostMap[h.Name] = h
	}

	pool := &SSHPool{
		connections: make(map[string]*sshConn),
		sshConfig:   sshCfg,
		hostConfigs: hostMap,
		logger:      logger,
	}

	return pool, nil
}

func (p *SSHPool) GetConnection(hostName string) (*sshConn, error) {
	p.mu.RLock()
	conn, exists := p.connections[hostName]
	p.mu.RUnlock()

	if exists && conn.isAlive() {
		return conn, nil
	}

	return p.reconnect(hostName)
}

func (p *SSHPool) reconnect(hostName string) (*sshConn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if conn, exists := p.connections[hostName]; exists && conn.isAlive() {
		return conn, nil
	}

	hostCfg, exists := p.hostConfigs[hostName]
	if !exists {
		p.logger.Error("unknown host for SSH reconnect", "host", hostName)
		return nil, fmt.Errorf("unknown host: %s", hostName)
	}

	sshCfg := *p.sshConfig
	sshCfg.User = hostCfg.User

	client, err := ssh.Dial("tcp", hostCfg.SSHAddress(), &sshCfg)
	if err != nil {
		p.logger.Error("SSH connection failed", "host", hostName, "address", hostCfg.SSHAddress(), "error", err)
		return nil, fmt.Errorf("SSH connection to %s (%s): %w", hostName, hostCfg.SSHAddress(), err)
	}

	conn := &sshConn{
		client: client,
		host:   hostCfg,
		logger: p.logger,
	}
	p.connections[hostName] = conn

	return conn, nil
}

func (c *sshConn) isAlive() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client == nil {
		return false
	}

	_, _, err := c.client.SendRequest("keepalive@openssh.com", true, nil)
	return err == nil
}

func (c *sshConn) Run(ctx context.Context, command string) (*CommandResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client == nil {
		c.logger.Error("SSH command attempted on closed connection", "host", c.host.Name, "command", command)
		return nil, fmt.Errorf("SSH connection is closed")
	}

	session, err := c.client.NewSession()
	if err != nil {
		c.logger.Error("failed to create SSH session", "host", c.host.Name, "command", command, "error", err)
		return nil, fmt.Errorf("creating SSH session: %w", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	done := make(chan error, 1)
	start := time.Now()

	go func() {
		done <- session.Run(command)
	}()

	select {
	case <-ctx.Done():
		session.Signal(ssh.SIGTERM)
		c.logger.Error("SSH command canceled", "host", c.host.Name, "command", command, "error", ctx.Err())
		return nil, ctx.Err()
	case err := <-done:
		result := &CommandResult{
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			Duration: time.Since(start),
		}

		if err != nil {
			if exitErr, ok := err.(*ssh.ExitError); ok {
				result.ExitCode = exitErr.ExitStatus()
				c.logger.Error("SSH command exited with non-zero status", "host", c.host.Name, "command", command, "exit_code", result.ExitCode, "stderr", strings.TrimSpace(result.Stderr))
			} else {
				c.logger.Error("SSH command execution failed", "host", c.host.Name, "command", command, "error", err)
				return result, fmt.Errorf("running command: %w", err)
			}
		}

		return result, nil
	}
}

func (p *SSHPool) Ping(hostName string) (time.Duration, error) {
	hostCfg, exists := p.hostConfigs[hostName]
	if !exists {
		return 0, fmt.Errorf("unknown host: %s", hostName)
	}

	start := time.Now()
	conn, err := net.DialTimeout("tcp", hostCfg.SSHAddress(), 3*time.Second)
	if err != nil {
		return 0, err
	}
	conn.Close()
	return time.Since(start), nil
}

func (p *SSHPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for name, conn := range p.connections {
		conn.mu.Lock()
		if conn.client != nil {
			conn.client.Close()
		}
		conn.mu.Unlock()
		delete(p.connections, name)
	}
}

func (p *SSHPool) HostNames() []string {
	names := make([]string, 0, len(p.hostConfigs))
	for name := range p.hostConfigs {
		names = append(names, name)
	}
	return names
}

func (p *SSHPool) HostConfig(name string) (config.HostConfig, bool) {
	h, ok := p.hostConfigs[name]
	return h, ok
}
