package room

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// No worker consumes this connection's output: it models stalled delivery.
func TestRoomChangesDoNotWaitForNetworkDelivery(t *testing.T) {
	r := New(Options{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	slow := &connection{room: r, ctx: ctx, cancel: cancel, origin: "https://home", outgoing: make(chan outboundMessage, 32)}
	done := make(chan error, 1)
	go func() {
		if err := r.join(slow, "Kitchen"); err != nil {
			done <- err
			return
		}
		done <- r.rename(slow, "Hallway")
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("room mutation waited for stalled network delivery")
	}
	welcome := <-slow.outgoing
	first := <-slow.outgoing
	second := <-slow.outgoing
	if welcome.messageType != "welcome" || first.messageType != "participants" || second.messageType != "participants" {
		t.Fatal("welcome and snapshots lost their order")
	}
	before := first.payload.(map[string]any)["participants"].([]Participant)
	after := second.payload.(map[string]any)["participants"].([]Participant)
	if before[0].Name != "Kitchen" || after[0].Name != "Hallway" {
		t.Fatalf("snapshots were overwritten or reordered: %q -> %q", before[0].Name, after[0].Name)
	}
}

func TestFailedRecipientDoesNotRejectAnotherParticipantsRename(t *testing.T) {
	transportServer := httptest.NewServer(New(Options{}).Handler())
	defer transportServer.Close()
	deadSocket := dialTestClient(t, transportServer.URL)
	defer deadSocket.CloseNow()
	var logs synchronizedBuffer
	r := New(Options{Logger: slog.New(slog.NewTextHandler(&logs, nil))})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	actor := &connection{room: r, ctx: ctx, cancel: cancel, origin: "https://home", outgoing: make(chan outboundMessage, 32)}
	if err := r.join(actor, "Kitchen"); err != nil {
		t.Fatal(err)
	}
	deadCtx, deadCancel := context.WithCancel(context.Background())
	deadCancel()
	r.participants["departing"] = &participantSession{Participant: Participant{ID: "departing", Name: "Departing"}, connection: &connection{ctx: deadCtx, cancel: deadCancel, ws: deadSocket, outgoing: make(chan outboundMessage, 32)}}
	if err := r.rename(actor, "Hallway"); err != nil {
		t.Fatalf("successful rename reported another recipient's delivery failure: %v", err)
	}
	if got := r.participants[actor.id].Name; got != "Hallway" {
		t.Fatalf("rename was not applied: %q", got)
	}
	if !strings.Contains(logs.String(), "room delivery failed") {
		t.Fatal("recipient delivery failure was not logged")
	}
	if err := r.rename(&connection{id: "unjoined"}, "Other"); err == nil {
		t.Fatal("invalid requests must still return errors")
	}
}
