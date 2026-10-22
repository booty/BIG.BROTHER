package room

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const maxMessageSize = 1 << 20
const outboundQueueCapacityMessages = 32

type outboundMessage struct {
	messageType string
	payload     any
}

type wireMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type connection struct {
	room     *Room
	ws       *websocket.Conn
	ctx      context.Context
	cancel   context.CancelFunc
	outgoing chan outboundMessage
	id       string
	origin   string
}

func (r *Room) serveWebSocket(w http.ResponseWriter, request *http.Request) {
	ws, err := websocket.Accept(w, request, nil)
	if err != nil {
		return
	}
	ctx, cancel := context.WithCancel(request.Context())
	scheme := "http"
	if request.TLS != nil {
		scheme = "https"
	}
	origin := (&url.URL{Scheme: scheme, Host: request.Host}).String()
	client := &connection{room: r, ws: ws, ctx: ctx, cancel: cancel, origin: origin, outgoing: make(chan outboundMessage, outboundQueueCapacityMessages)}
	writeDone := make(chan struct{})
	go func() { defer close(writeDone); client.writeLoop() }()
	clientIP, localAddress, localPort := requestAddresses(request)
	r.logger.Info("websocket connect", "client_ip", clientIP, "local_addr", localAddress, "local_port", localPort)
	pingDone := make(chan struct{})
	go func() {
		defer close(pingDone)
		client.pingLoop(r.pingInterval, r.pongTimeout)
	}()
	defer func() {
		cancel()
		ws.CloseNow()
		<-pingDone
		<-writeDone
		clientID := r.leave(client)
		r.logger.Info("websocket disconnect", "client_ip", clientIP, "local_addr", localAddress, "local_port", localPort, "client_id", clientID)
	}()

	for {
		messageType, reader, err := ws.Reader(ctx)
		if err != nil {
			return
		}
		payload, err := io.ReadAll(io.LimitReader(reader, maxMessageSize+1))
		if err != nil {
			return
		}
		if len(payload) > maxMessageSize {
			if _, err := io.Copy(io.Discard, reader); err != nil {
				return
			}
			if client.writeError("message is too large") != nil {
				return
			}
			continue
		}
		if messageType != websocket.MessageText {
			if client.writeError("room messages must be text JSON") != nil {
				return
			}
			continue
		}
		if !client.handleMessage(payload) {
			return
		}
	}
}

func (c *connection) handleMessage(payload []byte) bool {
	var message wireMessage
	if err := json.Unmarshal(payload, &message); err != nil {
		return c.writeError("malformed JSON message") == nil
	}
	if len(message.Payload) == 0 || string(message.Payload) == "null" {
		return c.writeError("message payload is required") == nil
	}
	switch message.Type {
	case "join":
		var payload struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(message.Payload, &payload); err != nil {
			return c.writeError("join payload must contain a name string") == nil
		}
		if err := c.room.join(c, payload.Name); err != nil {
			return c.writeError(err.Error()) == nil
		}
	case "rename":
		var payload struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(message.Payload, &payload); err != nil {
			return c.writeError("rename payload must contain a name string") == nil
		}
		if err := c.room.rename(c, payload.Name); err != nil {
			return c.writeError(err.Error()) == nil
		}
	case "mediaState":
		var payload struct {
			AudioEnabled *bool `json:"audioEnabled"`
			VideoEnabled *bool `json:"videoEnabled"`
		}
		if err := json.Unmarshal(message.Payload, &payload); err != nil || payload.AudioEnabled == nil || payload.VideoEnabled == nil {
			return c.writeError("mediaState payload must contain audioEnabled and videoEnabled booleans") == nil
		}
		if err := c.room.updateMediaState(c, *payload.AudioEnabled, *payload.VideoEnabled); err != nil {
			return c.writeError(err.Error()) == nil
		}
	case "streamState":
		var payload struct {
			Ready *bool `json:"ready"`
		}
		if err := json.Unmarshal(message.Payload, &payload); err != nil || payload.Ready == nil {
			return c.writeError("streamState payload must contain a ready boolean") == nil
		}
		if err := c.room.updateStreamState(c, *payload.Ready); err != nil {
			return c.writeError(err.Error()) == nil
		}
	default:
		if strings.TrimSpace(message.Type) == "" {
			return c.writeError("message type is required") == nil
		}
		return c.writeError("unsupported message type: "+message.Type) == nil
	}
	return true
}

func (c *connection) pingLoop(interval, timeout time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(c.ctx, timeout)
			err := c.ws.Ping(ctx)
			cancel()
			if err != nil {
				c.closeNow()
				return
			}
		}
	}
}

// Queue in room-mutation order without doing network I/O under the room lock.
func (c *connection) writeMessage(messageType string, payload any) error {
	if err := c.ctx.Err(); err != nil {
		return err
	}
	select {
	case c.outgoing <- outboundMessage{messageType: messageType, payload: payload}:
		return nil
	case <-c.ctx.Done():
		return c.ctx.Err()
	default:
		return errors.New("client is too slow to receive room updates")
	}
}

func (c *connection) writeLoop() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case message := <-c.outgoing:
			ctx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
			err := wsjson.Write(ctx, c.ws, map[string]any{"type": message.messageType, "payload": message.payload})
			cancel()
			if err != nil {
				c.closeNow()
				return
			}
		}
	}
}

func (c *connection) writeError(message string) error {
	return c.writeMessage("error", map[string]string{"message": message})
}

func (c *connection) closeNow() {
	c.cancel()
	c.ws.CloseNow()
}

func requestAddresses(request *http.Request) (clientIP, localAddress, localPort string) {
	clientIP = hostPart(request.RemoteAddr)
	localAddress, localPort = "unknown", "unknown"
	if address, ok := request.Context().Value(http.LocalAddrContextKey).(net.Addr); ok && address != nil {
		if host, port, err := net.SplitHostPort(address.String()); err == nil {
			localAddress, localPort = host, port
		} else {
			localAddress = address.String()
		}
	}
	return
}

func hostPart(address string) string {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return address
	}
	return host
}
