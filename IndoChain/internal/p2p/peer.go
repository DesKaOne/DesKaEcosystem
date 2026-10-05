package p2p

import (
	"errors"
	"sync"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var (
	ErrNilPeer      = errors.New("nil peer")
	ErrDuplicatePeer = errors.New("peer already exists")
)

type PeerID string

type Peer struct {
	ID PeerID
	Address string
	ProtocolVersion types.ProtocolVersion
}

type PeerSet struct {
	mu sync.RWMutex
	peers map[PeerID]Peer
}

func NewPeerSet() *PeerSet {
	return &PeerSet{peers: make(map[PeerID]Peer)}
}

func (s *PeerSet) Add(peer Peer) error {
	if peer.ID == "" {
		return ErrNilPeer
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.peers[peer.ID]; exists {
		return ErrDuplicatePeer
	}
	s.peers[peer.ID] = peer
	return nil
}

func (s *PeerSet) Remove(id PeerID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.peers[id]; !ok {
		return false
	}
	delete(s.peers, id)
	return true
}

func (s *PeerSet) Get(id PeerID) (Peer, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	peer, ok := s.peers[id]
	return peer, ok
}

func (s *PeerSet) Snapshot() []Peer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Peer, 0, len(s.peers))
	for _, peer := range s.peers {
		out = append(out, peer)
	}
	return out
}

func (s *PeerSet) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.peers)
}
