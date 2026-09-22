package p2p

import (
	"errors"
	"testing"
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
				Reader: rangeReader{},
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
	if len(resp.Blocks) != 0 {
		t.Fatalf("expected empty response, got %d blocks", len(resp.Blocks))
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
