package node

import (
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

func recoveryValidatorConfig(t *testing.T) (consensus.ValidatorSet, consensus.VotingPowerSet) {
	t.Helper()
	ids := [][]byte{[]byte("validator-a"), []byte("validator-b"), []byte("validator-c")}
	validators, err := consensus.NewValidatorSet(ids)
	if err != nil { t.Fatal(err) }
	power, err := consensus.NewVotingPowerSet([]consensus.ValidatorVotingPower{
		{ValidatorID: ids[0], Power: 4}, {ValidatorID: ids[1], Power: 3}, {ValidatorID: ids[2], Power: 3},
	})
	if err != nil { t.Fatal(err) }
	return validators, power
}

func TestReconstructConsensusRuntimeUsesDurableCanonicalState(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store); if err != nil { t.Fatal(err) }
	validators, power := recoveryValidatorConfig(t)
	recovered, err := n.ReconstructConsensusRuntime(7, validators, power, consensus.QuorumThreshold{Numerator: 2, Denominator: 3}, consensus.RoundRobinProposer{})
	if err != nil { t.Fatal(err) }
	if recovered.Height != n.Head.Header.Height || recovered.Height != 0 { t.Fatalf("recovered height = %d, want %d", recovered.Height, n.Head.Header.Height) }
	if recovered.CanonicalHash != n.HeadHash || recovered.PreviousHash != n.HeadHash { t.Fatal("recovered canonical hash does not match durable head") }
	if recovered.StateRoot != n.State.Root() || recovered.StateRoot == (types.Hash{}) { t.Fatal("recovered state root does not match durable state") }
	if got := recovered.Runtime.State(); got.Phase != consensus.PhaseProposal || got.Round != 0 || got.Height != n.Head.Header.Height { t.Fatalf("unexpected recovered runtime state: %+v", got) }
	proposer, err := recovered.Runtime.ExpectedProposer(); if err != nil { t.Fatal(err) }
	if string(proposer) != "validator-a" { t.Fatalf("proposer = %q, want validator-a", proposer) }
}

func TestReconstructConsensusRuntimeIgnoresStaleInMemoryNodeState(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store); if err != nil { t.Fatal(err) }
	originalHash, originalRoot := n.HeadHash, n.State.Root()
	n.HeadHash = types.Hash{99}
	n.State = n.State.Snapshot()
	n.State.Set(types.Address([]byte("ephemeral-corruption")), state.Account{Balance: 999})
	validators, power := recoveryValidatorConfig(t)
	recovered, err := n.ReconstructConsensusRuntime(3, validators, power, consensus.QuorumThreshold{Numerator: 2, Denominator: 3}, consensus.RoundRobinProposer{})
	if err != nil { t.Fatal(err) }
	if recovered.CanonicalHash != originalHash || recovered.StateRoot != originalRoot { t.Fatal("recovery consumed stale in-memory canonical values") }
}

func TestReconstructConsensusRuntimeRejectsCorruptDurableStore(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store); if err != nil { t.Fatal(err) }
	blockAtHead, _, err := store.Head(); if err != nil { t.Fatal(err) }
	if err := store.SaveBlock(blockAtHead, types.Hash{77}); err != nil { t.Fatal(err) }
	validators, power := recoveryValidatorConfig(t)
	_, err = n.ReconstructConsensusRuntime(1, validators, power, consensus.QuorumThreshold{Numerator: 2, Denominator: 3}, consensus.RoundRobinProposer{})
	if !errors.Is(err, ErrBlockHashMismatch) { t.Fatalf("error = %v, want %v", err, ErrBlockHashMismatch) }
}

func TestConsensusRecoveryNextBlockContextUsesCanonicalPreviousHash(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store); if err != nil { t.Fatal(err) }
	validators, power := recoveryValidatorConfig(t)
	recovered, err := n.ReconstructConsensusRuntime(5, validators, power, consensus.QuorumThreshold{Numerator: 2, Denominator: 3}, consensus.RoundRobinProposer{})
	if err != nil { t.Fatal(err) }
	ctx, err := recovered.NextBlockContext(); if err != nil { t.Fatal(err) }
	if ctx.State.Height != n.Head.Header.Height || ctx.State.Round != 0 || ctx.State.Phase != consensus.PhaseProposal { t.Fatalf("unexpected next-block state: %+v", ctx.State) }
	if ctx.PreviousHash != n.HeadHash { t.Fatal("next-block context did not use durable canonical previous hash") }
	if string(ctx.Proposer) != "validator-a" { t.Fatalf("next-block proposer = %q, want validator-a", ctx.Proposer) }
}
