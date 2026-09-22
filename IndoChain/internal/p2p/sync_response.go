package p2p

import "errors"

var ErrNilSyncResponseEncoder = errors.New("nil sync response encoder")

// SyncResponseEncoder abstracts development serialization of BlockResponse.
// Canonical block serialization remains outside this boundary until frozen.
type SyncResponseEncoder interface {
	EncodeBlockResponse(resp BlockResponse) ([]byte, error)
}

// BuildBlockResponseMessage wraps an already-encoded block response in the
// development P2P envelope. It does not define canonical block encoding.
func BuildBlockResponseMessage(resp BlockResponse, encoder SyncResponseEncoder, maxPayload uint32) (Message, error) {
	if encoder == nil {
		return Message{}, ErrNilSyncResponseEncoder
	}
	payload, err := encoder.EncodeBlockResponse(resp)
	if err != nil {
		return Message{}, err
	}
	msg := Message{
		Type:    MessageTypeBlockResponse,
		Payload: payload,
	}
	if err := ValidateMessage(msg, maxPayload); err != nil {
		return Message{}, err
	}
	return msg, nil
}
