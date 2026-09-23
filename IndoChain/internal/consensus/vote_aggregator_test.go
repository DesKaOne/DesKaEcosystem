package consensus

import (
	"bytes"
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func testVoteAggregator(t *testing.T) VoteAggregator {
	t.Helper()
	state, err := NewRoundState(1, 1001, 7, 42)
	if err != nil { t.Fatal(err) }
	validators, err := NewValidatorSet([][]byte{{1}, {2}, {3}})
	if err != nil { t.Fatal(err) }
	votingPower, err := NewVotingPowerSet([]ValidatorVotingPower{
		{ValidatorID: []byte{1}, Power: 2},
		{ValidatorID: []byte{2}, Power: 3},
		{ValidatorID: []byte{3}, Power: 5},
	})
	if err != nil { t.Fatal(err) }
	agg, err := NewVoteAggregator(ValidationRules{
		ProtocolVersion: 1, ChainID: 1001, MaxPayloadSize: 1024,
		RequireSender: true, RequireSignature: false,
	}, state, validators, votingPower)
	if err != nil { t.Fatal(err) }
	return agg
}

func makeVote(sender, payload []byte) Message {
	return Message{
		ProtocolVersion: 1, ChainID: 1001, Epoch: 7, Height: 42, Round: 0,
		Sender: append([]byte(nil), sender...), Type: MessageTypeVote,
		Payload: append([]byte(nil), payload...),
	}
}

func TestVoteAggregatorRejectsNonVote(t *testing.T) {
	agg := testVoteAggregator(t)
	err := agg.AddVote(Message{ProtocolVersion: 1, ChainID: 1001, Epoch: 7, Height: 42, Round: 0, Sender: []byte{1}, Type: MessageTypeProposal})
	if !errors.Is(err, ErrInvalidVote) { t.Fatalf("expected invalid vote, got %v", err) }
}

func TestVoteAggregatorRejectsDuplicateSender(t *testing.T) {
	agg := testVoteAggregator(t)
	if err := agg.AddVote(makeVote([]byte{1}, []byte("a"))); err != nil { t.Fatal(err) }
	err := agg.AddVote(makeVote([]byte{1}, []byte("b")))
	if !errors.Is(err, ErrDuplicateVote) { t.Fatalf("expected duplicate vote, got %v", err) }
}

func TestVoteAggregatorRejectsSenderWithoutVotingPower(t *testing.T) {
	agg := testVoteAggregator(t)
	err := agg.AddVote(makeVote([]byte{9}, []byte("a")))
	if !errors.Is(err, ErrConsensusMessageUnauthorized) {
		t.Fatalf("expected validator authorization error, got %v", err)
	}
}

func TestVoteAggregatorCalculatesPayloadPowerAndQuorum(t *testing.T) {
	agg := testVoteAggregator(t)
	for _, sender := range [][]byte{{1}, {2}} {
		if err := agg.AddVote(makeVote(sender, []byte("block-A"))); err != nil { t.Fatal(err) }
	}
	if err := agg.AddVote(makeVote([]byte{3}, []byte("block-B"))); err != nil { t.Fatal(err) }

	power, err := agg.VotingPowerForPayload([]byte("block-A"))
	if err != nil { t.Fatal(err) }
	if power != 5 { t.Fatalf("expected 5, got %d", power) }

	reached, err := agg.QuorumForPayload([]byte("block-A"), QuorumThreshold{Numerator: 1, Denominator: 2})
	if err != nil { t.Fatal(err) }
	if !reached { t.Fatal("expected quorum") }

	reached, err = agg.QuorumForPayload([]byte("block-B"), QuorumThreshold{Numerator: 1, Denominator: 2})
	if err != nil { t.Fatal(err) }
	if reached { t.Fatal("did not expect quorum") }
}

func TestVoteAggregatorClonesVoteInput(t *testing.T) {
	agg := testVoteAggregator(t)
	msg := makeVote([]byte{1}, []byte("payload"))
	if err := agg.AddVote(msg); err != nil { t.Fatal(err) }
	msg.Sender[0] = 9
	msg.Payload[0] = 'X'
	msg.Payload = append(msg.Payload, 'Y')
	if !bytes.Equal(agg.Votes[0].Sender, []byte{1}) || !bytes.Equal(agg.Votes[0].Payload, []byte("payload")) {
		t.Fatal("stored vote changed after input mutation")
	}
}

func TestVoteAggregatorRejectsContextMismatch(t *testing.T) {
	agg := testVoteAggregator(t)
	msg := makeVote([]byte{1}, []byte("a"))
	msg.Height = types.Height(43)
	err := agg.AddVote(msg)
	if !errors.Is(err, ErrConsensusMessageContextMismatch) {
		t.Fatalf("expected context mismatch, got %v", err)
	}
}
