package p2p

import (
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var (
	ErrProtocolMismatch = errors.New("protocol version mismatch")
	ErrInvalidHandshake = errors.New("invalid handshake")
)

type Handshake struct {
	PeerID          PeerID
	ProtocolVersion types.ProtocolVersion
	ChainID         types.ChainID
	NetworkProfile  string
}

type HandshakeRules struct {
	ProtocolVersion types.ProtocolVersion
	ChainID         types.ChainID
	NetworkProfile  string
}

func ValidateHandshake(h Handshake, rules HandshakeRules) error {
	if h.PeerID == "" || h.NetworkProfile == "" {
		return ErrInvalidHandshake
	}
	if h.ProtocolVersion != rules.ProtocolVersion || h.ChainID != rules.ChainID {
		return ErrProtocolMismatch
	}
	if h.NetworkProfile != rules.NetworkProfile {
		return ErrProtocolMismatch
	}
	return nil
}
