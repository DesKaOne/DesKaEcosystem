package p2p

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestValidateHandshake(t *testing.T) {
	rules := HandshakeRules{
		ProtocolVersion: 1,
		ChainID:         1001,
		NetworkProfile:  "devnet",
	}
	valid := Handshake{
		PeerID:          "peer-1",
		ProtocolVersion: 1,
		ChainID:         1001,
		NetworkProfile:  "devnet",
	}
	if err := ValidateHandshake(valid, rules); err != nil {
		t.Fatalf("valid handshake rejected: %v", err)
	}

	tests := []Handshake{
		{PeerID: "peer-1", ProtocolVersion: 2, ChainID: 1001, NetworkProfile: "devnet"},
		{PeerID: "peer-1", ProtocolVersion: 1, ChainID: 1002, NetworkProfile: "devnet"},
		{PeerID: "peer-1", ProtocolVersion: 1, ChainID: 1001, NetworkProfile: "testnet"},
	}
	for _, tc := range tests {
		if err := ValidateHandshake(tc, rules); err != ErrProtocolMismatch {
			t.Fatalf("error = %v, want %v", err, ErrProtocolMismatch)
		}
	}

	if err := ValidateHandshake(Handshake{
		PeerID: "peer-1",
		ProtocolVersion: types.ProtocolVersion(1),
		ChainID: types.ChainID(1001),
		NetworkProfile: "devnet",
	}, rules); err != nil {
		t.Fatalf("typed handshake rejected: %v", err)
	}
}

func TestValidateHandshakeRejectsMalformedIdentity(t *testing.T) {
	rules := HandshakeRules{ProtocolVersion: 1, ChainID: 1001, NetworkProfile: "devnet"}
	if err := ValidateHandshake(Handshake{ProtocolVersion: 1, ChainID: 1001, NetworkProfile: "devnet"}, rules); err != ErrInvalidHandshake {
		t.Fatalf("error = %v, want %v", err, ErrInvalidHandshake)
	}
}
