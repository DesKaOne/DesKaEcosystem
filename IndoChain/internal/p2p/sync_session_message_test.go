package p2p

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestSyncSessionNextRequestMessage(t *testing.T) {
	session := &SyncSession{
		Planner: SyncPlanner{MaxBatch: 4},
		Cursor: SyncCursor{Height: 5},
	}
	msg, needed, err := session.NextRequestMessage(9, 4, 16)
	if err != nil {
		t.Fatalf("next request message: %v", err)
	}
	if !needed {
		t.Fatal("expected sync request")
	}
	if msg.Type != MessageTypeBlockRequest {
		t.Fatalf("unexpected message type: %v", msg.Type)
	}
	req, err := DecodeBlockRequest(msg.Payload, 4)
	if err != nil {
		t.Fatalf("decode request: %v", err)
	}
	if req.FromHeight != 6 || req.Limit != 4 {
		t.Fatalf("unexpected request: %+v", req)
	}
}

func TestSyncSessionNextRequestMessageCaughtUp(t *testing.T) {
	session := &SyncSession{
		Planner: SyncPlanner{MaxBatch: 4},
		Cursor: SyncCursor{Height: 9},
	}
	msg, needed, err := session.NextRequestMessage(9, 4, 16)
	if err != nil {
		t.Fatalf("caught-up request: %v", err)
	}
	if needed {
		t.Fatal("caught-up session unexpectedly requested sync")
	}
	if msg.Type != 0 || len(msg.Payload) != 0 {
		t.Fatalf("unexpected message for caught-up session: %+v", msg)
	}
}

func TestSyncSessionNextRequestMessageRejectsPayloadLimit(t *testing.T) {
	session := &SyncSession{
		Planner: SyncPlanner{MaxBatch: 1},
		Cursor: SyncCursor{Height: types.Height(1)},
	}
	_, needed, err := session.NextRequestMessage(2, 1, 15)
	if err != ErrInvalidMessage {
		t.Fatalf("expected invalid message error, got %v", err)
	}
	if needed != true {
		t.Fatal("expected request planning to be needed")
	}
}
