package p2p

import "testing"

func TestPeerSelector(t *testing.T) {
	selector := PeerSelector{MinScore: 10, MaxPeers: 2}
	got, err := selector.Select([]PeerScore{
		{ID: "peer-low", Score: 5},
		{ID: "peer-a", Score: 20},
		{ID: "peer-b", Score: 30},
		{ID: "peer-c", Score: 30},
		{ID: "peer-b", Score: 30},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []PeerID{"peer-b", "peer-c"}
	if len(got) != len(want) {
		t.Fatalf("selected %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("selected %v, want %v", got, want)
		}
	}
}

func TestPeerSelectorRejectsInvalidLimit(t *testing.T) {
	if _, err := (PeerSelector{}).Select(nil); err != ErrInvalidPeerScore {
		t.Fatalf("error = %v, want %v", err, ErrInvalidPeerScore)
	}
}
