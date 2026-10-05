package p2p

import "errors"

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
