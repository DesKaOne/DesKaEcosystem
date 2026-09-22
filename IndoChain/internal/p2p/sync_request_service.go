package p2p

import (
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var ErrNilSyncRequestService = errors.New("nil sync request service")

// SyncRequestService provides a small orchestration boundary around the
// development block-request message handler.
type SyncRequestService struct {
	Handler *SyncMessageHandler
}

// Handle validates and serves one block-request message through the configured
// sync handler. Transport remains outside this boundary.
func (s *SyncRequestService) Handle(msg Message) (BlockResponse, error) {
	if s == nil || s.Handler == nil {
		return BlockResponse{}, ErrNilSyncRequestService
	}
	return s.Handler.Handle(msg)
}

// _ keeps the protocol type dependency explicit for future request metadata
// extensions without coupling this service to a concrete transport.
var _ types.Height
