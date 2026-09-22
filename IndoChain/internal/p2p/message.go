package p2p

import (
	"encoding/binary"
	"errors"
)

var (
	ErrInvalidMessage = errors.New("invalid p2p message")
	ErrUnknownMessage = errors.New("unknown p2p message type")
)

type MessageType uint16

const (
	MessageTypeHandshake MessageType = iota + 1
	MessageTypeTransaction
	MessageTypeBlock
	MessageTypeBlockRequest
	MessageTypeBlockResponse
)

type Message struct {
	Type    MessageType
	Payload []byte
}

func ValidateMessage(msg Message, maxPayload uint32) error {
	if msg.Type < MessageTypeHandshake || msg.Type > MessageTypeBlockResponse {
		return ErrUnknownMessage
	}
	if len(msg.Payload) == 0 || uint32(len(msg.Payload)) > maxPayload {
		return ErrInvalidMessage
	}
	return nil
}

func EncodeMessage(msg Message, maxPayload uint32) ([]byte, error) {
	if err := ValidateMessage(msg, maxPayload); err != nil {
		return nil, err
	}
	out := make([]byte, 6+len(msg.Payload))
	binary.BigEndian.PutUint16(out[:2], uint16(msg.Type))
	binary.BigEndian.PutUint32(out[2:6], uint32(len(msg.Payload)))
	copy(out[6:], msg.Payload)
	return out, nil
}

func DecodeMessage(data []byte, maxPayload uint32) (Message, error) {
	if len(data) < 6 {
		return Message{}, ErrInvalidMessage
	}
	msgType := MessageType(binary.BigEndian.Uint16(data[:2]))
	payloadLen := binary.BigEndian.Uint32(data[2:6])
	if payloadLen == 0 || payloadLen > maxPayload {
		return Message{}, ErrInvalidMessage
	}
	if uint64(payloadLen)+6 != uint64(len(data)) {
		return Message{}, ErrInvalidMessage
	}
	msg := Message{Type: msgType, Payload: append([]byte(nil), data[6:]...)}
	if err := ValidateMessage(msg, maxPayload); err != nil {
		return Message{}, err
	}
	return msg, nil
}
