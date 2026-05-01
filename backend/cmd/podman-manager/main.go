package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/brdweb/podman-manager/internal/api"
	"github.com/brdweb/podman-manager/internal/config"
)

var version = "2026.05.01"

func main() {
	configPath := flag.String("config", "config.yaml", "path to configuration file")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("podman-manager", version)
		os.Exit(0)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("failed to load config", "config_path", *configPath, "error", err)
		os.Exit(1)
	}

	logger.Info("podman-manager starting", "version", version)
	logger.Info("configured hosts", "count", len(cfg.Hosts))
	for _, h := range cfg.Hosts {
		logger.Info("configured host", "name", h.Name, "user", h.User, "address", h.Address, "mode", h.Mode)
	}

	server, err := api.NewServer(*configPath, cfg, logger, version)
	if err != nil {
		logger.Error("failed to initialize API server", "error", err)
		os.Exit(1)
	}
	defer server.Close()

	addr := fmt.Sprintf("%s:%d", cfg.Server.Bind, cfg.Server.Port)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      server.Handler(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		logger.Info("API server listening", "address", addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Info("received signal, shutting down", "signal", sig.String())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}

	server.Close()
	logger.Info("podman-manager stopped")
}
