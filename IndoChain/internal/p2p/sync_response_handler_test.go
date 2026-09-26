package p2p

import (
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
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


func TestSyncResponseHandlerAdvancesSession(t *testing.T) {
	genesis := types.Hash{7}
	b1 := block.Block{Header: block.Header{Height: 1, PreviousHash: genesis}}
	h1, err := block.Hash(b1)
	if err != nil {
		t.Fatalf("hash b1: %v", err)
	}

	h := &SyncResponseHandler{
		Session: &SyncSession{
			Planner: SyncPlanner{MaxBatch: 1},
			Coordinator: &SyncCoordinator{
				Reader:   &cursorApplyReader{},
				Importer: &plannedResponseImporter{},
			},
			Cursor: SyncCursor{Height: 0, BlockHash: genesis},
		},
		Decoder: testSyncResponseDecoder{
			response: BlockResponse{Blocks: []block.Block{b1}},
		},
		MaxPayload: 16,
	}

	advanced, err := h.Handle(1, Message{
		Type:    MessageTypeBlockResponse,
		Payload: []byte{1},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !advanced {
		t.Fatal("expected session to advance")
	}
	if h.Session.Cursor.Height != 1 || h.Session.Cursor.BlockHash != h1 {
		t.Fatalf("unexpected cursor: %+v", h.Session.Cursor)
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
			Coordinator: &SyncCoordinator{
				Reader:   &cursorApplyReader{},
				Importer: &plannedResponseImporter{},
			},
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
