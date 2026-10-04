package p2p

import (
	"errors"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var ErrInvalidSyncProgress = errors.New("invalid sync progress")

// SyncCursor tracks the canonical tip known by a sync driver.
// It is advanced only from successful SyncProgress results.
type SyncCursor struct {
	Height    types.Height
	BlockHash types.Hash
}

func (c SyncCursor) Advance(progress SyncProgress) (SyncCursor, error) {
	if progress.Applied == 0 {
		return c, nil
	}

	expectedHeight := uint64(c.Height) + progress.Applied
	if uint64(progress.LastHeight) != expectedHeight {
		return c, fmt.Errorf("%w: got last height %d want %d", ErrInvalidSyncProgress, progress.LastHeight, expectedHeight)
	}

	return SyncCursor{
		Height:    progress.LastHeight,
		BlockHash: progress.LastBlockHash,
	}, nil
}
