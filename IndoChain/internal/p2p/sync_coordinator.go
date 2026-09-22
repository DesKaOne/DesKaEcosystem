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
	if req.Limit == 0 || uint64(len(resp.Blocks)) > req.Limit {
		return ErrSyncHeightMismatch
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
		if b.Header.PreviousHash != parent {
			return ErrSyncParentMismatch
		}
		hash, err := block.Hash(b)
		if err != nil {
			return err
		}
		parent = hash
		if err := c.Importer.ImportBlock(b); err != nil {
			return err
		}
	}
	return nil
}
