package p2p

import (
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var (
	ErrInvalidSyncRequest = errors.New("invalid sync request")
	ErrSyncReadFailure    = errors.New("sync read failure")
)

type BlockRequest struct {
	FromHeight types.Height
	Limit      uint64
}

type BlockResponse struct {
	Blocks []block.Block
}

type SyncReader interface {
	BlockByHeight(height types.Height) (block.Block, types.Hash, error)
}

func ValidateBlockRequest(req BlockRequest, maxLimit uint64) error {
	if maxLimit == 0 || req.Limit == 0 || req.Limit > maxLimit {
		return ErrInvalidSyncRequest
	}
	return nil
}
