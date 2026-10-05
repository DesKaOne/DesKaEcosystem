package p2p

import (
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/transaction"
)

var (
	ErrUnsupportedGossip = errors.New("unsupported gossip message")
	ErrEmptyGossip      = errors.New("empty gossip payload")
)

type GossipMessage struct {
	Type    MessageType
	Payload any
}

func NewTransactionGossip(tx transaction.Transaction) GossipMessage {
	return GossipMessage{Type: MessageTypeTransaction, Payload: tx}
}

func NewBlockGossip(b block.Block) GossipMessage {
	return GossipMessage{Type: MessageTypeBlock, Payload: b}
}

func ValidateGossip(msg GossipMessage) error {
	switch msg.Type {
	case MessageTypeTransaction:
		if _, ok := msg.Payload.(transaction.Transaction); !ok {
			return ErrUnsupportedGossip
		}
	case MessageTypeBlock:
		if _, ok := msg.Payload.(block.Block); !ok {
			return ErrUnsupportedGossip
		}
	default:
		return ErrUnsupportedGossip
	}
	return nil
}
