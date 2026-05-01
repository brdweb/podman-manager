package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	connect "github.com/brdweb/podman-manager/agent/internal"
	"github.com/brdweb/podman-manager/agent/internal/config"
	"github.com/brdweb/podman-manager/agent/internal/podman"
)

var version = "2026.05.01"

func main() {
	configPath := flag.String("config", "/etc/podman-agent/config.yaml", "path to configuration file")
	managerAddress := flag.String("manager-address", "", "manager gRPC address override")
	token := flag.String("token", "", "enrollment token override")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("podman-agent", version)
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
		logger.Error("failed to load config", "config_path", *configPath, "error", err)
		os.Exit(1)
	}

	if *managerAddress != "" {
		cfg.Manager.Address = *managerAddress
	}
	if *token != "" {
		cfg.Agent.Token = *token
	}
	if err := cfg.Validate(); err != nil {
		logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
		logger.Error("invalid config", "error", err)
		os.Exit(1)
	}

	logger := newLogger(cfg.Log)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("Agent starting", "version", version, "manager_address", cfg.Manager.Address)
	if err := run(ctx, *configPath, cfg, logger); err != nil {
		logger.Error("agent stopped with error", "error", err)
		os.Exit(1)
	}
	logger.Info("agent stopped")
}

func run(ctx context.Context, configPath string, cfg *config.Config, logger *slog.Logger) error {
	podmanClient, err := podman.NewClient(cfg.Podman.SocketPath, cfg.Podman.Timeout)
	if err != nil {
		return err
	}

	manager := connect.NewManager(cfg, podmanClient, logger)
	enrolled, err := manager.EnsureEnrolled(ctx, version)
	if err != nil {
		return err
	}
	if enrolled {
		if err := config.Save(configPath, cfg); err != nil {
			return err
		}
		logger.Info("agent enrolled", "agent_id", cfg.Agent.ID)
	}

	if err := manager.Connect(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

func newLogger(cfg config.LogConfig) *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	opts := &slog.HandlerOptions{Level: level}
	if strings.ToLower(cfg.Format) == "text" {
		return slog.New(slog.NewTextHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}
