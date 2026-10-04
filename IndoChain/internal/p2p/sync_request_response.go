package p2p

import "errors"

var ErrNilSyncRequestResponseEncoder = errors.New("nil sync request response encoder")

// HandleMessage executes one block request and wraps the resulting response
// using the existing development response-message boundary.
func (s *SyncRequestService) HandleMessage(msg Message, encoder SyncResponseEncoder, maxPayload uint32) (Message, error) {
	if s == nil || s.Handler == nil {
		return Message{}, ErrNilSyncRequestService
	}
	if encoder == nil {
		return Message{}, ErrNilSyncRequestResponseEncoder
	}
	resp, err := s.Handle(msg)
	if err != nil {
		return Message{}, err
	}
	return BuildBlockResponseMessage(resp, encoder, maxPayload)
}
