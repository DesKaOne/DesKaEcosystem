package p2p

import (
	"bytes"
	"errors"
	"testing"
)

func TestInMemoryTransportRoutesMessageBetweenNodes(t *testing.T) {
	a := NewInMemoryTransport(PeerID("node-a"), 1024)
	b := NewInMemoryTransport(PeerID("node-b"), 1024)

	if err := a.Connect(PeerID("node-b"), b); err != nil { t.Fatal(err) }
	if err := b.Connect(PeerID("node-a"), a); err != nil { t.Fatal(err) }

	msg := Message{Type: MessageTypeBlock, Payload: []byte("consensus-payload")}
	if err := a.Send(PeerID("node-b"), msg); err != nil { t.Fatal(err) }

	from, got, err := b.Receive()
	if err != nil { t.Fatal(err) }
	if from != PeerID("node-a") { t.Fatalf("from = %q, want node-a", from) }
	if got.Type != msg.Type || !bytes.Equal(got.Payload, msg.Payload) {
		t.Fatalf("got = %+v, want %+v", got, msg)
	}
}

func TestInMemoryTransportRejectsUnknownPeerWithoutMutation(t *testing.T) {
	a := NewInMemoryTransport(PeerID("node-a"), 1024)
	msg := Message{Type: MessageTypeVote, Payload: []byte("vote")}
	if err := a.Send(PeerID("missing"), msg); !errors.Is(err, ErrUnknownPeer) { t.Fatalf("error = %v, want %v", err, ErrUnknownPeer) }
	if _, _, err := a.Receive(); !errors.Is(err, ErrUnknownMessage) { t.Fatalf("receive error = %v, want %v", err, ErrUnknownMessage) }
}

func TestInMemoryTransportClonesPayload(t *testing.T) {
	a := NewInMemoryTransport(PeerID("node-a"), 1024)
	b := NewInMemoryTransport(PeerID("node-b"), 1024)
	if err := a.Connect(PeerID("node-b"), b); err != nil { t.Fatal(err) }

	payload := []byte("immutable")
	if err := a.Send(PeerID("node-b"), Message{Type: MessageTypeVote, Payload: payload}); err != nil { t.Fatal(err) }
	payload[0] = 'X'

	_, got, err := b.Receive()
	if err != nil { t.Fatal(err) }
	if string(got.Payload) != "immutable" { t.Fatalf("payload = %q, want immutable", got.Payload) }
}
