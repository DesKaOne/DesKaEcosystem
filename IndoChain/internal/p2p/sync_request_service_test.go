package p2p

import (
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestSyncRequestServiceRejectsNilHandler(t *testing.T) {
	s := &SyncRequestService{}
	_, err := s.Handle(Message{Type: MessageTypeBlockRequest, Payload: make([]byte, 16)})
	if !errors.Is(err, ErrNilSyncRequestService) {
		t.Fatalf("expected nil service error, got %v", err)
	}
}

func TestSyncRequestServiceDelegatesToHandler(t *testing.T) {
	service := &SyncRequestService{
		Handler: &SyncMessageHandler{
			Service: &SyncService{
				Reader: rangeReader{blocks: map[types.Height]block.Block{1: {Header: block.Header{Height: 1}}}},
				MaxLimit: 2,
			},
			MaxPayload: 16,
			MaxLimit:   2,
		},
	}
	payload, err := EncodeBlockRequest(BlockRequest{FromHeight: 1, Limit: 1}, 2)
	if err != nil {
		t.Fatalf("encode request: %v", err)
	}
	resp, err := service.Handle(Message{
		Type:    MessageTypeBlockRequest,
		Payload: payload,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Blocks) != 1 {
		t.Fatalf("expected one block, got %d", len(resp.Blocks))
	}
	if resp.Blocks[0].Header.Height != 1 {
		t.Fatalf("got height %d, want 1", resp.Blocks[0].Header.Height)
	}
}

func TestSyncRequestServicePreservesHandlerValidationError(t *testing.T) {
	service := &SyncRequestService{
		Handler: &SyncMessageHandler{
			Service:    &SyncService{Reader: rangeReader{}, MaxLimit: 2},
			MaxPayload: 8,
			MaxLimit:   2,
		},
	}
	_, err := service.Handle(Message{
		Type:    MessageTypeBlockRequest,
		Payload: make([]byte, 16),
	})
	if !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("expected message validation error, got %v", err)
	}
}
