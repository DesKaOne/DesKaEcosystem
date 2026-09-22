package p2p

import (
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var ErrSyncCursorParentMismatch = errors.New("sync cursor parent mismatch")

// ValidateSyncCursorParent verifies that a non-empty batch starts from the
// cursor's known block hash.
func ValidateSyncCursorParent(cursor SyncCursor, req BlockRequest, resp BlockResponse) error {
	if len(resp.Blocks) == 0 {
		return nil
	}
	if req.FromHeight != cursor.Height+1 {
		return ErrSyncHeightMismatch
	}
	if resp.Blocks[0].Header.PreviousHash != cursor.BlockHash {
		return ErrSyncCursorParentMismatch
	}
	_ = types.Hash{}
	return nil
}
