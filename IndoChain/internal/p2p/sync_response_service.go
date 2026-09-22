package p2p

import (
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var ErrNilSyncResponseSession = errors.New("nil sync response session")

// SyncResponseService connects a validated block-response message to a
// session. It keeps message decoding separate from session state changes.
type SyncResponseService struct {
	Handler *SyncResponseHandler
}

// Handle applies one block-response message through the configured handler.
func (s *SyncResponseService) Handle(remoteHeight types.Height, msg Message) (bool, error) {
	if s == nil || s.Handler == nil {
		return false, ErrNilSyncResponseSession
	}
	return s.Handler.Handle(remoteHeight, msg)
}
