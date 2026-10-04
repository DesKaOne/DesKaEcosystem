package p2p

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestValidateSyncCursorParent(t *testing.T) {
	var parent types.Hash
	parent[0] = 0x44
	cursor := SyncCursor{Height: 5, BlockHash: parent}
	resp := BlockResponse{Blocks: []block.Block{{
		Header: block.Header{
			Height:       6,
			PreviousHash: parent,
		},
	}}}

	if err := ValidateSyncCursorParent(cursor, BlockRequest{FromHeight: 6, Limit: 1}, resp); err != nil {
		t.Fatal(err)
	}
}

func TestValidateSyncCursorParentRejectsWrongStartHeight(t *testing.T) {
	var parent types.Hash
	cursor := SyncCursor{Height: 5, BlockHash: parent}
	resp := BlockResponse{Blocks: []block.Block{{
		Header: block.Header{Height: 7, PreviousHash: parent},
	}}}

	if err := ValidateSyncCursorParent(cursor, BlockRequest{FromHeight: 7, Limit: 1}, resp); err != ErrSyncHeightMismatch {
		t.Fatalf("error = %v, want %v", err, ErrSyncHeightMismatch)
	}
}

func TestValidateSyncCursorParentRejectsWrongParent(t *testing.T) {
	var parent types.Hash
	parent[0] = 0x55
	cursor := SyncCursor{Height: 5, BlockHash: parent}
	resp := BlockResponse{Blocks: []block.Block{{
		Header: block.Header{
			Height:       6,
			PreviousHash: types.Hash{},
		},
	}}}

	if err := ValidateSyncCursorParent(cursor, BlockRequest{FromHeight: 6, Limit: 1}, resp); err != ErrSyncCursorParentMismatch {
		t.Fatalf("error = %v, want %v", err, ErrSyncCursorParentMismatch)
	}
}

func TestValidateSyncCursorParentAllowsEmptyResponse(t *testing.T) {
	cursor := SyncCursor{Height: 5}
	if err := ValidateSyncCursorParent(cursor, BlockRequest{FromHeight: 6, Limit: 1}, BlockResponse{}); err != nil {
		t.Fatal(err)
	}
}
