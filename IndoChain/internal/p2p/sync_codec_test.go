package p2p

import (
	"bytes"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestSyncRequestCodecRoundTrip(t *testing.T) {
	req := BlockRequest{FromHeight: 42, Limit: 8}
	data, err := EncodeBlockRequest(req, 16)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeBlockRequest(data, 16)
	if err != nil {
		t.Fatal(err)
	}
	if got != req {
		t.Fatalf("got %+v, want %+v", got, req)
	}
}

func TestSyncRequestCodecIsDeterministic(t *testing.T) {
	req := BlockRequest{FromHeight: 7, Limit: 3}
	a, err := EncodeBlockRequest(req, 8)
	if err != nil {
		t.Fatal(err)
	}
	b, err := EncodeBlockRequest(req, 8)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("encoding is not deterministic")
	}
}

func TestSyncRequestCodecRejectsMalformedPayload(t *testing.T) {
	if _, err := DecodeBlockRequest([]byte{1, 2, 3}, 8); err != ErrInvalidSyncPayload {
		t.Fatalf("error = %v, want %v", err, ErrInvalidSyncPayload)
	}
	if _, err := DecodeBlockRequest(make([]byte, 16), 8); err == nil {
		t.Fatal("expected zero limit rejection")
	}
	if _, err := DecodeBlockRequest(func() []byte {
		data, _ := EncodeBlockRequest(BlockRequest{FromHeight: types.Height(1), Limit: 2}, 2)
		data[15] = 3
		return data
	}(), 2); err == nil {
		t.Fatal("expected limit rejection")
	}
}
