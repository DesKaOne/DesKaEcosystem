package p2p

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

type cursorApplyImporter struct {
	imported []block.Block
}

func (i *cursorApplyImporter) ImportBlock(b block.Block) error {
	i.imported = append(i.imported, b)
	return nil
}

type cursorApplyReader struct{}

func (cursorApplyReader) BlockByHeight(height types.Height) (block.Block, types.Hash, error) {
	return block.Block{}, types.Hash{}, nil
}

func TestApplyResponseFromCursorAdvancesAfterSuccess(t *testing.T) {
	var parent types.Hash
	parent[0] = 0x10

	first := block.Block{
		Header: block.Header{
			Height:       6,
			PreviousHash: parent,
		},
	}
	firstHash, err := block.Hash(first)
	if err != nil {
		t.Fatal(err)
	}
	second := block.Block{
		Header: block.Header{
			Height:       7,
			PreviousHash: firstHash,
		},
	}

	importer := &cursorApplyImporter{}
	coordinator := &SyncCoordinator{
		Reader:   cursorApplyReader{},
		Importer: importer,
	}
	cursor := SyncCursor{Height: 5, BlockHash: parent}

	next, err := coordinator.ApplyResponseFromCursor(
		cursor,
		BlockRequest{FromHeight: 6, Limit: 2},
		BlockResponse{Blocks: []block.Block{first, second}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if next.Height != 7 {
		t.Fatalf("height = %d, want 7", next.Height)
	}
	secondHash, err := block.Hash(second)
	if err != nil {
		t.Fatal(err)
	}
	if next.BlockHash != secondHash {
		t.Fatalf("block hash = %v, want %v", next.BlockHash, secondHash)
	}
	if len(importer.imported) != 2 {
		t.Fatalf("imported = %d, want 2", len(importer.imported))
	}
}

func TestApplyResponseFromCursorKeepsCursorOnFailure(t *testing.T) {
	var parent types.Hash
	parent[0] = 0x22

	importer := &cursorApplyImporter{}
	coordinator := &SyncCoordinator{
		Reader:   cursorApplyReader{},
		Importer: importer,
	}
	cursor := SyncCursor{Height: 5, BlockHash: parent}

	_, err := coordinator.ApplyResponseFromCursor(
		cursor,
		BlockRequest{FromHeight: 6, Limit: 1},
		BlockResponse{Blocks: []block.Block{{
			Header: block.Header{
				Height:       6,
				PreviousHash: types.Hash{},
			},
		}}},
	)
	if err != ErrSyncParentMismatch {
		t.Fatalf("error = %v, want %v", err, ErrSyncParentMismatch)
	}
	if len(importer.imported) != 0 {
		t.Fatalf("imported = %d, want 0", len(importer.imported))
	}
}

func TestApplyResponseFromCursorRejectsInvalidRequestCursorMismatch(t *testing.T) {
	var parent types.Hash
	parent[0] = 0x33

	coordinator := &SyncCoordinator{
		Reader:   cursorApplyReader{},
		Importer: &cursorApplyImporter{},
	}
	cursor := SyncCursor{Height: 5, BlockHash: parent}

	_, err := coordinator.ApplyResponseFromCursor(
		cursor,
		BlockRequest{FromHeight: 8, Limit: 1},
		BlockResponse{Blocks: []block.Block{{
			Header: block.Header{
				Height:       8,
				PreviousHash: parent,
			},
		}}},
	)
	if err != nil {
		// The coordinator deliberately validates the request's expected height
		// independently from the cursor height; this case is valid at the
		// coordinator boundary and cursor advancement rejects the gap.
		return
	}
	t.Fatal("expected cursor advancement to reject a non-contiguous height")
}
