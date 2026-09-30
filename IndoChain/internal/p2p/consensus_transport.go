package p2p

import "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"

// SendConsensus encodes a consensus message and carries it through the
// explicit node-to-node transport boundary.
func (t *InMemoryTransport) SendConsensus(peer PeerID, msg consensus.Message, rules consensus.ValidationRules) error {
	return SendConsensus(t, peer, msg, rules)
}

// SendConsensus carries one consensus message through any Transport.
func SendConsensus(t Transport, peer PeerID, msg consensus.Message, rules consensus.ValidationRules) error {
	if t == nil {
		return ErrNilTransport
	}
	payload, err := consensus.EncodeMessage(msg, rules)
	if err != nil {
		return err
	}
	return t.Send(peer, Message{Type: MessageTypeConsensus, Payload: payload})
}

// ReceiveConsensus receives and decodes one consensus message from any Transport.
func ReceiveConsensus(t Transport, rules consensus.ValidationRules) (PeerID, consensus.Message, error) {
	if t == nil {
		return "", consensus.Message{}, ErrNilTransport
	}
	from, msg, err := t.Receive()
	if err != nil {
		return "", consensus.Message{}, err
	}
	if msg.Type != MessageTypeConsensus {
		return from, consensus.Message{}, ErrUnknownMessage
	}
	decoded, err := consensus.DecodeMessage(msg.Payload, rules)
	if err != nil {
		return from, consensus.Message{}, err
	}
	return from, decoded, nil
}

// ReceiveConsensus receives and decodes one consensus message from the
// in-memory transport.
func (t *InMemoryTransport) ReceiveConsensus(rules consensus.ValidationRules) (PeerID, consensus.Message, error) {
	return ReceiveConsensus(t, rules)
}
