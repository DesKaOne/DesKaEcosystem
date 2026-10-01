package node

import (
	"errors"
	"reflect"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

func TestCommitFinalityEvidenceCommitsCanonicalBlock(t *testing.T) {
	n, ctx, candidate, certificate, validatorResolver, senderResolver, recipient := finalizedBlockFixture(t, storage.NewMemoryStore())
	if err := n.CommitFinalityEvidence(
		ctx, candidate, certificate,
		mustValidatorSet(t, certificate), mustVotingPowerSet(t, certificate),
		validatorResolver, senderResolver,
	); err != nil {
		t.Fatal(err)
	}
	if n.Head.Header.Height != 1 {
		t.Fatalf("head height = %d, want 1", n.Head.Header.Height)
	}
	if got, ok := n.State.Get(recipient); !ok || got.Balance != 20 {
		t.Fatalf("recipient = %+v, want balance 20", got)
	}
}

func TestCommitFinalityEvidenceRejectsTamperedEvidenceWithoutMutation(t *testing.T) {
	n, ctx, candidate, certificate, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	certificate.Votes[0].Signature[0] ^= 0xff
	beforeHead, beforeHash, beforeRoot := n.Head, n.HeadHash, n.State.Root()
	_, storedHash, err := n.Store.Head()
	if err != nil {
		t.Fatal(err)
	}
	if err := n.CommitFinalityEvidence(
		ctx, candidate, certificate,
		mustValidatorSet(t, certificate), mustVotingPowerSet(t, certificate),
		validatorResolver, senderResolver,
	); err == nil {
		t.Fatal("expected tampered evidence rejection")
	}
	if !reflect.DeepEqual(n.Head, beforeHead) || n.HeadHash != beforeHash || n.State.Root() != beforeRoot {
		t.Fatal("node mutated after tampered evidence rejection")
	}
	_, afterHash, err := n.Store.Head()
	if err != nil {
		t.Fatal(err)
	}
	if afterHash != storedHash {
		t.Fatal("storage mutated after tampered evidence rejection")
	}
}

func TestCommitFinalityEvidenceRejectsStaleCanonicalContextWithoutMutation(t *testing.T) {
	n, ctx, candidate, certificate, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	stale := ctx
	stale.State.Height++
	stale.PreviousHash = candidate.Header.PreviousHash
	beforeHead, beforeHash, beforeRoot := n.Head, n.HeadHash, n.State.Root()
	err := n.CommitFinalityEvidence(
		stale, candidate, certificate,
		mustValidatorSet(t, certificate), mustVotingPowerSet(t, certificate),
		validatorResolver, senderResolver,
	)
	if !errors.Is(err, ErrConsensusContextMismatch) {
		t.Fatalf("error = %v, want %v", err, ErrConsensusContextMismatch)
	}
	if !reflect.DeepEqual(n.Head, beforeHead) || n.HeadHash != beforeHash || n.State.Root() != beforeRoot {
		t.Fatal("node mutated after stale context rejection")
	}
}

func TestCommitFinalityEvidenceReplayIsIdempotentlyRejected(t *testing.T) {
	n, ctx, candidate, certificate, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	validators := mustValidatorSet(t, certificate)
	power := mustVotingPowerSet(t, certificate)
	if err := n.CommitFinalityEvidence(ctx, candidate, certificate, validators, power, validatorResolver, senderResolver); err != nil {
		t.Fatal(err)
	}
	beforeHead, beforeHash, beforeRoot := n.Head, n.HeadHash, n.State.Root()
	err := n.CommitFinalityEvidence(ctx, candidate, certificate, validators, power, validatorResolver, senderResolver)
	if err != ErrConsensusContextMismatch && err != ErrFinalizedBlockAlreadyCommitted {
		t.Fatalf("error = %v, want replay rejection", err)
	}
	if !reflect.DeepEqual(n.Head, beforeHead) || n.HeadHash != beforeHash || n.State.Root() != beforeRoot {
		t.Fatal("node mutated after replay rejection")
	}
}
