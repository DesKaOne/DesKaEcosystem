package node

import (
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

func TestPersistCandidateForFinalityDoesNotAdvanceCanonicalHead(t *testing.T) {
	n, _, candidate, _, _, _, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	store := storage.NewMemoryCandidateStore()
	before := n.HeadHash
	key, err := n.PersistCandidateForFinality(store, candidate)
	if err != nil { t.Fatal(err) }
	if n.HeadHash != before || n.Head.Header.Height != 0 { t.Fatal("candidate persistence advanced canonical head") }
	got, err := store.GetCandidate(key)
	if err != nil { t.Fatal(err) }
	h, err := block.Hash(got)
	if err != nil { t.Fatal(err) }
	if h != key.Hash { t.Fatal("stored candidate hash mismatch") }
}

func TestResumeFinalityCommitFromCandidateStoreAfterRestart(t *testing.T) {
	n, _, candidate, certificate, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	validators := mustValidatorSet(t, certificate)
	power := mustVotingPowerSet(t, certificate)
	candidateStore := storage.NewMemoryCandidateStore()
	if _, err := n.PersistCandidateForFinality(candidateStore, candidate); err != nil { t.Fatal(err) }

	recovery, err := n.ReconstructConsensusRuntime(1, validators, power, certificate.Threshold, consensus.RoundRobinProposer{})
	if err != nil { t.Fatal(err) }
	result, err := n.ResumeFinalityCommitFromCandidateStore(recovery, candidateStore, certificate, validators, power, validatorResolver, senderResolver, validatorResolver)
	if err != nil { t.Fatal(err) }
	if !result.Committed { t.Fatalf("result=%+v", result) }
}

func TestResumeFinalityCommitFromCandidateStoreMissingCandidate(t *testing.T) {
	n, _, _, certificate, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	validators := mustValidatorSet(t, certificate)
	power := mustVotingPowerSet(t, certificate)
	recovery, err := n.ReconstructConsensusRuntime(1, validators, power, certificate.Threshold, consensus.RoundRobinProposer{})
	if err != nil { t.Fatal(err) }
	_, err = n.ResumeFinalityCommitFromCandidateStore(recovery, storage.NewMemoryCandidateStore(), certificate, validators, power, validatorResolver, senderResolver, validatorResolver)
	if !errors.Is(err, ErrFinalityRecoveryCandidateRequired) { t.Fatalf("error=%v", err) }
}

