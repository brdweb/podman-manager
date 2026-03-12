package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/brdweb/podman-manager/internal/api"
	"github.com/brdweb/podman-manager/internal/config"
	"github.com/brdweb/podman-manager/internal/podman"
)

var version = "dev"

func main() {
	configPath := flag.String("config", "config.yaml", "path to configuration file")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("podman-manager", version)
		os.Exit(0)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	log.Printf("podman-manager %s starting", version)
	log.Printf("configured %d host(s)", len(cfg.Hosts))
	for _, h := range cfg.Hosts {
		log.Printf("  - %s (%s@%s, %s)", h.Name, h.User, h.Address, h.Mode)
	}

	pool, err := podman.NewSSHPool(cfg)
	if err != nil {
		log.Fatalf("failed to initialize SSH pool: %v", err)
	}
	defer pool.Close()

	client := podman.NewClient(pool)
	server := api.NewServer(client)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Bind, cfg.Server.Port)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      server.Handler(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("API server listening on %s", addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("received %s, shutting down", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}

	pool.Close()
	log.Println("podman-manager stopped")
}
