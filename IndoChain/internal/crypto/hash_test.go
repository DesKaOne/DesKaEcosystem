package crypto

import "testing"

func TestSHA256HasherDeterministic(t *testing.T) {
	h := SHA256Hasher{}
	a := h.Hash([]byte("indochain"))
	b := h.Hash([]byte("indochain"))
	if a != b {
		t.Fatal("same input must produce same hash")
	}
}
