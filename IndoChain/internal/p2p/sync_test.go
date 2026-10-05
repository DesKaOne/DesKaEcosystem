package p2p

import "testing"

func TestValidateBlockRequest(t *testing.T) {
	valid := BlockRequest{FromHeight: 10, Limit: 5}
	if err := ValidateBlockRequest(valid, 10); err != nil {
		t.Fatal(err)
	}
	for _, req := range []BlockRequest{
		{FromHeight: 0, Limit: 0},
		{FromHeight: 0, Limit: 11},
	} {
		if err := ValidateBlockRequest(req, 10); err != ErrInvalidSyncRequest {
			t.Fatalf("error = %v, want %v", err, ErrInvalidSyncRequest)
		}
	}
}
