package p2p

import (
	"errors"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var (
	ErrNilSyncReader      = errors.New("nil sync reader")
	ErrSyncHeightMismatch = errors.New("sync height mismatch")
	ErrSyncParentMismatch = errors.New("sync parent mismatch")
)

type BlockImporter interface {
	ImportBlock(b block.Block) error
}

type SyncCoordinator struct {
	Reader   SyncReader
	Importer BlockImporter
}

func (c *SyncCoordinator) ApplyResponse(req BlockRequest, resp BlockResponse, expectedParent types.Hash) error {
	_, err := c.ApplyResponseWithProgress(req, resp, expectedParent)
	return err
}

func (c *SyncCoordinator) ApplyResponseWithProgress(req BlockRequest, resp BlockResponse, expectedParent types.Hash) (SyncProgress, error) {
	if c == nil || c.Reader == nil || c.Importer == nil {
		return SyncProgress{}, ErrNilSyncReader
	}
	if req.Limit == 0 || uint64(len(resp.Blocks)) > req.Limit {
		return SyncProgress{}, ErrSyncHeightMismatch
	}
	if len(resp.Blocks) == 0 {
		return SyncProgress{}, nil
	}

	parent := expectedParent
	var lastHash types.Hash
	for i, b := range resp.Blocks {
		expectedHeight := req.FromHeight + types.Height(i)
		if b.Header.Height != expectedHeight {
			return SyncProgress{}, fmt.Errorf("%w: got %d want %d", ErrSyncHeightMismatch, b.Header.Height, expectedHeight)
		}
		if b.Header.PreviousHash != parent {
			return SyncProgress{}, ErrSyncParentMismatch
		}

		hash, err := block.Hash(b)
		if err != nil {
			return SyncProgress{}, err
		}
		if err := c.Importer.ImportBlock(b); err != nil {
			return SyncProgress{}, err
		}
		parent = hash
		lastHash = hash
	}

	return progressFromBlocks(resp.Blocks, uint64(len(resp.Blocks)), lastHash), nil
}
