package inference

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type State string

const (
	StateStopped  State = "stopped"
	StateStarting State = "starting"
	StateReady    State = "ready"
	StateStopping State = "stopping"
	StateFailed   State = "failed"
)

type Config struct {
	BinaryPath      string
	ModelPath       string
	Host            string
	Port            int
	Threads         int
	ContextSize     int
	StartupTimeout  time.Duration
	ShutdownTimeout time.Duration
}

type Status struct {
	State     State     `json:"state"`
	Endpoint  string    `json:"endpoint"`
	PID       int       `json:"pid,omitempty"`
	StartedAt time.Time `json:"started_at,omitempty"`
	LastError string    `json:"last_error,omitempty"`
}

type Supervisor struct {
	cfg    Config
	client *http.Client

	mu        sync.RWMutex
	state     State
	cmd       *exec.Cmd
	done      chan struct{}
	waitErr   error
	startedAt time.Time
	lastError string
}

func NewSupervisor(cfg Config) *Supervisor {
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.Port == 0 {
		cfg.Port = 8081
	}
	if cfg.Threads == 0 {
		cfg.Threads = 2
	}
	if cfg.ContextSize == 0 {
		cfg.ContextSize = 2048
	}
	if cfg.StartupTimeout == 0 {
		cfg.StartupTimeout = 2 * time.Minute
	}
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = 10 * time.Second
	}
	return &Supervisor{
		cfg:    cfg,
		client: &http.Client{Timeout: time.Second},
		state:  StateStopped,
	}
}

func (s *Supervisor) Endpoint() string {
	return "http://" + net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))
}

func (s *Supervisor) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	status := Status{State: s.state, Endpoint: s.Endpoint(), StartedAt: s.startedAt, LastError: s.lastError}
	if s.cmd != nil && s.cmd.Process != nil {
		status.PID = s.cmd.Process.Pid
	}
	return status
}

func (s *Supervisor) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.state != StateStopped && s.state != StateFailed {
		s.mu.Unlock()
		return fmt.Errorf("cannot start native runtime while state is %q", s.state)
	}
	s.state = StateStarting
	s.lastError = ""
	s.waitErr = nil
	s.mu.Unlock()

	if err := s.validate(); err != nil {
		s.setFailed(err)
		return err
	}

	args := []string{
		"--model", s.cfg.ModelPath,
		"--host", s.cfg.Host,
		"--port", strconv.Itoa(s.cfg.Port),
		"--threads", strconv.Itoa(s.cfg.Threads),
		"--threads-batch", strconv.Itoa(s.cfg.Threads),
		"--ctx-size", strconv.Itoa(s.cfg.ContextSize),
		"--n-gpu-layers", "0",
	}
	cmd := exec.Command(s.cfg.BinaryPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		s.setFailed(fmt.Errorf("start llama-server: %w", err))
		return err
	}

	done := make(chan struct{})
	s.mu.Lock()
	s.cmd = cmd
	s.done = done
	s.startedAt = time.Now().UTC()
	s.state = StateStarting
	s.mu.Unlock()

	go func() {
		err := cmd.Wait()
		s.mu.Lock()
		s.waitErr = err
		if s.state != StateStopping {
			s.state = StateFailed
			if err != nil {
				s.lastError = err.Error()
			} else {
				s.lastError = "llama-server exited unexpectedly"
			}
		}
		s.mu.Unlock()
		close(done)
	}()

	deadline := time.NewTimer(s.cfg.StartupTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		if err := s.probe(ctx); err == nil {
			s.mu.Lock()
			if s.state == StateStarting {
				s.state = StateReady
			}
			state := s.state
			s.mu.Unlock()
			if state == StateReady {
				return nil
			}
			return errors.New("native runtime exited before becoming ready")
		}

		select {
		case <-ctx.Done():
			_ = s.Shutdown()
			return ctx.Err()
		case <-deadline.C:
			err := fmt.Errorf("native runtime did not become healthy within %s", s.cfg.StartupTimeout)
			s.setFailed(err)
			_ = s.Shutdown()
			return err
		case <-done:
			s.mu.RLock()
			waitErr := s.waitErr
			s.mu.RUnlock()
			if waitErr == nil {
				return errors.New("native runtime exited before becoming ready")
			}
			return fmt.Errorf("native runtime exited before becoming ready: %w", waitErr)
		case <-ticker.C:
		}
	}
}

func (s *Supervisor) Shutdown() error {
	s.mu.Lock()
	cmd, done := s.cmd, s.done
	if cmd == nil || cmd.Process == nil || done == nil {
		s.state = StateStopped
		s.mu.Unlock()
		return nil
	}
	if s.state == StateStopped {
		s.mu.Unlock()
		return nil
	}
	s.state = StateStopping
	s.mu.Unlock()

	_ = cmd.Process.Signal(os.Interrupt)
	timer := time.NewTimer(s.cfg.ShutdownTimeout)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return fmt.Errorf("kill llama-server after shutdown timeout: %w", err)
		}
		<-done
	}

	s.mu.Lock()
	s.state = StateStopped
	s.cmd = nil
	s.done = nil
	s.mu.Unlock()
	return nil
}

func (s *Supervisor) HealthHandler(w http.ResponseWriter, _ *http.Request) {
	status := s.Status()
	code := http.StatusServiceUnavailable
	if status.State == StateReady {
		code = http.StatusOK
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(status)
}

func (s *Supervisor) validate() error {
	if strings.TrimSpace(s.cfg.BinaryPath) == "" {
		return errors.New("llama-server binary path is required")
	}
	if strings.TrimSpace(s.cfg.ModelPath) == "" {
		return errors.New("model path is required")
	}
	if !isLoopbackHost(s.cfg.Host) {
		return fmt.Errorf("native runtime must bind to loopback during M1, got %q", s.cfg.Host)
	}
	if s.cfg.Port < 1 || s.cfg.Port > 65535 {
		return fmt.Errorf("invalid native runtime port %d", s.cfg.Port)
	}
	if s.cfg.Threads < 1 {
		return errors.New("threads must be at least 1")
	}
	if s.cfg.ContextSize < 1 {
		return errors.New("context size must be at least 1")
	}
	binary, err := os.Stat(s.cfg.BinaryPath)
	if err != nil {
		return fmt.Errorf("stat llama-server binary: %w", err)
	}
	if binary.IsDir() || binary.Mode()&0111 == 0 {
		return errors.New("llama-server path is not an executable file")
	}
	model, err := os.Stat(s.cfg.ModelPath)
	if err != nil {
		return fmt.Errorf("stat model: %w", err)
	}
	if !model.Mode().IsRegular() {
		return errors.New("model path is not a regular file")
	}
	return nil
}

func (s *Supervisor) probe(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.Endpoint()+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("native health returned %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return err
	}
	var result struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return err
	}
	if result.Status != "ok" {
		return fmt.Errorf("native health status is %q", result.Status)
	}
	return nil
}

func (s *Supervisor) setFailed(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = StateFailed
	s.lastError = err.Error()
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
