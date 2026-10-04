package consensus

import (
	"bytes"
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func finalizedBlockFixture(t *testing.T) (BlockProductionContext, block.Block, FinalityCertificate, ValidatorSet, VotingPowerSet) {
	t.Helper()
	state := RoundState{ProtocolVersion: 1, ChainID: 1001, Epoch: 1, Height: 7, Round: 2, Phase: PhasePrecommit}
	validators, err := NewValidatorSet([][]byte{[]byte("validator-a"), []byte("validator-b")})
	if err != nil { t.Fatal(err) }
	power, err := NewVotingPowerSet([]ValidatorVotingPower{{ValidatorID: []byte("validator-a"), Power: 2}, {ValidatorID: []byte("validator-b"), Power: 2}})
	if err != nil { t.Fatal(err) }
	ctx := BlockProductionContext{State: state, PreviousHash: types.Hash{9}, Proposer: []byte("validator-a")}
	candidate := block.Block{Header: block.Header{Version: 1, ChainID: 1001, Height: 8, PreviousHash: ctx.PreviousHash, Proposer: append([]byte(nil), ctx.Proposer...)}}
	payload, err := ValidateProducedBlock(ctx, candidate)
	if err != nil { t.Fatal(err) }
	votes := []Message{
		{ProtocolVersion: 1, ChainID: 1001, Epoch: 1, Height: 7, Round: 2, Sender: []byte("validator-a"), Type: MessageTypeVote, Payload: payload[:]},
		{ProtocolVersion: 1, ChainID: 1001, Epoch: 1, Height: 7, Round: 2, Sender: []byte("validator-b"), Type: MessageTypeVote, Payload: payload[:]},
	}
	certificate, err := NewFinalityCertificate(state, validators, power, QuorumThreshold{Numerator: 1, Denominator: 2}, payload[:], votes)
	if err != nil { t.Fatal(err) }
	return ctx, candidate, certificate, validators, power
}

func TestValidateFinalizedBlockBindsCertificateToCandidate(t *testing.T) {
	ctx, candidate, certificate, validators, power := finalizedBlockFixture(t)
	payload, err := ValidateFinalizedBlock(ctx, candidate, certificate, validators, power)
	if err != nil { t.Fatalf("ValidateFinalizedBlock() error = %v", err) }
	if !bytes.Equal(payload[:], certificate.Payload) { t.Fatal("returned payload does not match certificate") }
}

func TestValidateFinalizedBlockRejectsDifferentCandidate(t *testing.T) {
	ctx, candidate, certificate, validators, power := finalizedBlockFixture(t)
	candidate.Header.Timestamp = 1
	if _, err := ValidateFinalizedBlock(ctx, candidate, certificate, validators, power); !errors.Is(err, ErrFinalizedBlockMismatch) {
		t.Fatalf("expected finalized block mismatch, got %v", err)
	}
}
