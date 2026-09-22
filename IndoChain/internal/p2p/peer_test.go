package p2p

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestPeerSet(t *testing.T) {
	set := NewPeerSet()
	peer := Peer{ID: "peer-1", Address: "127.0.0.1:9000", ProtocolVersion: 1}
	if err := set.Add(peer); err != nil {
		t.Fatal(err)
	}
	if err := set.Add(peer); err != ErrDuplicatePeer {
		t.Fatalf("duplicate error = %v, want %v", err, ErrDuplicatePeer)
	}
	got, ok := set.Get(peer.ID)
	if !ok || got.Address != peer.Address || got.ProtocolVersion != types.ProtocolVersion(1) {
		t.Fatalf("unexpected peer: %+v", got)
	}
	if set.Len() != 1 {
		t.Fatalf("peer count = %d, want 1", set.Len())
	}
	if !set.Remove(peer.ID) || set.Len() != 0 {
		t.Fatal("peer removal failed")
	}
	if set.Remove(peer.ID) {
		t.Fatal("second removal should report false")
	}
}

func TestPeerSetRejectsEmptyID(t *testing.T) {
	set := NewPeerSet()
	if err := set.Add(Peer{}); err != ErrNilPeer {
		t.Fatalf("error = %v, want %v", err, ErrNilPeer)
	}
}
