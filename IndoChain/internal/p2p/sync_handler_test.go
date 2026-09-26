package p2p

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestSyncMessageHandlerRoutesBlockRequest(t *testing.T) {
	reader := rangeReader{blocks: map[types.Height]block.Block{
		10: {Header: block.Header{Height: 10}},
	}}
	service := &SyncService{Reader: reader, MaxLimit: 4}
	handler := &SyncMessageHandler{Service: service, MaxPayload: 64, MaxLimit: 4}

	payload, err := EncodeBlockRequest(BlockRequest{FromHeight: 10, Limit: 1}, 4)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := handler.Handle(Message{Type: MessageTypeBlockRequest, Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Blocks) != 1 || resp.Blocks[0].Header.Height != 10 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestSyncMessageHandlerRejectsWrongMessageType(t *testing.T) {
	handler := &SyncMessageHandler{Service: &SyncService{MaxLimit: 4}, MaxPayload: 64, MaxLimit: 4}
	_, err := handler.Handle(Message{Type: MessageTypeBlockResponse, Payload: []byte{1}})
	if err != ErrUnexpectedSyncMessage {
		t.Fatalf("error = %v, want %v", err, ErrUnexpectedSyncMessage)
	}
}

func TestSyncMessageHandlerRejectsOversizedPayload(t *testing.T) {
	handler := &SyncMessageHandler{Service: &SyncService{MaxLimit: 4}, MaxPayload: 8, MaxLimit: 4}
	_, err := handler.Handle(Message{Type: MessageTypeBlockRequest, Payload: make([]byte, 16)})
	if err == nil {
		t.Fatal("expected oversized payload rejection")
	}
}
