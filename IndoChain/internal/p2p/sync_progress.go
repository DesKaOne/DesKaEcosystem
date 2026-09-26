package p2p

import (
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

type SyncProgress struct {
	Applied        uint64
	LastHeight     types.Height
	LastBlockHash  types.Hash
}

func progressFromBlocks(blocks []block.Block, applied uint64, lastHash types.Hash) SyncProgress {
	if len(blocks) == 0 {
		return SyncProgress{}
	}
	return SyncProgress{
		Applied:       applied,
		LastHeight:    blocks[len(blocks)-1].Header.Height,
		LastBlockHash: lastHash,
	}
}
