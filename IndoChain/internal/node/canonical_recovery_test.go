package node

import (
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

type finalizedRecoveryFixture struct {
	node        *Node
	ctx         consensus.BlockProductionContext
	candidate   block.Block
	certificate consensus.FinalityCertificate
	validators  consensus.ValidatorSet
	power       consensus.VotingPowerSet
	authority   consensus.ValidatorAuthoritySet
	signerA     *crypto.Ed25519Signer
}

func buildFinalizedRecoveryFixture(t *testing.T) finalizedRecoveryFixture {
	t.Helper()
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store)
	if err != nil { t.Fatal(err) }
	validators, power := recoveryValidatorConfig(t)
	signerA := testEd25519Signer(t, 31)
	signerB := testEd25519Signer(t, 32)
	signerC := testEd25519Signer(t, 33)
	authority, err := consensus.NewValidatorAuthoritySet(0, validators, map[string][]byte{
		"validator-a": signerA.PublicKey(),
		"validator-b": signerB.PublicKey(),
		"validator-c": signerC.PublicKey(),
	})
	if err != nil { t.Fatal(err) }
	resolver, err := authority.ConsensusAuthorityResolver()
	if err != nil { t.Fatal(err) }
	recovery, err := n.ReconstructConsensusRuntimeWithAuthority(
		0, validators, power,
		consensus.QuorumThreshold{Numerator: 2, Denominator: 3},
		consensus.RoundRobinProposer{}, authority,
	)
	if err != nil { t.Fatal(err) }
	rules, err := n.Config.BlockRules(nil)
	if err != nil { t.Fatal(err) }
	proposal, err := recovery.BuildNextBlockProposalFromRecovery(
		n.State, n.Head.Header.Timestamp+1, nil, nil, rules,
	)
	if err != nil { t.Fatal(err) }
	if err := recovery.Runtime.AcceptBlockProposal(proposal); err != nil { t.Fatal(err) }
	payload := proposal.MessagePayload()
	makeVote := func(id string, signer *crypto.Ed25519Signer, typ consensus.MessageType) consensus.Message {
		msg := consensus.Message{
			ProtocolVersion: recovery.State.ProtocolVersion,
			ChainID: recovery.State.ChainID,
			Epoch: recovery.State.Epoch,
			Height: recovery.State.Height,
			Round: recovery.State.Round,
			Sender: []byte(id),
			Type: typ,
			Payload: payload,
		}
		signed, err := msg.Sign(signer)
		if err != nil { t.Fatal(err) }
		return signed
	}
	for _, vote := range []consensus.Message{
		makeVote("validator-a", signerA, consensus.MessageTypePrevote),
		makeVote("validator-b", signerB, consensus.MessageTypePrevote),
		makeVote("validator-a", signerA, consensus.MessageTypePrecommit),
		makeVote("validator-b", signerB, consensus.MessageTypePrecommit),
	} {
		if err := recovery.Runtime.AddVote(vote); err != nil { t.Fatal(err) }
	}
	certificate, err := recovery.Runtime.FinalizeProposal(resolver)
	if err != nil { t.Fatal(err) }
	ctx, err := recovery.NextBlockContext()
	if err != nil { t.Fatal(err) }
	return finalizedRecoveryFixture{
		node: n, ctx: ctx, candidate: proposal.Candidate,
		certificate: certificate, validators: validators, power: power,
		authority: authority, signerA: signerA,
	}
}

func TestClassifyFinalizedCommitDistinguishesMissingAndMatchedCanonicalState(t *testing.T) {
	f := buildFinalizedRecoveryFixture(t)
	validatorResolver := recoveryValidatorAuthorityResolver{authority: f.authority}
	classification, err := f.node.ClassifyFinalizedCommit(
		f.ctx, f.candidate, f.certificate, f.validators, f.power, validatorResolver,
	)
	if err != nil { t.Fatal(err) }
	if classification != FinalizedCommitEvidencePresentCanonicalMissing {
		t.Fatalf("classification = %s, want %s", classification, FinalizedCommitEvidencePresentCanonicalMissing)
	}

	if err := f.node.CommitFinalizedBlock(
		f.ctx, f.candidate, f.certificate, f.validators, f.power,
		validatorResolver, senderAuthorityResolver{publicKey: f.signerA.PublicKey()},
	); err != nil { t.Fatal(err) }

	classification, err = f.node.ClassifyFinalizedCommit(
		f.ctx, f.candidate, f.certificate, f.validators, f.power, validatorResolver,
	)
	if err != nil { t.Fatal(err) }
	if classification != FinalizedCommitCanonicalMatched {
		t.Fatalf("post-commit classification = %s, want %s", classification, FinalizedCommitCanonicalMatched)
	}
}

func TestCommitFinalizedBlockRejectsInvalidEvidenceBeforeAnyMutation(t *testing.T) {
	f := buildFinalizedRecoveryFixture(t)
	originalHead := f.node.HeadHash
	originalHeight := f.node.Head.Header.Height
	invalid := f.certificate
	invalid.Votes = append([]consensus.Message(nil), f.certificate.Votes...)
	invalid.Votes[0].Signature = append([]byte(nil), invalid.Votes[0].Signature...)
	invalid.Votes[0].Signature[0] ^= 0xff

	err := f.node.CommitFinalizedBlock(
		f.ctx, f.candidate, invalid, f.validators, f.power,
		recoveryValidatorAuthorityResolver{authority: f.authority},
		senderAuthorityResolver{publicKey: f.signerA.PublicKey()},
	)
	if !errors.Is(err, consensus.ErrInvalidSignature) {
		t.Fatalf("error = %v, want invalid consensus signature", err)
	}
	if f.node.Head.Header.Height != originalHeight || f.node.HeadHash != originalHead {
		t.Fatal("invalid recovery evidence mutated canonical head")
	}
}

func TestCommitFinalizedBlockSecondRecoveryIsIdempotent(t *testing.T) {
	f := buildFinalizedRecoveryFixture(t)
	validatorResolver := recoveryValidatorAuthorityResolver{authority: f.authority}
	senderResolver := senderAuthorityResolver{publicKey: f.signerA.PublicKey()}

	if err := f.node.CommitFinalizedBlock(
		f.ctx, f.candidate, f.certificate, f.validators, f.power,
		validatorResolver, senderResolver,
	); err != nil { t.Fatal(err) }
	head := f.node.HeadHash
	height := f.node.Head.Header.Height

	err := f.node.CommitFinalizedBlock(
		f.ctx, f.candidate, f.certificate, f.validators, f.power,
		validatorResolver, senderResolver,
	)
	if !errors.Is(err, ErrFinalizedBlockAlreadyCommitted) {
		t.Fatalf("second recovery error = %v, want %v", err, ErrFinalizedBlockAlreadyCommitted)
	}
	if f.node.HeadHash != head || f.node.Head.Header.Height != height {
		t.Fatal("second recovery mutated canonical head")
	}
}
