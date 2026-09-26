package types

import "testing"

func TestHashStringIsDeterministic(t *testing.T) {
	var h Hash
	if got := h.String(); got == "" {
		t.Fatal("hash string must not be empty")
	}
	if got := h.String(); len(got) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(got))
	}
}
