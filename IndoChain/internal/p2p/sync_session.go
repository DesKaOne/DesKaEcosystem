package p2p

import (
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var ErrNilSyncSession = errors.New("nil sync session")

// SyncSession holds the minimal state needed to perform repeated bounded
// synchronization rounds from a local cursor toward a remote height.
type SyncSession struct {
	Planner     SyncPlanner
	Coordinator *SyncCoordinator
	Cursor      SyncCursor
}

// NextRequest returns the next bounded request, or false when the cursor is
// already caught up with the remote height.
func (s *SyncSession) NextRequest(remoteHeight types.Height) (BlockRequest, bool, error) {
	if s == nil {
		return BlockRequest{}, false, ErrNilSyncSession
	}
	return SyncPlanFromCursor(s.Planner, s.Cursor, remoteHeight)
}

// ApplyResponse validates and applies one planned response, then advances the
// session cursor only after the response has been imported successfully.
func (s *SyncSession) ApplyResponse(remoteHeight types.Height, resp BlockResponse) (bool, error) {
	if s == nil || s.Coordinator == nil {
		return false, ErrNilSyncSession
	}
	next, advanced, err := ApplyPlannedResponse(
		s.Coordinator,
		s.Planner,
		s.Cursor,
		remoteHeight,
		resp,
	)
	if err != nil {
		return false, err
	}
	if advanced {
		s.Cursor = next
	}
	return advanced, nil
}
