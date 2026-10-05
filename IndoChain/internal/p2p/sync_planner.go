package p2p

import (
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var (
	ErrInvalidSyncRange = errors.New("invalid sync range")
)

type SyncPlanner struct {
	MaxBatch uint64
}

// Plan returns the next bounded request needed to move from localHeight
// toward remoteHeight. The local height is inclusive in the completed chain,
// so the next request starts at localHeight + 1.
func (p SyncPlanner) Plan(localHeight, remoteHeight types.Height) (BlockRequest, bool, error) {
	if p.MaxBatch == 0 {
		return BlockRequest{}, false, ErrInvalidSyncRange
	}
	if remoteHeight <= localHeight {
		return BlockRequest{}, false, nil
	}

	distance := uint64(remoteHeight - localHeight)
	limit := distance
	if limit > p.MaxBatch {
		limit = p.MaxBatch
	}
	return BlockRequest{
		FromHeight: localHeight + 1,
		Limit:      limit,
	}, true, nil
}
