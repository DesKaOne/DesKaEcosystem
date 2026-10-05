package p2p

import (
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestSyncServiceHandlesBlockRequest(t *testing.T) {
	r := rangeReader{blocks: map[types.Height]block.Block{
		5: {Header: block.Header{Height: 5}},
		6: {Header: block.Header{Height: 6}},
	}}
	s := &SyncService{Reader: r, MaxLimit: 2}
	resp, err := s.HandleBlockRequest(BlockRequest{FromHeight: 5, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Blocks) != 2 {
		t.Fatalf("got %d blocks, want 2", len(resp.Blocks))
	}
}

func TestSyncServiceRejectsOverLimit(t *testing.T) {
	s := &SyncService{Reader: rangeReader{}, MaxLimit: 2}
	_, err := s.HandleBlockRequest(BlockRequest{FromHeight: 1, Limit: 3})
	if !errors.Is(err, ErrInvalidSyncRequest) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidSyncRequest)
	}
}

func TestSyncServiceWrapsReadFailure(t *testing.T) {
	s := &SyncService{Reader: rangeReader{err: errors.New("boom")}, MaxLimit: 2}
	_, err := s.HandleBlockRequest(BlockRequest{FromHeight: 1, Limit: 1})
	if !errors.Is(err, ErrSyncReadFailure) {
		t.Fatalf("error = %v, want %v", err, ErrSyncReadFailure)
	}
}
