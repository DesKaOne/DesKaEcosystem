package p2p

import "testing"

func TestPropagatorSuppressesDuplicates(t *testing.T) {
	p, err := NewPropagator(PropagationPolicy{MaxPeers: 3})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.MarkSeen("tx-1"); err != nil {
		t.Fatal(err)
	}
	if err := p.MarkSeen("tx-1"); err != ErrDuplicateMessage {
		t.Fatalf("error = %v, want %v", err, ErrDuplicateMessage)
	}
}

func TestPropagatorFanout(t *testing.T) {
	p, err := NewPropagator(PropagationPolicy{MaxPeers: 3})
	if err != nil {
		t.Fatal(err)
	}
	for peers, want := range map[int]int{0: 0, 2: 2, 3: 3, 10: 3} {
		if got := p.Fanout(peers); got != want {
			t.Fatalf("Fanout(%d) = %d, want %d", peers, got, want)
		}
	}
}

func TestPropagatorRejectsInvalidPolicy(t *testing.T) {
	if _, err := NewPropagator(PropagationPolicy{}); err != ErrInvalidPropagation {
		t.Fatalf("error = %v, want %v", err, ErrInvalidPropagation)
	}
}
