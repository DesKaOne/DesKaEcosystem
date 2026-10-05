package node

import (
	"errors"
	"reflect"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

func TestResumeFinalityCommitCompletesPendingCanonicalCommit(t *testing.T) {
	n, _, candidate, certificate, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	validators := mustValidatorSet(t, certificate)
	power := mustVotingPowerSet(t, certificate)
	recovery, err := n.ReconstructConsensusRuntime(1, validators, power, certificate.Threshold, consensus.RoundRobinProposer{})
	if err != nil { t.Fatal(err) }

	result, err := n.ResumeFinalityCommit(
		recovery, candidate, certificate, validators, power,
		validatorResolver, senderResolver, validatorResolver,
	)
	if err != nil { t.Fatal(err) }
	if !result.Committed || result.AlreadyCommitted {
		t.Fatalf("unexpected recovery result: %+v", result)
	}
	if n.Head.Header.Height != candidate.Header.Height || n.HeadHash != result.BlockHash {
		t.Fatal("canonical head did not resume from pending finality")
	}
	if got := recovery.Runtime.State(); got.Height != candidate.Header.Height || got.Phase != consensus.PhaseProposal {
		t.Fatalf("runtime did not publish next-height state: %+v", got)
	}
}

func TestResumeFinalityCommitIsIdempotentAfterCanonicalCommit(t *testing.T) {
	n, ctx, candidate, certificate, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	validators := mustValidatorSet(t, certificate)
	power := mustVotingPowerSet(t, certificate)
	if err := n.CommitFinalityEvidence(ctx, candidate, certificate, validators, power, validatorResolver, senderResolver); err != nil {
		t.Fatal(err)
	}
	recovery, err := n.ReconstructConsensusRuntime(1, validators, power, certificate.Threshold, consensus.RoundRobinProposer{})
	if err != nil { t.Fatal(err) }
	beforeHead, beforeHash, beforeRoot := n.Head, n.HeadHash, n.State.Root()

	result, err := n.ResumeFinalityCommit(
		recovery, candidate, certificate, validators, power,
		validatorResolver, senderResolver, validatorResolver,
	)
	if err != nil { t.Fatal(err) }
	if !result.AlreadyCommitted || result.Committed {
		t.Fatalf("unexpected idempotent recovery result: %+v", result)
	}
	if !reflect.DeepEqual(n.Head, beforeHead) || n.HeadHash != beforeHash || n.State.Root() != beforeRoot {
		t.Fatal("idempotent finality recovery mutated canonical state")
	}
}

func TestResumeFinalityCommitRejectsMismatchedCandidateWithoutMutation(t *testing.T) {
	n, _, candidate, certificate, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	validators := mustValidatorSet(t, certificate)
	power := mustVotingPowerSet(t, certificate)
	recovery, err := n.ReconstructConsensusRuntime(1, validators, power, certificate.Threshold, consensus.RoundRobinProposer{})
	if err != nil { t.Fatal(err) }
	candidate.Header.Timestamp++
	beforeHead, beforeHash, beforeRoot := n.Head, n.HeadHash, n.State.Root()
	_, err = n.ResumeFinalityCommit(
		recovery, candidate, certificate, validators, power,
		validatorResolver, senderResolver, validatorResolver,
	)
	if !errors.Is(err, consensus.ErrCanonicalCommitPublicationContextMismatch) {
		t.Fatalf("error = %v, want candidate mismatch", err)
	}
	if !reflect.DeepEqual(n.Head, beforeHead) || n.HeadHash != beforeHash || n.State.Root() != beforeRoot {
		t.Fatal("canonical state mutated after mismatched recovery candidate")
	}
}

func TestResumeFinalityCommitStoreFailurePreservesRecoveredFinality(t *testing.T) {
	store := &failingCommitStore{MemoryStore: storage.NewMemoryStore()}
	n, _, candidate, certificate, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, store)
	validators := mustValidatorSet(t, certificate)
	power := mustVotingPowerSet(t, certificate)
	recovery, err := n.ReconstructConsensusRuntime(1, validators, power, certificate.Threshold, consensus.RoundRobinProposer{})
	if err != nil { t.Fatal(err) }
	beforeHead, beforeHash, beforeRoot := n.Head, n.HeadHash, n.State.Root()
	store.failCommit = true

	_, err = n.ResumeFinalityCommit(
		recovery, candidate, certificate, validators, power,
		validatorResolver, senderResolver, validatorResolver,
	)
	if !errors.Is(err, errCommitFailed) {
		t.Fatalf("error = %v, want %v", err, errCommitFailed)
	}
	if !reflect.DeepEqual(n.Head, beforeHead) || n.HeadHash != beforeHash || n.State.Root() != beforeRoot {
		t.Fatal("canonical state advanced after failed recovery commit")
	}
	if got := recovery.Runtime.State(); got.Phase != consensus.PhaseFinalized || got.Height != beforeHead.Header.Height {
		t.Fatalf("recovered finality was lost after failed commit: %+v", got)
	}
}
