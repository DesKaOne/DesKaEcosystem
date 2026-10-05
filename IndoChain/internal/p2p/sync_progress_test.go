package p2p

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestSyncCoordinatorReturnsProgress(t *testing.T) {
	b1 := block.Block{Header: block.Header{Height: 4}}
	b2 := block.Block{Header: block.Header{Height: 5}}
	b1.Header.PreviousHash = types.Hash{1}
	h1, err := block.Hash(b1)
	if err != nil {
		t.Fatal(err)
	}
	b2.Header.PreviousHash = h1
	h2, err := block.Hash(b2)
	if err != nil {
		t.Fatal(err)
	}

	importer := &fakeImporter{}
	c := &SyncCoordinator{Reader: fakeReader{}, Importer: importer}
	progress, err := c.ApplyResponseWithProgress(
		BlockRequest{FromHeight: 4, Limit: 2},
		BlockResponse{Blocks: []block.Block{b1, b2}},
		b1.Header.PreviousHash,
	)
	if err != nil {
		t.Fatal(err)
	}
	if progress.Applied != 2 {
		t.Fatalf("applied = %d, want 2", progress.Applied)
	}
	if progress.LastHeight != 5 {
		t.Fatalf("last height = %d, want 5", progress.LastHeight)
	}
	if progress.LastBlockHash != h2 {
		t.Fatal("last block hash mismatch")
	}
}
