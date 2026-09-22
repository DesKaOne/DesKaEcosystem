package p2p

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestPlanNextSyncUsesCursorAndPlanner(t *testing.T) {
	planner := SyncPlanner{MaxBatch: 3}
	cursor := SyncCursor{Height: 4}

	req, ok, err := PlanNextSync(planner, cursor, types.Height(10))
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected request")
	}
	if req.FromHeight != 5 || req.Limit != 3 {
		t.Fatalf("got %+v, want from=5 limit=3", req)
	}
}

func TestPlanNextSyncReturnsNoRequestAtTip(t *testing.T) {
	planner := SyncPlanner{MaxBatch: 3}
	cursor := SyncCursor{Height: 10}

	_, ok, err := PlanNextSync(planner, cursor, types.Height(10))
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected no request")
	}
}
