package p2p

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestSyncPlannerPlansBoundedRange(t *testing.T) {
	p := SyncPlanner{MaxBatch: 10}
	req, ok, err := p.Plan(types.Height(5), types.Height(20))
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected a sync request")
	}
	if req.FromHeight != 6 || req.Limit != 10 {
		t.Fatalf("got %+v, want from=6 limit=10", req)
	}
}

func TestSyncPlannerStopsAtRemoteHeight(t *testing.T) {
	p := SyncPlanner{MaxBatch: 10}
	_, ok, err := p.Plan(types.Height(20), types.Height(20))
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected no request when heights match")
	}
}

func TestSyncPlannerRejectsZeroBatch(t *testing.T) {
	p := SyncPlanner{}
	if _, _, err := p.Plan(1, 2); err != ErrInvalidSyncRange {
		t.Fatalf("error = %v, want %v", err, ErrInvalidSyncRange)
	}
}
