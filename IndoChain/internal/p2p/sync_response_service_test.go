package p2p

import (
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestSyncResponseServiceDelegatesToHandler(t *testing.T) {
	h := &SyncResponseHandler{
		Session: &SyncSession{
			Planner: SyncPlanner{MaxBatch: 1},
			Coordinator: &SyncCoordinator{
				Reader:   &cursorApplyReader{},
				Importer: &plannedResponseImporter{},
			},
		},
		Decoder: testSyncResponseDecoder{},
		MaxPayload: 16,
	}
	s := &SyncResponseService{Handler: h}
	advanced, err := s.Handle(0, Message{
		Type:    MessageTypeBlockResponse,
		Payload: []byte{1},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if advanced {
		t.Fatal("empty response should not advance")
	}
}

func TestSyncResponseServiceRejectsNilHandler(t *testing.T) {
	s := &SyncResponseService{}
	_, err := s.Handle(0, Message{Type: MessageTypeBlockResponse, Payload: []byte{1}})
	if !errors.Is(err, ErrNilSyncResponseSession) {
		t.Fatalf("expected nil service error, got %v", err)
	}
}

func TestSyncResponseServicePreservesHandlerError(t *testing.T) {
	s := &SyncResponseService{
		Handler: &SyncResponseHandler{
			Session: &SyncSession{},
			Decoder: testSyncResponseDecoder{},
			MaxPayload: 16,
		},
	}
	_, err := s.Handle(types.Height(0), Message{
		Type:    MessageTypeBlockResponse,
		Payload: []byte{1},
	})
	if !errors.Is(err, ErrNilSyncSession) {
		t.Fatalf("expected handler error, got %v", err)
	}
}
