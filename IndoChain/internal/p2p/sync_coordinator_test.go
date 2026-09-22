package p2p

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

type fakeImporter struct {
	blocks []block.Block
	err error
}

func (f *fakeImporter) ImportBlock(b block.Block) error {
	if f.err != nil {
		return f.err
	}
	f.blocks = append(f.blocks, b)
	return nil
}

type fakeReader struct{}

func (fakeReader) BlockByHeight(height types.Height) (block.Block, types.Hash, error) {
	return block.Block{}, types.Hash{}, nil
}

func TestSyncCoordinatorAppliesOrderedBlocks(t *testing.T) {
	b1 := block.Block{Header: block.Header{Height: 1}}
	b1.Header.PreviousHash = types.Hash{1}
	b2 := block.Block{Header: block.Header{Height: 2}}
	b2.Header.PreviousHash = block.Hash(b1)

	importer := &fakeImporter{}
	c := &SyncCoordinator{Reader: fakeReader{}, Importer: importer}
	if err := c.ApplyResponse(BlockRequest{FromHeight: 1, Limit: 2}, BlockResponse{Blocks: []block.Block{b1, b2}}, b1.Header.PreviousHash); err != nil {
		t.Fatal(err)
	}
	if len(importer.blocks) != 2 {
		t.Fatalf("imported %d blocks, want 2", len(importer.blocks))
	}
}

func TestSyncCoordinatorRejectsHeightMismatch(t *testing.T) {
	c := &SyncCoordinator{Reader: fakeReader{}, Importer: &fakeImporter{}}
	err := c.ApplyResponse(BlockRequest{FromHeight: 1, Limit: 1}, BlockResponse{Blocks: []block.Block{{Header: block.Header{Height: 3}}}}, types.Hash{})
	if err == nil {
		t.Fatal("expected height mismatch")
	}
}

func TestSyncCoordinatorRejectsParentMismatch(t *testing.T) {
	b1 := block.Block{Header: block.Header{Height: 1}}
	b2 := block.Block{Header: block.Header{Height: 2, PreviousHash: types.Hash{9}}}
	c := &SyncCoordinator{Reader: fakeReader{}, Importer: &fakeImporter{}}
	err := c.ApplyResponse(BlockRequest{FromHeight: 1, Limit: 2}, BlockResponse{Blocks: []block.Block{b1, b2}}, types.Hash{})
	if err != ErrSyncParentMismatch {
		t.Fatalf("error = %v, want %v", err, ErrSyncParentMismatch)
	}
}
