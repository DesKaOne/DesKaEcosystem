package p2p

import "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"

// SyncPlanFromCursor derives the next bounded request from the caller's
// current canonical synchronization cursor.
func SyncPlanFromCursor(planner SyncPlanner, cursor SyncCursor, remoteHeight types.Height) (BlockRequest, bool, error) {
	return planner.Plan(cursor.Height, remoteHeight)
}
