package encoding

import "testing"

func TestUint64RoundTrip(t *testing.T) {
	want := uint64(1001)
	got, err := Uint64From(Uint64(want))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %d, want %d", got, want)
	}
}

func TestUint64RejectsWrongSize(t *testing.T) {
	if _, err := Uint64From([]byte{1, 2}); err == nil {
		t.Fatal("expected invalid data error")
	}
}
