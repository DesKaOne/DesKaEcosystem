package p2p

import "errors"

var (
	ErrNilSyncResponseDecoder = errors.New("nil sync response decoder")
	ErrUnexpectedSyncResponse = errors.New("unexpected sync response")
)

// SyncResponseDecoder abstracts development deserialization of BlockResponse.
// Canonical block serialization remains outside this boundary until frozen.
type SyncResponseDecoder interface {
	DecodeBlockResponse(payload []byte) (BlockResponse, error)
}

// SyncResponseHandler connects a block-response message to a sync session.
// It deliberately leaves network transport and canonical serialization outside.
type SyncResponseHandler struct {
	Session    *SyncSession
	Decoder    SyncResponseDecoder
	MaxPayload uint32
}

// Handle decodes and applies one BlockResponse message. The session cursor is
// advanced only if the underlying sync application succeeds.
func (h *SyncResponseHandler) Handle(remoteHeight types.Height, msg Message) (bool, error) {
	if h == nil || h.Session == nil {
		return false, ErrNilSyncSession
	}
	if h.Decoder == nil {
		return false, ErrNilSyncResponseDecoder
	}
	if msg.Type != MessageTypeBlockResponse {
		return false, ErrUnexpectedSyncResponse
	}
	if err := ValidateMessage(msg, h.MaxPayload); err != nil {
		return false, err
	}
	resp, err := h.Decoder.DecodeBlockResponse(msg.Payload)
	if err != nil {
		return false, err
	}
	return h.Session.ApplyResponse(remoteHeight, resp)
}
