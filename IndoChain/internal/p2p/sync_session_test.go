package p2p

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestSyncSessionAdvancesAcrossRounds(t *testing.T) {
	genesis := types.Hash{1}
	b1 := block.Block{Header: block.Header{Height: 1, PreviousHash: genesis}}
	h1, err := block.Hash(b1)
	if err != nil {
		t.Fatalf("hash b1: %v", err)
	}
	b2 := block.Block{Header: block.Header{Height: 2, PreviousHash: h1}}

	importer := &plannedResponseImporter{}
	session := &SyncSession{
		Planner:     SyncPlanner{MaxBatch: 1},
		Coordinator: &SyncCoordinator{
			Reader:   &cursorApplyReader{},
			Importer: importer,
		},
		Cursor: SyncCursor{Height: 0, BlockHash: genesis},
	}

	req, needed, err := session.NextRequest(2)
	if err != nil || !needed {
		t.Fatalf("next request: needed=%v err=%v", needed, err)
	}
	if req.FromHeight != 1 || req.Limit != 1 {
		t.Fatalf("unexpected first request: %+v", req)
	}

	advanced, err := session.ApplyResponse(2, BlockResponse{Blocks: []block.Block{b1}})
	if err != nil || !advanced {
		t.Fatalf("apply first response: advanced=%v err=%v", advanced, err)
	}
	if session.Cursor.Height != 1 || session.Cursor.BlockHash != h1 {
		t.Fatalf("unexpected first cursor: %+v", session.Cursor)
	}

	req, needed, err = session.NextRequest(2)
	if err != nil || !needed || req.FromHeight != 2 || req.Limit != 1 {
		t.Fatalf("unexpected second request: %+v needed=%v err=%v", req, needed, err)
	}

	advanced, err = session.ApplyResponse(2, BlockResponse{Blocks: []block.Block{b2}})
	if err != nil || !advanced {
		t.Fatalf("apply second response: advanced=%v err=%v", advanced, err)
	}
	if session.Cursor.Height != 2 {
		t.Fatalf("unexpected final cursor: %+v", session.Cursor)
	}
}

func TestSyncSessionCaughtUpIsNoOp(t *testing.T) {
	session := &SyncSession{
		Planner: SyncPlanner{MaxBatch: 2},
		Cursor: SyncCursor{Height: 3},
		Coordinator: &SyncCoordinator{},
	}
	req, needed, err := session.NextRequest(3)
	if err != nil || needed || req.Limit != 0 {
		t.Fatalf("expected caught-up no-op: req=%+v needed=%v err=%v", req, needed, err)
	}
}

func TestSyncSessionRejectsNilSession(t *testing.T) {
	var session *SyncSession
	if _, _, err := session.NextRequest(1); err != ErrNilSyncSession {
		t.Fatalf("expected nil session error, got %v", err)
	}
	if _, err := session.ApplyResponse(1, BlockResponse{}); err != ErrNilSyncSession {
		t.Fatalf("expected nil session error, got %v", err)
	}
}

func TestSyncSessionDoesNotAdvanceOnRejectedResponse(t *testing.T) {
	genesis := types.Hash{9}
	session := &SyncSession{
		Planner: SyncPlanner{MaxBatch: 2},
		Coordinator: &SyncCoordinator{
			Reader:   &cursorApplyReader{},
			Importer: &plannedResponseImporter{},
		},
		Cursor: SyncCursor{Height: 0, BlockHash: genesis},
	}
	_, err := session.ApplyResponse(1, BlockResponse{})
	if err != ErrSyncResponseRequestMismatch {
		t.Fatalf("expected response mismatch, got %v", err)
	}
	if session.Cursor.Height != 0 || session.Cursor.BlockHash != genesis {
		t.Fatalf("cursor advanced after rejected response: %+v", session.Cursor)
	}
}


func TestSyncSessionApplyWhenCaughtUpIsNoOp(t *testing.T) {
	genesis := types.Hash{4}
	session := &SyncSession{
		Planner: SyncPlanner{MaxBatch: 2},
		Coordinator: &SyncCoordinator{
			Reader:   &cursorApplyReader{},
			Importer: &plannedResponseImporter{},
		},
		Cursor: SyncCursor{Height: 2, BlockHash: genesis},
	}
	advanced, err := session.ApplyResponse(2, BlockResponse{})
	if err != nil {
		t.Fatalf("expected caught-up no-op, got %v", err)
	}
	if advanced {
		t.Fatal("caught-up session unexpectedly advanced")
	}
	if session.Cursor.Height != 2 || session.Cursor.BlockHash != genesis {
		t.Fatalf("cursor changed while caught up: %+v", session.Cursor)
	}
}

