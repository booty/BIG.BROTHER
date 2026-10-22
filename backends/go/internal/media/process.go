package media

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// ResolvePaths finds the media files beside a packaged application or in the
// current source checkout. It never downloads runtime dependencies.
func ResolvePaths(executablePath, workingDir string) (binary, config string, err error) {
	candidates := []string{}
	if executablePath != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(executablePath), "media"))
	}
	if workingDir != "" {
		candidates = append(candidates,
			filepath.Join(workingDir, "media"),
			filepath.Join(workingDir, "..", "..", "media"),
		)
	}
	for _, directory := range candidates {
		candidateBinary := filepath.Join(directory, "mediamtx")
		candidateConfig := filepath.Join(directory, "mediamtx.yml")
		binaryInfo, binaryErr := os.Stat(candidateBinary)
		configInfo, configErr := os.Stat(candidateConfig)
		if binaryErr == nil && binaryInfo.Mode().IsRegular() && binaryInfo.Mode().Perm()&0o111 != 0 && configErr == nil && configInfo.Mode().IsRegular() {
			return candidateBinary, candidateConfig, nil
		}
	}
	return "", "", errors.New("packaged MediaMTX binary/configuration is missing; run scripts/build --target OS/ARCH and launch the resulting tmp/build/OS-ARCH/bigbrother")
}

// Start launches MediaMTX and waits for the loopback Control API readiness
// endpoint before the application begins accepting browser connections.
func (m *Manager) Start(ctx context.Context) error {
	if m.options.BinaryPath == "" || m.options.ConfigPath == "" {
		return errors.New("MediaMTX binary and configuration paths are required")
	}
	if info, err := os.Stat(m.options.BinaryPath); err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("MediaMTX binary %q is missing or not executable; rebuild the package with scripts/build", m.options.BinaryPath)
	}
	if info, err := os.Stat(m.options.ConfigPath); err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("MediaMTX configuration %q is missing; restore the packaged media/mediamtx.yml", m.options.ConfigPath)
	}

	m.mu.Lock()
	if m.process.running {
		m.mu.Unlock()
		return errors.New("MediaMTX is already running")
	}
	command := exec.Command(m.options.BinaryPath, m.options.ConfigPath)
	command.Dir = filepath.Dir(m.options.ConfigPath)
	command.Stdout = m.options.Stdout
	if command.Stdout == nil {
		command.Stdout = os.Stdout
	}
	command.Stderr = m.options.Stderr
	if command.Stderr == nil {
		command.Stderr = os.Stderr
	}
	if err := command.Start(); err != nil {
		m.mu.Unlock()
		return fmt.Errorf("start MediaMTX: %w", err)
	}
	done := make(chan struct{})
	m.process = processState{cmd: command, done: done, running: true}
	m.mu.Unlock()
	go func() {
		waitErr := command.Wait()
		m.mu.Lock()
		m.process.waitErr = waitErr
		m.mu.Unlock()
		close(done)
	}()

	deadline := time.NewTimer(m.options.ReadyTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(m.options.PollInterval)
	defer ticker.Stop()
	for {
		if err := m.checkReady(ctx); err == nil {
			select {
			case <-done:
				m.mu.Lock()
				exitErr := m.process.waitErr
				m.process.running = false
				m.mu.Unlock()
				return exitedBeforeReadiness(exitErr)
			default:
				return nil
			}
		}
		select {
		case <-done:
			m.mu.Lock()
			exitErr := m.process.waitErr
			m.process.running = false
			m.mu.Unlock()
			return exitedBeforeReadiness(exitErr)
		case <-ctx.Done():
			_ = m.Stop()
			return fmt.Errorf("wait for MediaMTX readiness: %w", ctx.Err())
		case <-deadline.C:
			_ = m.Stop()
			return fmt.Errorf("MediaMTX did not become ready within %s at %s/v3/paths/list", m.options.ReadyTimeout, m.options.ControlAPIURL)
		case <-ticker.C:
		}
	}
}

func exitedBeforeReadiness(exitErr error) error {
	if exitErr == nil {
		return errors.New("MediaMTX exited before becoming ready")
	}
	return fmt.Errorf("MediaMTX exited before becoming ready: %w", exitErr)
}

func (m *Manager) checkReady(ctx context.Context) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, m.options.ControlAPIURL+"/v3/paths/list", nil)
	if err != nil {
		return err
	}
	response, err := m.options.HTTPClient.Do(request)
	if err != nil {
		return err
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Control API returned HTTP %d", response.StatusCode)
	}
	return nil
}

// Stop removes tracked WHIP/WHEP resources, asks MediaMTX to exit gracefully,
// and waits for its child process to be reaped (killing it after a timeout).
func (m *Manager) Stop() error {
	m.mu.Lock()
	if !m.process.running || m.process.cmd == nil {
		m.mu.Unlock()
		return nil
	}
	command := m.process.cmd
	done := m.process.done
	sessions := make([]string, 0)
	for owner := range m.sessions {
		for resource := range m.sessions[owner] {
			sessions = append(sessions, resource)
		}
	}
	for owner := range m.sessions {
		delete(m.sessions, owner)
	}
	for sessionID := range m.streams {
		delete(m.streams, sessionID)
	}
	m.mu.Unlock()

	for _, resource := range sessions {
		request, err := http.NewRequest(http.MethodDelete, m.options.HTTPURL+resource, nil)
		if err != nil {
			continue
		}
		response, err := m.options.HTTPClient.Do(request)
		if err == nil {
			response.Body.Close()
		}
	}

	if command.Process == nil {
		return nil
	}
	if err := command.Process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("send termination signal to MediaMTX: %w", err)
	}
	timer := time.NewTimer(m.options.ShutdownTimeout)
	defer timer.Stop()
	select {
	case <-done:
		m.mu.Lock()
		m.process.running = false
		m.mu.Unlock()
		return nil
	case <-timer.C:
		if err := command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return fmt.Errorf("kill unresponsive MediaMTX: %w", err)
		}
		<-done
		m.mu.Lock()
		m.process.running = false
		m.mu.Unlock()
		return nil
	}
}
