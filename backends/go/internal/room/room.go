package room

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type Participant struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	AudioEnabled bool   `json:"audioEnabled"`
	VideoEnabled bool   `json:"videoEnabled"`
	StreamID     string `json:"streamId"`
	PublishURL   string `json:"publishUrl"`
	ReadURL      string `json:"readUrl"`
	StreamReady  bool   `json:"streamReady"`
}

type Options struct {
	PingInterval  time.Duration
	PongTimeout   time.Duration
	Logger        *slog.Logger
	AssignStream  func(sessionID string) (string, error)
	ReleaseStream func(sessionID string)
}

type Room struct {
	mu            sync.Mutex
	participants  map[string]*participantSession
	pingInterval  time.Duration
	pongTimeout   time.Duration
	logger        *slog.Logger
	assignStream  func(sessionID string) (string, error)
	releaseStream func(sessionID string)
}

type participantSession struct {
	Participant
	connection *connection
}

func New(options Options) *Room {
	if options.PingInterval <= 0 {
		options.PingInterval = 10 * time.Second
	}
	if options.PongTimeout <= 0 {
		options.PongTimeout = 30 * time.Second
	}
	if options.Logger == nil {
		options.Logger = slog.Default()
	}
	if options.AssignStream == nil {
		options.AssignStream = func(string) (string, error) { return newSessionID() }
	}
	return &Room{
		participants:  make(map[string]*participantSession),
		pingInterval:  options.PingInterval,
		pongTimeout:   options.PongTimeout,
		logger:        options.Logger,
		assignStream:  options.AssignStream,
		releaseStream: options.ReleaseStream,
	}
}

func (r *Room) Handler() http.Handler {
	return http.HandlerFunc(r.serveWebSocket)
}

func (r *Room) snapshotLocked(origin string) []Participant {
	participants := make([]Participant, 0, len(r.participants))
	for _, session := range r.participants {
		participant := session.Participant
		if participant.StreamID != "" {
			base := origin + "/media/" + participant.StreamID
			participant.PublishURL = base + "/whip"
			participant.ReadURL = base + "/whep"
		}
		participants = append(participants, participant)
	}
	sort.Slice(participants, func(i, j int) bool { return participants[i].ID < participants[j].ID })
	return participants
}

func normalizeName(name, id string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Guest " + id[:4]
	}
	return name
}

func newSessionID() (string, error) {
	var value [12]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("create a room session ID: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}

func (r *Room) join(client *connection, name string) error {
	r.mu.Lock()
	if client.id != "" {
		r.mu.Unlock()
		return client.writeError("this connection has already joined")
	}
	id, err := newSessionID()
	if err != nil {
		r.mu.Unlock()
		return err
	}
	for r.participants[id] != nil {
		id, err = newSessionID()
		if err != nil {
			r.mu.Unlock()
			return err
		}
	}
	streamID, err := r.assignStream(id)
	if err != nil {
		r.mu.Unlock()
		return fmt.Errorf("assign a media stream: %w", err)
	}
	client.id = id
	participant := Participant{
		ID:           id,
		Name:         normalizeName(name, id),
		AudioEnabled: false,
		VideoEnabled: false,
		StreamID:     streamID,
	}
	r.participants[id] = &participantSession{Participant: participant, connection: client}
	if err := client.writeMessage("welcome", map[string]any{"selfId": id, "participants": r.snapshotLocked(client.origin)}); err != nil {
		delete(r.participants, id)
		client.id = ""
		r.mu.Unlock()
		if r.releaseStream != nil {
			r.releaseStream(id)
		}
		return err
	}
	err = r.broadcastLocked()
	r.mu.Unlock()
	return err
}

func (r *Room) rename(client *connection, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	participant := r.participants[client.id]
	if participant == nil {
		return errors.New("join the room before renaming")
	}
	participant.Name = normalizeName(name, participant.ID)
	return r.broadcastLocked()
}

func (r *Room) updateMediaState(client *connection, audioEnabled, videoEnabled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	participant := r.participants[client.id]
	if participant == nil {
		return errors.New("join the room before updating media state")
	}
	participant.AudioEnabled = audioEnabled
	participant.VideoEnabled = videoEnabled
	return r.broadcastLocked()
}

func (r *Room) updateStreamState(client *connection, ready bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	participant := r.participants[client.id]
	if participant == nil {
		return errors.New("join the room before updating stream state")
	}
	participant.StreamReady = ready
	return r.broadcastLocked()
}

func (r *Room) leave(client *connection) string {
	r.mu.Lock()
	if client.id == "" {
		r.mu.Unlock()
		return ""
	}
	id := client.id
	if _, exists := r.participants[client.id]; !exists {
		client.id = ""
		r.mu.Unlock()
		return id
	}
	delete(r.participants, client.id)
	client.id = ""
	_ = r.broadcastLocked()
	r.mu.Unlock()
	if r.releaseStream != nil {
		r.releaseStream(id)
	}
	return id
}

func (r *Room) broadcastLocked() error {
	var firstError error
	for _, session := range r.participants {
		if err := session.connection.writeMessage("participants", map[string]any{"participants": r.snapshotLocked(session.connection.origin)}); err != nil {
			session.connection.closeNow()
			if firstError == nil {
				firstError = err
			}
		}
	}
	return firstError
}
