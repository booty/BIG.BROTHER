package media

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"
)

// Manager owns the MediaMTX process and the short-lived media sessions created
// through the same-origin WHIP/WHEP proxy.
type Manager struct {
	options Options
	logger  *slog.Logger

	mu       sync.Mutex
	process  processState
	streams  map[string]string
	sessions map[string]map[string]struct{}
}

type processState struct {
	cmd     *exec.Cmd
	done    <-chan struct{}
	waitErr error
	running bool
}

// Options describes the packaged MediaMTX runtime. The defaults match the
// checked-in v1.21.2 configuration and validated loopback endpoints.
type Options struct {
	BinaryPath      string
	ConfigPath      string
	HTTPURL         string
	ControlAPIURL   string
	HTTPClient      *http.Client
	ReadyTimeout    time.Duration
	ShutdownTimeout time.Duration
	PollInterval    time.Duration
	Stdout          io.Writer
	Stderr          io.Writer
	Logger          *slog.Logger
}

func New(options Options) *Manager {
	if options.HTTPURL == "" {
		options.HTTPURL = "http://127.0.0.1:8889"
	}
	if options.ControlAPIURL == "" {
		options.ControlAPIURL = "http://127.0.0.1:9997"
	}
	if options.HTTPClient == nil {
		options.HTTPClient = &http.Client{Timeout: 2 * time.Second}
	}
	if options.ReadyTimeout <= 0 {
		options.ReadyTimeout = 10 * time.Second
	}
	if options.ShutdownTimeout <= 0 {
		options.ShutdownTimeout = 5 * time.Second
	}
	if options.PollInterval <= 0 {
		options.PollInterval = 100 * time.Millisecond
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		options:  options,
		logger:   logger,
		streams:  make(map[string]string),
		sessions: make(map[string]map[string]struct{}),
	}
}

// AssignStream returns a stable, opaque ID for one room connection.
func (m *Manager) AssignStream(sessionID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing := m.streams[sessionID]; existing != "" {
		return existing, nil
	}
	for {
		var bytes [12]byte
		if _, err := rand.Read(bytes[:]); err != nil {
			return "", fmt.Errorf("create a media stream ID: %w", err)
		}
		id := hex.EncodeToString(bytes[:])
		collision := false
		for _, current := range m.streams {
			if current == id {
				collision = true
				break
			}
		}
		if !collision {
			m.streams[sessionID] = id
			return id, nil
		}
	}
}

// ReleaseStream removes a participant's association and deletes its owned
// WHIP/WHEP resources from MediaMTX. MediaMTX drops the dynamic path after its
// final publisher/reader session is removed.
func (m *Manager) ReleaseStream(sessionID string) {
	m.mu.Lock()
	owner := m.streams[sessionID]
	delete(m.streams, sessionID)
	resources := make([]string, 0, len(m.sessions[owner]))
	for resource := range m.sessions[owner] {
		resources = append(resources, resource)
	}
	delete(m.sessions, owner)
	m.mu.Unlock()

	for _, resource := range resources {
		request, err := http.NewRequest(http.MethodDelete, strings.TrimRight(m.options.HTTPURL, "/")+resource, nil)
		if err != nil {
			m.logger.Warn("could not create MediaMTX session cleanup request", "resource", resource, "error", err)
			continue
		}
		response, err := m.options.HTTPClient.Do(request)
		if err != nil {
			m.logger.Warn("could not clean up MediaMTX session", "resource", resource, "error", err)
			continue
		}
		response.Body.Close()
		if response.StatusCode != http.StatusNotFound && (response.StatusCode < 200 || response.StatusCode >= 300) {
			m.logger.Warn("MediaMTX session cleanup returned an error", "resource", resource, "status", response.StatusCode)
		}
	}
}

func (m *Manager) recordSession(ownerID, resource string) {
	if !validStreamID(ownerID) || !validSessionResource(resource) {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sessions[ownerID] == nil {
		m.sessions[ownerID] = make(map[string]struct{})
	}
	m.sessions[ownerID][resource] = struct{}{}
}

func (m *Manager) forgetSession(ownerID, resource string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions[ownerID], resource)
	if len(m.sessions[ownerID]) == 0 {
		delete(m.sessions, ownerID)
	}
}

func validStreamID(value string) bool {
	if len(value) != 24 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 12
}

func validSessionResource(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/") {
		return false
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	return len(parts) == 3 && validStreamID(parts[0]) && (parts[1] == "whip" || parts[1] == "whep") && parts[2] != "" && path.Clean(parsed.Path) == parsed.Path
}

// Status describes media readiness independently of the HTTP/room liveness.
type Status struct {
	Available bool   `json:"available"`
	Message   string `json:"message,omitempty"`
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.process.running {
		message := "MediaMTX is not running"
		if m.process.waitErr != nil {
			message = fmt.Sprintf("MediaMTX exited: %v", m.process.waitErr)
		}
		return Status{Message: message}
	}
	select {
	case <-m.process.done:
		m.process.running = false
		if m.process.waitErr == nil {
			return Status{Message: "MediaMTX exited unexpectedly"}
		}
		return Status{Message: fmt.Sprintf("MediaMTX exited: %v", m.process.waitErr)}
	default:
		return Status{Available: true}
	}
}

// StatusHandler reports media health without changing /healthz liveness.
func (m *Manager) StatusHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(m.Status())
	})
}
