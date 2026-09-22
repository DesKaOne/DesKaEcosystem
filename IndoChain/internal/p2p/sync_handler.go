package p2p

import "errors"

var (
	ErrUnexpectedSyncMessage = errors.New("unexpected sync message")
)

// SyncMessageHandler connects the development P2P message envelope to the
// synchronization service without coupling it to a concrete network transport.
type SyncMessageHandler struct {
	Service *SyncService
	MaxPayload uint32
	MaxLimit uint64
}

func (h *SyncMessageHandler) Handle(msg Message) (BlockResponse, error) {
	if h == nil || h.Service == nil {
		return BlockResponse{}, ErrNilSyncService
	}
	if msg.Type != MessageTypeBlockRequest {
		return BlockResponse{}, ErrUnexpectedSyncMessage
	}
	if err := ValidateMessage(msg, h.MaxPayload); err != nil {
		return BlockResponse{}, err
	}
	req, err := DecodeBlockRequest(msg.Payload, h.MaxLimit)
	if err != nil {
		return BlockResponse{}, err
	}
	return h.Service.HandleBlockRequest(req)
}
