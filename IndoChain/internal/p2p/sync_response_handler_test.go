package p2p

import (
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

type testSyncResponseDecoder struct {
	response BlockResponse
	err      error
}

func (d testSyncResponseDecoder) DecodeBlockResponse([]byte) (BlockResponse, error) {
	if d.err != nil {
		return BlockResponse{}, d.err
	}
	return d.response, nil
}

func TestSyncResponseHandlerRejectsWrongMessageType(t *testing.T) {
	h := &SyncResponseHandler{
		Session: &SyncSession{},
		Decoder: testSyncResponseDecoder{},
		MaxPayload: 16,
	}
	_, err := h.Handle(0, Message{Type: MessageTypeBlockRequest, Payload: []byte{1}})
	if !errors.Is(err, ErrUnexpectedSyncResponse) {
		t.Fatalf("expected unexpected response error, got %v", err)
	}
}

func TestSyncResponseHandlerRejectsNilDecoder(t *testing.T) {
	h := &SyncResponseHandler{Session: &SyncSession{}}
	_, err := h.Handle(0, Message{Type: MessageTypeBlockResponse, Payload: []byte{1}})
	if !errors.Is(err, ErrNilSyncResponseDecoder) {
		t.Fatalf("expected nil decoder error, got %v", err)
	}
}

func TestSyncResponseHandlerPropagatesDecodeError(t *testing.T) {
	want := errors.New("decode failed")
	h := &SyncResponseHandler{
		Session: &SyncSession{},
		Decoder: testSyncResponseDecoder{err: want},
		MaxPayload: 16,
	}
	_, err := h.Handle(0, Message{Type: MessageTypeBlockResponse, Payload: []byte{1}})
	if !errors.Is(err, want) {
		t.Fatalf("expected decode error, got %v", err)
	}
}

func TestSyncResponseHandlerValidatesEnvelope(t *testing.T) {
	h := &SyncResponseHandler{
		Session: &SyncSession{},
		Decoder: testSyncResponseDecoder{},
		MaxPayload: 1,
	}
	_, err := h.Handle(0, Message{Type: MessageTypeBlockResponse, Payload: []byte{1, 2}})
	if !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("expected invalid message error, got %v", err)
	}
}

func TestSyncResponseHandlerAppliesSession(t *testing.T) {
	h := &SyncResponseHandler{
		Session: &SyncSession{
			Planner: SyncPlanner{MaxBatch: 2},
			Cursor: SyncCursor{Height: 0},
		},
		Decoder: testSyncResponseDecoder{
			response: BlockResponse{},
		},
		MaxPayload: 16,
	}
	advanced, err := h.Handle(types.Height(0), Message{
		Type: MessageTypeBlockResponse,
		Payload: []byte{1},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if advanced {
		t.Fatal("empty response should not advance")
	}
}
