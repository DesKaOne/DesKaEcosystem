package p2p

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestSyncPlanFromCursorStartsAfterCursor(t *testing.T) {
	planner := SyncPlanner{MaxBatch: 4}
	cursor := SyncCursor{Height: 7}

	req, ok, err := SyncPlanFromCursor(planner, cursor, types.Height(12))
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected sync request")
	}
	if req.FromHeight != 8 || req.Limit != 4 {
		t.Fatalf("got %+v, want from=8 limit=4", req)
	}
}

func TestSyncPlanFromCursorStopsAtRemoteTip(t *testing.T) {
	planner := SyncPlanner{MaxBatch: 4}
	cursor := SyncCursor{Height: 12}

	_, ok, err := SyncPlanFromCursor(planner, cursor, types.Height(12))
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected no request at remote tip")
	}
}
