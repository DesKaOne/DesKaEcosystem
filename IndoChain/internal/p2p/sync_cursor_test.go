package p2p

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestSyncCursorAdvancesFromProgress(t *testing.T) {
	var hash types.Hash
	hash[0] = 0x42

	cursor := SyncCursor{Height: 5}
	progress := SyncProgress{
		Applied:       3,
		LastHeight:    8,
		LastBlockHash: hash,
	}

	next, err := cursor.Advance(progress)
	if err != nil {
		t.Fatal(err)
	}
	if next.Height != 8 {
		t.Fatalf("height = %d, want 8", next.Height)
	}
	if next.BlockHash != hash {
		t.Fatalf("block hash = %v, want %v", next.BlockHash, hash)
	}
}

func TestSyncCursorRejectsNonContiguousProgress(t *testing.T) {
	cursor := SyncCursor{Height: 5}
	progress := SyncProgress{
		Applied:    3,
		LastHeight: 9,
	}

	if _, err := cursor.Advance(progress); err == nil {
		t.Fatal("expected invalid progress error")
	}
}

func TestSyncCursorKeepsStateForEmptyProgress(t *testing.T) {
	var hash types.Hash
	hash[0] = 0x11
	cursor := SyncCursor{Height: 7, BlockHash: hash}

	next, err := cursor.Advance(SyncProgress{})
	if err != nil {
		t.Fatal(err)
	}
	if next != cursor {
		t.Fatalf("cursor changed: got %+v want %+v", next, cursor)
	}
}
