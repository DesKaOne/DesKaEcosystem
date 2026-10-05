package p2p

import (
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

type plannedResponseImporter struct { imported []block.Block }
func (i *plannedResponseImporter) ImportBlock(b block.Block) error { i.imported = append(i.imported, b); return nil }

func TestApplyPlannedResponseAdvancesCursor(t *testing.T) {
	var parent types.Hash; parent[0] = 0x44
	first := block.Block{Header: block.Header{Height: 6, PreviousHash: parent}}
	firstHash, err := block.Hash(first); if err != nil { t.Fatal(err) }
	second := block.Block{Header: block.Header{Height: 7, PreviousHash: firstHash}}
	importer := &plannedResponseImporter{}
	c := &SyncCoordinator{Reader: cursorApplyReader{}, Importer: importer}
	cursor := SyncCursor{Height: 5, BlockHash: parent}
	next, applied, err := ApplyPlannedResponse(c, SyncPlanner{MaxBatch: 4}, cursor, 7, BlockResponse{Blocks: []block.Block{first, second}})
	if err != nil { t.Fatal(err) }; if !applied { t.Fatal("applied = false, want true") }
	if next.Height != 7 { t.Fatalf("height = %d, want 7", next.Height) }
	if len(importer.imported) != 2 { t.Fatalf("imported = %d, want 2", len(importer.imported)) }
}

func TestApplyPlannedResponseStopsWhenCaughtUp(t *testing.T) {
	var parent types.Hash; cursor := SyncCursor{Height: 7, BlockHash: parent}
	c := &SyncCoordinator{Reader: cursorApplyReader{}, Importer: &plannedResponseImporter{}}
	next, applied, err := ApplyPlannedResponse(c, SyncPlanner{MaxBatch: 4}, cursor, 7, BlockResponse{})
	if err != nil { t.Fatal(err) }; if applied { t.Fatal("applied = true, want false") }
	if next != cursor { t.Fatalf("cursor changed: %#v -> %#v", cursor, next) }
}

func TestApplyPlannedResponseRejectsOversizedResponse(t *testing.T) {
	var parent types.Hash; cursor := SyncCursor{Height: 5, BlockHash: parent}
	c := &SyncCoordinator{Reader: cursorApplyReader{}, Importer: &plannedResponseImporter{}}
	_, _, err := ApplyPlannedResponse(c, SyncPlanner{MaxBatch: 1}, cursor, 6, BlockResponse{Blocks: []block.Block{{Header: block.Header{Height: 6, PreviousHash: parent}}, {Header: block.Header{Height: 7, PreviousHash: parent}}}})
	if !errors.Is(err, ErrSyncResponseRequestMismatch) { t.Fatalf("error = %v, want %v", err, ErrSyncResponseRequestMismatch) }
}

func TestApplyPlannedResponseRejectsUnexpectedFirstHeight(t *testing.T) {
	var parent types.Hash; cursor := SyncCursor{Height: 5, BlockHash: parent}
	c := &SyncCoordinator{Reader: cursorApplyReader{}, Importer: &plannedResponseImporter{}}
	_, _, err := ApplyPlannedResponse(c, SyncPlanner{MaxBatch: 4}, cursor, 7, BlockResponse{Blocks: []block.Block{{Header: block.Header{Height: 7, PreviousHash: parent}}}})
	if !errors.Is(err, ErrSyncResponseRequestMismatch) { t.Fatalf("error = %v, want %v", err, ErrSyncResponseRequestMismatch) }
}
