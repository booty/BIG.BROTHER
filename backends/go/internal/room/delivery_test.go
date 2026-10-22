package room

import (
	"context"
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
