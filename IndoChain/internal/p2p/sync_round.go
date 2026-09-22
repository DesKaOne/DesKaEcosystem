package p2p

import (
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var ErrSyncResponseRequestMismatch = errors.New("sync response request mismatch")

// ApplyPlannedResponse applies a response only when it matches the request
// planned from the current cursor and remote height.
func ApplyPlannedResponse(coordinator *SyncCoordinator, planner SyncPlanner, cursor SyncCursor, remoteHeight types.Height, resp BlockResponse) (SyncCursor, bool, error) {
	if coordinator == nil { return cursor, false, ErrNilSyncCoordinator }
	req, needed, err := SyncPlanFromCursor(planner, cursor, remoteHeight)
	if err != nil { return cursor, false, err }
	if !needed { return cursor, false, nil }
	if uint64(len(resp.Blocks)) == 0 || uint64(len(resp.Blocks)) > req.Limit { return cursor, false, ErrSyncResponseRequestMismatch }
	if resp.Blocks[0].Header.Height != req.FromHeight { return cursor, false, ErrSyncResponseRequestMismatch }
	next, err := coordinator.ApplyResponseFromCursor(cursor, req, resp)
	if err != nil { return cursor, false, err }
	return next, true, nil
}
