package p2p

import (
	"bytes"
	"testing"
)

func TestMessageRoundTrip(t *testing.T) {
	original := Message{Type: MessageTypeTransaction, Payload: []byte("tx-payload")}
	encoded, err := EncodeMessage(original, 1024)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeMessage(encoded, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Type != original.Type || !bytes.Equal(decoded.Payload, original.Payload) {
		t.Fatalf("decoded = %+v, want %+v", decoded, original)
	}
}

func TestMessageRejectsUnknownType(t *testing.T) {
	if _, err := EncodeMessage(Message{Type: 99, Payload: []byte("x")}, 1024); err != ErrUnknownMessage {
		t.Fatalf("error = %v, want %v", err, ErrUnknownMessage)
	}
}

func TestMessageRejectsOversizedPayload(t *testing.T) {
	if _, err := EncodeMessage(Message{Type: MessageTypeBlock, Payload: []byte("12345")}, 4); err != ErrInvalidMessage {
		t.Fatalf("error = %v, want %v", err, ErrInvalidMessage)
	}
}

func TestDecodeMessageRejectsTruncatedOrTrailingData(t *testing.T) {
	encoded, err := EncodeMessage(Message{Type: MessageTypeBlock, Payload: []byte("block")}, 1024)
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{
		encoded[:5],
		append(append([]byte(nil), encoded...), 0),
	} {
		if _, err := DecodeMessage(data, 1024); err != ErrInvalidMessage {
			t.Fatalf("error = %v, want %v", err, ErrInvalidMessage)
		}
	}
}
