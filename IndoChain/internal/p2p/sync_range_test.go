package p2p

import (
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

type rangeReader struct {
	blocks map[types.Height]block.Block
	err    error
}

func (r rangeReader) BlockByHeight(height types.Height) (block.Block, types.Hash, error) {
	if r.err != nil {
		return block.Block{}, types.Hash{}, r.err
	}
	b, ok := r.blocks[height]
	if !ok {
		return block.Block{}, types.Hash{}, errors.New("missing block")
	}
	return b, types.Hash{1}, nil
}

func TestReadBlockRange(t *testing.T) {
	r := rangeReader{blocks: map[types.Height]block.Block{
		3: {Header: block.Header{Height: 3}},
		4: {Header: block.Header{Height: 4}},
	}}
	resp, err := ReadBlockRange(r, BlockRequest{FromHeight: 3, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Blocks) != 2 || resp.Blocks[0].Header.Height != 3 || resp.Blocks[1].Header.Height != 4 {
		t.Fatalf("unexpected range: %+v", resp.Blocks)
	}
}

func TestReadBlockRangeStopsAtMissingTail(t *testing.T) {
	r := rangeReader{blocks: map[types.Height]block.Block{
		3: {Header: block.Header{Height: 3}},
	}}
	resp, err := ReadBlockRange(r, BlockRequest{FromHeight: 3, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Blocks) != 1 {
		t.Fatalf("got %d blocks, want 1", len(resp.Blocks))
	}
}

func TestReadBlockRangeRejectsGap(t *testing.T) {
	r := rangeReader{blocks: map[types.Height]block.Block{
		3: {Header: block.Header{Height: 9}},
	}}
	_, err := ReadBlockRange(r, BlockRequest{FromHeight: 3, Limit: 1})
	if !errors.Is(err, ErrSyncRangeGap) {
		t.Fatalf("error = %v, want %v", err, ErrSyncRangeGap)
	}
}
