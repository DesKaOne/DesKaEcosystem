package consensus

import (
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestValidateMessageAgainstStateAcceptsExactContext(t *testing.T) {
	state, err := NewRoundState(1, 1001, 7, 12)
	if err != nil {
		t.Fatal(err)
	}
	msg := Message{
		ProtocolVersion: 1,
		ChainID:         1001,
		Epoch:           7,
		Height:          12,
		Round:           0,
		Type:            MessageTypeProposal,
	}
	if err := ValidateMessageAgainstState(state, msg); err != nil {
		t.Fatalf("expected matching context, got %v", err)
	}
}

func TestValidateMessageAgainstStateRejectsProtocolMismatch(t *testing.T) {
	state, _ := NewRoundState(1, 1001, 7, 12)
	msg := Message{ProtocolVersion: 2, ChainID: 1001, Epoch: 7, Height: 12, Round: 0, Type: MessageTypeProposal}
	if err := ValidateMessageAgainstState(state, msg); !errors.Is(err, ErrConsensusMessageContextMismatch) {
		t.Fatalf("expected context mismatch, got %v", err)
	}
}

func TestValidateMessageAgainstStateRejectsChainMismatch(t *testing.T) {
	state, _ := NewRoundState(1, 1001, 7, 12)
	msg := Message{ProtocolVersion: 1, ChainID: 1002, Epoch: 7, Height: 12, Round: 0, Type: MessageTypeProposal}
	if err := ValidateMessageAgainstState(state, msg); !errors.Is(err, ErrConsensusMessageContextMismatch) {
		t.Fatalf("expected context mismatch, got %v", err)
	}
}

func TestValidateMessageAgainstStateRejectsEpochMismatch(t *testing.T) {
	state, _ := NewRoundState(1, 1001, 7, 12)
	msg := Message{ProtocolVersion: 1, ChainID: 1001, Epoch: 8, Height: 12, Round: 0, Type: MessageTypeProposal}
	if err := ValidateMessageAgainstState(state, msg); !errors.Is(err, ErrConsensusMessageContextMismatch) {
		t.Fatalf("expected context mismatch, got %v", err)
	}
}

func TestValidateMessageAgainstStateRejectsHeightMismatch(t *testing.T) {
	state, _ := NewRoundState(1, 1001, 7, 12)
	msg := Message{ProtocolVersion: 1, ChainID: 1001, Epoch: 7, Height: 13, Round: 0, Type: MessageTypeProposal}
	if err := ValidateMessageAgainstState(state, msg); !errors.Is(err, ErrConsensusMessageContextMismatch) {
		t.Fatalf("expected context mismatch, got %v", err)
	}
}

func TestValidateMessageAgainstStateRejectsRoundMismatch(t *testing.T) {
	state, _ := NewRoundState(1, 1001, 7, 12)
	msg := Message{ProtocolVersion: 1, ChainID: 1001, Epoch: 7, Height: 12, Round: 1, Type: MessageTypeProposal}
	if err := ValidateMessageAgainstState(state, msg); !errors.Is(err, ErrConsensusMessageContextMismatch) {
		t.Fatalf("expected context mismatch, got %v", err)
	}
}

func TestValidateMessageAgainstStateRejectsInvalidState(t *testing.T) {
	state := RoundState{
		ProtocolVersion: 1,
		ChainID:         1001,
		Epoch:           7,
		Height:          types.Height(12),
		Round:           0,
		Phase:           Phase(99),
	}
	msg := Message{ProtocolVersion: 1, ChainID: 1001, Epoch: 7, Height: 12, Round: 0, Type: MessageTypeProposal}
	if err := ValidateMessageAgainstState(state, msg); !errors.Is(err, ErrInvalidConsensusPhase) {
		t.Fatalf("expected invalid phase, got %v", err)
	}
}
