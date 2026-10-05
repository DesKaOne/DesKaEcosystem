package p2p

// BuildBlockRequestMessage creates the development P2P envelope used to ask
// a peer for a block range.
func BuildBlockRequestMessage(req BlockRequest, maxLimit uint64, maxPayload uint32) (Message, error) {
	payload, err := EncodeBlockRequest(req, maxLimit)
	if err != nil {
		return Message{}, err
	}
	if uint32(len(payload)) > maxPayload {
		return Message{}, ErrInvalidMessage
	}
	return Message{
		Type:    MessageTypeBlockRequest,
		Payload: payload,
	}, nil
}
