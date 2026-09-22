package p2p

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestBuildBlockRequestMessage(t *testing.T) {
	req := BlockRequest{FromHeight: 7, Limit: 3}
	msg, err := BuildBlockRequestMessage(req, 10, 16)
	if err != nil {
		t.Fatalf("build message: %v", err)
	}
	if msg.Type != MessageTypeBlockRequest {
		t.Fatalf("unexpected message type: %v", msg.Type)
	}
	decoded, err := DecodeBlockRequest(msg.Payload, 10)
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if decoded != req {
		t.Fatalf("decoded request mismatch: got %+v want %+v", decoded, req)
	}
}

func TestBuildBlockRequestMessageRejectsInvalidRequest(t *testing.T) {
	_, err := BuildBlockRequestMessage(BlockRequest{FromHeight: 1, Limit: 0}, 10, 16)
	if err != ErrInvalidSyncRequest {
		t.Fatalf("expected invalid request, got %v", err)
	}
}

func TestBuildBlockRequestMessageRejectsPayloadLimit(t *testing.T) {
	_, err := BuildBlockRequestMessage(BlockRequest{FromHeight: types.Height(1), Limit: 1}, 10, 15)
	if err != ErrInvalidMessage {
		t.Fatalf("expected invalid message, got %v", err)
	}
}
