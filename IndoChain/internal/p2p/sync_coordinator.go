package p2p

import (
	"errors"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var (
	ErrNilSyncReader       = errors.New("nil sync reader")
	ErrSyncHeightMismatch  = errors.New("sync height mismatch")
	ErrSyncParentMismatch  = errors.New("sync parent mismatch")
)

type BlockImporter interface {
	ImportBlock(b block.Block) error
}

type SyncCoordinator struct {
	Reader   SyncReader
	Importer BlockImporter
}

func (c *SyncCoordinator) ApplyResponse(req BlockRequest, resp BlockResponse, expectedParent types.Hash) error {
	if c == nil || c.Reader == nil || c.Importer == nil {
		return ErrNilSyncReader
	}
	if err := ValidateBlockRequest(req, uint64(len(resp.Blocks))); err != nil && len(resp.Blocks) > 0 {
		return fmt.Errorf("%w: %v", ErrSyncHeightMismatch, err)
	}
	if len(resp.Blocks) == 0 {
		return nil
	}
	parent := expectedParent
	for i, b := range resp.Blocks {
		expectedHeight := req.FromHeight + types.Height(i)
		if b.Header.Height != expectedHeight {
			return fmt.Errorf("%w: got %d want %d", ErrSyncHeightMismatch, b.Header.Height, expectedHeight)
		}
		if i > 0 && b.Header.PreviousHash != parent {
			return ErrSyncParentMismatch
		}
		parent = block.Hash(b)
		if err := c.Importer.ImportBlock(b); err != nil {
			return err
		}
	}
	return nil
}
