package room

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

type receivedMessage struct {
	message roomMessage
	err     error
}

type roomMessage struct {
	Type    string `json:"type"`
	Payload struct {
		SelfID       string        `json:"selfId"`
		Participants []Participant `json:"participants"`
	} `json:"payload"`
}

type synchronizedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *synchronizedBuffer) Write(value []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(value)
}

func (b *synchronizedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

func TestJoinDefaultsBlankNameAndWelcomesBeforeSnapshot(t *testing.T) {
	server := httptest.NewServer(New(Options{}).Handler())
	defer server.Close()
	client := dialTestClient(t, server.URL)
	defer client.CloseNow()

	if err := wsjson.Write(context.Background(), client, map[string]any{
		"type": "join", "payload": map[string]string{"name": "  "},
	}); err != nil {
		t.Fatal(err)
	}
	welcome := readTestMessage(t, client)
	if welcome.Type != "welcome" {
		t.Fatalf("first message type = %q, want welcome", welcome.Type)
	}
	if len(welcome.Payload.Participants) != 1 {
		t.Fatalf("welcome participants = %d, want 1", len(welcome.Payload.Participants))
	}
	participant := welcome.Payload.Participants[0]
	if participant.ID != welcome.Payload.SelfID {
		t.Fatalf("welcome self ID = %q, participant ID = %q", welcome.Payload.SelfID, participant.ID)
	}
	if want := "Guest " + participant.ID[:4]; participant.Name != want {
		t.Fatalf("fallback name = %q, want %q", participant.Name, want)
	}
	if participant.StreamID == "" {
		t.Fatal("participant has no assigned media stream ID")
	}
	if participant.PublishURL != server.URL+"/media/"+participant.StreamID+"/whip" {
		t.Fatalf("publish URL = %q", participant.PublishURL)
	}
	if participant.ReadURL != server.URL+"/media/"+participant.StreamID+"/whep" {
		t.Fatalf("read URL = %q", participant.ReadURL)
	}
	if participant.StreamReady {
		t.Fatal("new participant stream should not be ready before browser publication")
	}
	if participant.AudioEnabled || participant.VideoEnabled {
		t.Fatalf("pre-media flags = (%v, %v), want both disabled", participant.AudioEnabled, participant.VideoEnabled)
	}
	if next := readTestMessage(t, client); next.Type != "participants" {
		t.Fatalf("second message type = %q, want participants", next.Type)
	}
	if err := wsjson.Write(context.Background(), client, map[string]any{
		"type": "streamState", "payload": map[string]bool{"ready": true},
	}); err != nil {
		t.Fatal(err)
	}
	readySnapshot := readTestMessage(t, client)
	if readySnapshot.Type != "participants" || len(readySnapshot.Payload.Participants) != 1 || !readySnapshot.Payload.Participants[0].StreamReady {
		t.Fatalf("stream-ready snapshot = %#v", readySnapshot)
	}
}

func TestParticipantDepartureReleasesItsMediaAssociation(t *testing.T) {
	released := make(chan string, 1)
	room := New(Options{
		AssignStream:  func(sessionID string) (string, error) { return "0123456789abcdef01234567", nil },
		ReleaseStream: func(sessionID string) { released <- sessionID },
	})
	server := httptest.NewServer(room.Handler())
	defer server.Close()
	client := dialTestClient(t, server.URL)
	if err := wsjson.Write(context.Background(), client, map[string]any{"type": "join", "payload": map[string]string{"name": "Kitchen"}}); err != nil {
		t.Fatal(err)
	}
	if got := readTestMessage(t, client).Type; got != "welcome" {
		t.Fatalf("first message type = %q, want welcome", got)
	}
	client.CloseNow()
	select {
	case sessionID := <-released:
		if sessionID == "" {
			t.Fatal("release callback got an empty participant session ID")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("participant departure did not release its media association")
	}
}

func TestMissedPongExpiresConnectionAndRemovesItOnce(t *testing.T) {
	room := New(Options{PingInterval: 300 * time.Millisecond, PongTimeout: 100 * time.Millisecond})
	server := httptest.NewServer(room.Handler())
	defer server.Close()

	quietClient := dialTestClient(t, server.URL)
	defer quietClient.CloseNow()
	if err := wsjson.Write(context.Background(), quietClient, map[string]any{"type": "join", "payload": map[string]string{"name": "Quiet"}}); err != nil {
		t.Fatal(err)
	}
	if got := readTestMessage(t, quietClient).Type; got != "welcome" {
		t.Fatalf("quiet client first message = %q, want welcome", got)
	}

	observer := dialTestClient(t, server.URL)
	defer observer.CloseNow()
	messages := startTestReader(observer)
	if err := wsjson.Write(context.Background(), observer, map[string]any{"type": "join", "payload": map[string]string{"name": "Observer"}}); err != nil {
		t.Fatal(err)
	}
	welcome := receiveTestMessage(t, messages)
	if welcome.Type != "welcome" || len(welcome.Payload.Participants) != 2 {
		t.Fatalf("observer welcome = %#v, want two participants", welcome)
	}
	quietID := welcome.Payload.Participants[0].ID
	for _, participant := range welcome.Payload.Participants {
		if participant.Name == "Quiet" {
			quietID = participant.ID
		}
	}

	removals := 0
	deadline := time.After(2 * time.Second)
	for removals == 0 {
		select {
		case event := <-messages:
			if event.err != nil {
				t.Fatalf("read observer message: %v", event.err)
			}
			if event.message.Type == "participants" && !hasParticipant(event.message.Payload.Participants, quietID) {
				removals++
			}
		case <-deadline:
			t.Fatal("quiet client did not expire after missing a pong")
		}
	}

	duplicateDeadline := time.After(150 * time.Millisecond)
	for {
		select {
		case event := <-messages:
			if event.err != nil {
				t.Fatalf("observer disconnected before duplicate-removal check: %v", event.err)
			}
			if event.message.Type == "participants" && !hasParticipant(event.message.Payload.Participants, quietID) {
				removals++
			}
		case <-duplicateDeadline:
			if removals != 1 {
				t.Fatalf("removal notifications = %d, want exactly 1", removals)
			}
			return
		}
	}
}

func TestConnectAndDisconnectLogsContainClientAndLocalAddresses(t *testing.T) {
	var output synchronizedBuffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	server := httptest.NewServer(New(Options{Logger: logger}).Handler())
	defer server.Close()
	client := dialTestClient(t, server.URL)
	client.CloseNow()

	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(output.String(), "websocket disconnect") && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	logOutput := output.String()
	for _, field := range []string{"websocket connect", "websocket disconnect", "client_ip=", "local_addr=", "local_port="} {
		if !strings.Contains(logOutput, field) {
			t.Fatalf("log output %q does not contain %q", logOutput, field)
		}
	}
}

func dialTestClient(t *testing.T, httpURL string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(httpURL, "http")
	client, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", wsURL, err)
	}
	return client
}

func readTestMessage(t *testing.T, client *websocket.Conn) roomMessage {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, reader, err := client.Reader(ctx)
	if err != nil {
		t.Fatalf("read WebSocket message: %v", err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read WebSocket payload: %v", err)
	}
	var message roomMessage
	if err := json.Unmarshal(data, &message); err != nil {
		t.Fatalf("decode WebSocket message: %v", err)
	}
	return message
}

func startTestReader(client *websocket.Conn) <-chan receivedMessage {
	messages := make(chan receivedMessage, 16)
	go func() {
		for {
			_, reader, err := client.Reader(context.Background())
			if err != nil {
				messages <- receivedMessage{err: err}
				return
			}
			data, err := io.ReadAll(reader)
			if err != nil {
				messages <- receivedMessage{err: err}
				return
			}
			var message roomMessage
			if err := json.Unmarshal(data, &message); err != nil {
				messages <- receivedMessage{err: err}
				return
			}
			messages <- receivedMessage{message: message}
		}
	}()
	return messages
}

func receiveTestMessage(t *testing.T, messages <-chan receivedMessage) roomMessage {
	t.Helper()
	select {
	case event := <-messages:
		if event.err != nil {
			t.Fatalf("read WebSocket message: %v", event.err)
		}
		return event.message
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for WebSocket message")
		return roomMessage{}
	}
}

func hasParticipant(participants []Participant, id string) bool {
	for _, participant := range participants {
		if participant.ID == id {
			return true
		}
	}
	return false
}
