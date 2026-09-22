package p2p

import (
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/node"
)

var (
	ErrInvalidSyncRequest = errors.New("invalid sync request")
	ErrSyncReadFailure = errors.New("sync read failure")
)

type BlockRequest struct {
	FromHeight types.Height
	Limit      uint64
}

type BlockResponse struct {
	Blocks []node.ChainBlock
}

type SyncReader interface {
	BlocksByRange(from types.Height, limit uint64) ([]node.ChainBlock, error)
}

func ValidateBlockRequest(req BlockRequest, maxLimit uint64) error {
	if maxLimit == 0 || req.Limit == 0 || req.Limit > maxLimit {
		return ErrInvalidSyncRequest
	}
	return nil
}
