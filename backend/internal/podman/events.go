package podman

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/brdweb/podman-manager/internal/config"
)

type PodmanEvent struct {
	Host  string         `json:"host"`
	Event map[string]any `json:"event"`
}

type EventStream struct {
	pool   *SSHPool
	logger *slog.Logger

	events chan PodmanEvent

	mu            sync.Mutex
	subscriptions map[string]*eventSubscription
	closed        bool
}

type eventSubscription struct {
	host   string
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

func NewEventStream(pool *SSHPool, logger *slog.Logger) *EventStream {
	if logger == nil {
		logger = slog.Default()
	}

	return &EventStream{
		pool:          pool,
		logger:        logger,
		events:        make(chan PodmanEvent, 256),
		subscriptions: make(map[string]*eventSubscription),
	}
}

func (s *EventStream) Events() <-chan PodmanEvent {
	return s.events
}

func (s *EventStream) Subscribe(host string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("event stream is closed")
	}

	if _, ok := s.subscriptions[host]; ok {
		return nil
	}

	if _, ok := s.pool.HostConfig(host); !ok {
		return fmt.Errorf("unknown host: %s", host)
	}

	ctx, cancel := context.WithCancel(context.Background())
	sub := &eventSubscription{
		host:   host,
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
	}

	s.subscriptions[host] = sub
	go s.runSubscription(sub)
	return nil
}

func (s *EventStream) Unsubscribe(host string) {
	s.mu.Lock()
	sub, ok := s.subscriptions[host]
	if ok {
		delete(s.subscriptions, host)
	}
	s.mu.Unlock()

	if !ok {
		return
	}

	sub.cancel()
	<-sub.done
}

func (s *EventStream) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true

	subs := make([]*eventSubscription, 0, len(s.subscriptions))
	for host, sub := range s.subscriptions {
		delete(s.subscriptions, host)
		subs = append(subs, sub)
	}
	s.mu.Unlock()

	for _, sub := range subs {
		sub.cancel()
		<-sub.done
	}

	close(s.events)
}

func (s *EventStream) runSubscription(sub *eventSubscription) {
	defer close(sub.done)

	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		if sub.ctx.Err() != nil {
			return
		}

		err := s.streamHostEvents(sub.ctx, sub.host)
		if sub.ctx.Err() != nil {
			return
		}

		s.logger.Warn("podman event stream disconnected, retrying", "host", sub.host, "error", err, "retry_in", backoff.String())

		timer := time.NewTimer(backoff)
		select {
		case <-sub.ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func (s *EventStream) streamHostEvents(ctx context.Context, host string) error {
	conn, err := s.pool.GetConnection(host)
	if err != nil {
		return err
	}

	hostCfg, ok := s.pool.HostConfig(host)
	if !ok {
		return fmt.Errorf("unknown host: %s", host)
	}

	cmd := fmt.Sprintf("%s events --format json --stream", podmanCmdForHost(hostCfg))
	session, stdout, stderr, err := openStreamingSession(conn, cmd)
	if err != nil {
		return err
	}
	defer session.Close()

	go func() {
		<-ctx.Done()
		_ = session.Signal(ssh.SIGTERM)
		_ = session.Close()
	}()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			s.logger.Warn("failed to parse podman event JSON", "host", host, "error", err)
			continue
		}

		event := PodmanEvent{Host: host, Event: payload}
		select {
		case s.events <- event:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading event stream: %w", err)
	}

	if waitErr := session.Wait(); waitErr != nil && ctx.Err() == nil {
		return fmt.Errorf("event stream process ended: %w (stderr: %s)", waitErr, strings.TrimSpace(stderr.String()))
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	if stderr.Len() > 0 {
		return fmt.Errorf("event stream ended: %s", strings.TrimSpace(stderr.String()))
	}

	return fmt.Errorf("event stream ended unexpectedly")
}

func openStreamingSession(conn *sshConn, command string) (*ssh.Session, *bufio.Reader, *bytes.Buffer, error) {
	conn.mu.Lock()
	client := conn.client
	conn.mu.Unlock()

	if client == nil {
		return nil, nil, nil, fmt.Errorf("SSH connection is closed")
	}

	session, err := client.NewSession()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("creating SSH session: %w", err)
	}

	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		return nil, nil, nil, fmt.Errorf("opening stdout pipe: %w", err)
	}

	stderr := &bytes.Buffer{}
	session.Stderr = stderr

	if err := session.Start(command); err != nil {
		session.Close()
		return nil, nil, nil, fmt.Errorf("starting SSH command: %w", err)
	}

	return session, bufio.NewReader(stdoutPipe), stderr, nil
}

func podmanCmdForHost(hostCfg config.HostConfig) string {
	if hostCfg.IsRootful() {
		return "sudo podman"
	}
	return "podman"
}
