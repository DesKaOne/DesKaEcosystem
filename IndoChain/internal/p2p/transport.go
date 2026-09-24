package p2p

import (
	"errors"
	"sync"
)

var (
	ErrNilTransport = errors.New("nil p2p transport")
	ErrUnknownPeer = errors.New("unknown p2p peer")
	ErrEmptyPeerID = errors.New("empty peer id")
)

type Transport interface {
	Send(peer PeerID, msg Message) error
	Receive() (PeerID, Message, error)
}

type InMemoryTransport struct {
	mu         sync.Mutex
	localID    PeerID
	peers      map[PeerID]*InMemoryTransport
	inbox      []transportEnvelope
	maxPayload uint32
}

type transportEnvelope struct {
	from PeerID
	msg  Message
}

func NewInMemoryTransport(localID PeerID, maxPayload uint32) *InMemoryTransport {
	return &InMemoryTransport{localID: localID, peers: make(map[PeerID]*InMemoryTransport), maxPayload: maxPayload}
}

func (t *InMemoryTransport) Connect(peer PeerID, remote *InMemoryTransport) error {
	if t == nil || remote == nil { return ErrNilTransport }
	if t.localID == "" || peer == "" { return ErrEmptyPeerID }
	t.mu.Lock()
	defer t.mu.Unlock()
	t.peers[peer] = remote
	return nil
}

func (t *InMemoryTransport) Send(peer PeerID, msg Message) error {
	if t == nil { return ErrNilTransport }
	if t.localID == "" || peer == "" { return ErrEmptyPeerID }
	if err := ValidateMessage(msg, t.maxPayload); err != nil { return err }
	t.mu.Lock()
	remote, ok := t.peers[peer]
	t.mu.Unlock()
	if !ok { return ErrUnknownPeer }
	remote.mu.Lock()
	remote.inbox = append(remote.inbox, transportEnvelope{
		from: t.localID,
		msg: Message{Type: msg.Type, Payload: append([]byte(nil), msg.Payload...)},
	})
	remote.mu.Unlock()
	return nil
}

func (t *InMemoryTransport) Receive() (PeerID, Message, error) {
	if t == nil { return "", Message{}, ErrNilTransport }
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.inbox) == 0 { return "", Message{}, ErrUnknownMessage }
	envelope := t.inbox[0]
	t.inbox = t.inbox[1:]
	return envelope.from, Message{Type: envelope.msg.Type, Payload: append([]byte(nil), envelope.msg.Payload...)}, nil
}
