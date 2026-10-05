package p2p

import (
	"errors"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var (
	ErrNilSyncService = errors.New("nil sync service")
	ErrSyncLimitExceeded = errors.New("sync limit exceeded")
)

type SyncService struct {
	Reader   SyncReader
	MaxLimit uint64
}

func (s *SyncService) HandleBlockRequest(req BlockRequest) (BlockResponse, error) {
	if s == nil || s.Reader == nil {
		return BlockResponse{}, ErrNilSyncService
	}
	if err := ValidateBlockRequest(req, s.MaxLimit); err != nil {
		return BlockResponse{}, err
	}
	resp, err := ReadBlockRange(s.Reader, req)
	if err != nil {
		return BlockResponse{}, fmt.Errorf("%w: %v", ErrSyncReadFailure, err)
	}
	if uint64(len(resp.Blocks)) > s.MaxLimit {
		return BlockResponse{}, ErrSyncLimitExceeded
	}
	return normalizeBlockResponse(req, resp)
}

func normalizeBlockResponse(req BlockRequest, resp BlockResponse) (BlockResponse, error) {
	for i, b := range resp.Blocks {
		expected := req.FromHeight + types.Height(i)
		if b.Header.Height != expected {
			return BlockResponse{}, fmt.Errorf("%w: got %d want %d", ErrInvalidSyncRequest, b.Header.Height, expected)
		}
	}
	return BlockResponse{Blocks: append([]block.Block(nil), resp.Blocks...)}, nil
}
