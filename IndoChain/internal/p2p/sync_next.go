package p2p

import "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"

// PlanNextSync derives the next request directly from the current cursor.
func PlanNextSync(planner SyncPlanner, cursor SyncCursor, remoteHeight types.Height) (BlockRequest, bool, error) {
	return SyncPlanFromCursor(planner, cursor, remoteHeight)
}
