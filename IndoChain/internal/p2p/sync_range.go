package p2p

import (
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var ErrSyncRangeGap = fmt.Errorf("sync range gap")

// ReadBlockRange reads a bounded contiguous block range through the read-only
// SyncReader boundary. It does not mutate state or storage.
func ReadBlockRange(reader SyncReader, req BlockRequest) (BlockResponse, error) {
	if reader == nil {
		return BlockResponse{}, ErrNilSyncReader
	}
	if err := ValidateBlockRequest(req, req.Limit); err != nil {
		return BlockResponse{}, err
	}
	blocks := make([]block.Block, 0, req.Limit)
	for i := uint64(0); i < req.Limit; i++ {
		height := req.FromHeight + types.Height(i)
		b, _, err := reader.BlockByHeight(height)
		if err != nil {
			if len(blocks) == 0 {
				return BlockResponse{}, err
			}
			break
		}
		if b.Header.Height != height {
			return BlockResponse{}, fmt.Errorf("%w: got %d want %d", ErrSyncRangeGap, b.Header.Height, height)
		}
		blocks = append(blocks, b)
	}
	return BlockResponse{Blocks: blocks}, nil
}
