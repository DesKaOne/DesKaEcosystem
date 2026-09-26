package p2p

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/transaction"
)

func TestGossipValidation(t *testing.T) {
	tx := transaction.Transaction{Value: 1}
	if err := ValidateGossip(NewTransactionGossip(tx)); err != nil {
		t.Fatalf("transaction gossip rejected: %v", err)
	}

	b := block.Block{}
	if err := ValidateGossip(NewBlockGossip(b)); err != nil {
		t.Fatalf("block gossip rejected: %v", err)
	}
}

func TestGossipRejectsWrongPayload(t *testing.T) {
	if err := ValidateGossip(GossipMessage{Type: MessageTypeTransaction, Payload: block.Block{}}); err != ErrUnsupportedGossip {
		t.Fatalf("error = %v, want %v", err, ErrUnsupportedGossip)
	}
	if err := ValidateGossip(GossipMessage{Type: MessageTypeBlock, Payload: transaction.Transaction{}}); err != ErrUnsupportedGossip {
		t.Fatalf("error = %v, want %v", err, ErrUnsupportedGossip)
	}
	if err := ValidateGossip(GossipMessage{Type: MessageTypeHandshake, Payload: []byte("x")}); err != ErrUnsupportedGossip {
		t.Fatalf("error = %v, want %v", err, ErrUnsupportedGossip)
	}
}
