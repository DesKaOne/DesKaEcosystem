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


func TestReconstructConsensusRuntimeWithAuthorityBindsRuntimeSnapshot(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store)
	if err != nil { t.Fatal(err) }
	validators, power := recoveryValidatorConfig(t)
	authority, err := consensus.NewValidatorAuthoritySet(7, validators, map[string][]byte{
		"validator-a": []byte("key-a"),
		"validator-b": []byte("key-b"),
		"validator-c": []byte("key-c"),
	})
	if err != nil { t.Fatal(err) }

	recovered, err := n.ReconstructConsensusRuntimeWithAuthority(
		7, validators, power,
		consensus.QuorumThreshold{Numerator: 2, Denominator: 3},
		consensus.RoundRobinProposer{}, authority,
	)
	if err != nil { t.Fatal(err) }

	got, err := recovered.Runtime.Authority()
	if err != nil { t.Fatal(err) }
	if got.Epoch != authority.Epoch {
		t.Fatalf("recovered authority epoch = %d, want %d", got.Epoch, authority.Epoch)
	}
	key, err := got.PublicKeyForValidator([]byte("validator-a"))
	if err != nil || string(key) != "key-a" {
		t.Fatalf("unexpected recovered validator authority: %q, %v", key, err)
	}

	ctx, err := recovered.Runtime.PersistenceContext([32]byte{7}, [32]byte{8}, "round-robin-v0-dev", "1")
	if err != nil { t.Fatal(err) }
	if ctx.Epoch != 7 || ctx.ValidatorAuthorityDigest == ([32]byte{}) {
		t.Fatalf("unexpected authority-bound persistence context: %+v", ctx)
	}
}


func TestReconstructConsensusRuntimeWithAuthorityAndEvidenceBindsSameAuthority(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store)
	if err != nil { t.Fatal(err) }
	validators, power := recoveryValidatorConfig(t)
	authority, err := consensus.NewValidatorAuthoritySet(0, validators, map[string][]byte{
		"validator-a": []byte("key-a"),
		"validator-b": []byte("key-b"),
		"validator-c": []byte("key-c"),
	})
	if err != nil { t.Fatal(err) }

	recovery, err := n.ReconstructConsensusRuntimeWithAuthorityAndEvidence(
		0, validators, power,
		consensus.QuorumThreshold{Numerator: 2, Denominator: 3},
		consensus.RoundRobinProposer{},
		storage.NewMemoryConsensusEvidenceStore(), authority,
	)
	if err != nil { t.Fatal(err) }
	gotAuthority, err := recovery.Runtime.Authority()
	if err != nil { t.Fatal(err) }
	gotResolver, err := authority.ConsensusAuthorityResolver()
	if err != nil { t.Fatal(err) }
	gotKey, err := gotAuthority.PublicKeyForValidator([]byte("validator-a"))
	if err != nil { t.Fatal(err) }
	wantKey, err := gotResolver.PublicKeyForValidator([]byte("validator-a"))
	if err != nil { t.Fatal(err) }
	if string(gotKey) != string(wantKey) {
		t.Fatalf("runtime/evidence authority key mismatch: %q != %q", gotKey, wantKey)
	}
	ctx, err := recovery.Runtime.PersistenceContext([32]byte{1}, [32]byte{2}, "round-robin-v0-dev", "1")
	if err != nil { t.Fatal(err) }
	if ctx.ValidatorAuthorityDigest == ([32]byte{}) {
		t.Fatal("authority-bound persistence context must carry a non-zero authority digest")
	}
}

func TestReconstructConsensusRuntimeWithAuthorityRejectsEpochMismatch(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store)
	if err != nil { t.Fatal(err) }
	validators, power := recoveryValidatorConfig(t)
	authority, err := consensus.NewValidatorAuthoritySet(8, validators, map[string][]byte{
		"validator-a": []byte("key-a"),
		"validator-b": []byte("key-b"),
		"validator-c": []byte("key-c"),
	})
	if err != nil { t.Fatal(err) }

	_, err = n.ReconstructConsensusRuntimeWithAuthority(
		7, validators, power,
		consensus.QuorumThreshold{Numerator: 2, Denominator: 3},
		consensus.RoundRobinProposer{}, authority,
	)
	if !errors.Is(err, consensus.ErrValidatorAuthorityMismatch) {
		t.Fatalf("error = %v, want %v", err, consensus.ErrValidatorAuthorityMismatch)
	}
}

func TestReconstructConsensusRuntimeWithAuthorityAndContextReplay(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store)
	if err != nil { t.Fatal(err) }
	validators, power := recoveryValidatorConfig(t)
	authority, err := consensus.NewValidatorAuthoritySet(0, validators, map[string][]byte{
		"validator-a": []byte("key-a"),
		"validator-b": []byte("key-b"),
		"validator-c": []byte("key-c"),
	})
	if err != nil { t.Fatal(err) }
	evidenceStore := storage.NewMemoryConsensusEvidenceStore()
	recovery, ctx, err := n.ReconstructConsensusRuntimeWithAuthorityAndContextReplay(
		0, validators, power,
		consensus.QuorumThreshold{Numerator: 2, Denominator: 3},
		consensus.RoundRobinProposer{}, evidenceStore, authority,
		[32]byte{8}, "round-robin-v0-dev", "1",
	)
	if err != nil { t.Fatal(err) }
	if recovery.Runtime == nil || ctx.ValidatorAuthorityDigest == ([32]byte{}) {
		t.Fatal("authority-bound context replay did not construct runtime/context")
	}
	if len(recovery.Evidence) != 0 {
		t.Fatalf("unexpected recovered evidence count %d", len(recovery.Evidence))
	}
	if got := recovery.Runtime.State(); got.Phase != consensus.PhaseProposal {
		t.Fatalf("runtime phase after empty replay = %v", got.Phase)
	}
}


func TestConsensusRecoveryBuildNextBlockCandidateFromCanonicalState(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store)
	if err != nil { t.Fatal(err) }
	validators, power := recoveryValidatorConfig(t)
	recovered, err := n.ReconstructConsensusRuntime(0, validators, power,
		consensus.QuorumThreshold{Numerator: 2, Denominator: 3},
		consensus.RoundRobinProposer{})
	if err != nil { t.Fatal(err) }

	rules, err := n.Config.BlockRules(nil)
	if err != nil { t.Fatal(err) }
	candidate, err := recovered.BuildNextBlockCandidateFromRecovery(
		n.State, n.Head.Header.Timestamp+1, nil, nil, rules,
	)
	if err != nil { t.Fatal(err) }
	if candidate.Header.Height != n.Head.Header.Height+1 {
		t.Fatalf("candidate height = %d, want %d", candidate.Header.Height, n.Head.Header.Height+1)
	}
	if candidate.Header.PreviousHash != n.HeadHash {
		t.Fatal("candidate previous hash did not come from canonical recovery")
	}
	if candidate.Header.StateRoot != n.State.Root() {
		t.Fatalf("candidate state root = %x, want canonical %x", candidate.Header.StateRoot, n.State.Root())
	}
}

func TestConsensusRecoveryBuildNextBlockCandidateRejectsStaleStateRoot(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store)
	if err != nil { t.Fatal(err) }
	validators, power := recoveryValidatorConfig(t)
	recovered, err := n.ReconstructConsensusRuntime(0, validators, power,
		consensus.QuorumThreshold{Numerator: 2, Denominator: 3},
		consensus.RoundRobinProposer{})
	if err != nil { t.Fatal(err) }
	stale := n.State.Snapshot()
	stale.Set(types.Address([]byte("stale")), state.Account{Balance: 1})
	rules, err := n.Config.BlockRules(nil)
	if err != nil { t.Fatal(err) }
	if _, err := recovered.BuildNextBlockCandidateFromRecovery(
		stale, n.Head.Header.Timestamp+1, nil, nil, rules,
	); !errors.Is(err, ErrConsensusRecoveryMismatch) {
		t.Fatalf("error = %v, want %v", err, ErrConsensusRecoveryMismatch)
	}
}

func TestConsensusRecoveryBuildNextBlockCandidateRejectsStalePreviousHash(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store)
	if err != nil { t.Fatal(err) }
	validators, power := recoveryValidatorConfig(t)
	recovered, err := n.ReconstructConsensusRuntime(0, validators, power,
		consensus.QuorumThreshold{Numerator: 2, Denominator: 3},
		consensus.RoundRobinProposer{})
	if err != nil { t.Fatal(err) }
	recovered.PreviousHash = types.Hash{99}
	rules, err := n.Config.BlockRules(nil)
	if err != nil { t.Fatal(err) }
	if _, err := recovered.BuildNextBlockCandidateFromRecovery(
		n.State, n.Head.Header.Timestamp+1, nil, nil, rules,
	); !errors.Is(err, ErrConsensusRecoveryMismatch) {
		t.Fatalf("error = %v, want %v", err, ErrConsensusRecoveryMismatch)
	}
}


func TestConsensusRecoveryBuildNextBlockProposalFromCanonicalState(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store)
	if err != nil {
		t.Fatal(err)
	}
	validators, power := recoveryValidatorConfig(t)
	recovered, err := n.ReconstructConsensusRuntime(
		0, validators, power,
		consensus.QuorumThreshold{Numerator: 2, Denominator: 3},
		consensus.RoundRobinProposer{},
	)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := n.Config.BlockRules(nil)
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := recovered.BuildNextBlockProposalFromRecovery(
		n.State, n.Head.Header.Timestamp+1, nil, nil, rules,
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := recovered.NextBlockContext()
	if err != nil {
		t.Fatal(err)
	}
	payload, err := consensus.ValidateProducedBlock(ctx, proposal.Candidate)
	if err != nil {
		t.Fatal(err)
	}
	if !proposal.SamePayload(payload[:]) {
		t.Fatal("proposal payload does not identify recovered candidate")
	}
	if proposal.Candidate.Header.Height != n.Head.Header.Height+1 {
		t.Fatalf("candidate height = %d, want %d", proposal.Candidate.Header.Height, n.Head.Header.Height+1)
	}
	if proposal.Candidate.Header.PreviousHash != n.HeadHash {
		t.Fatal("proposal candidate previous hash did not come from canonical recovery")
	}
	if got := recovered.Runtime.State(); got.Phase != consensus.PhaseProposal {
		t.Fatalf("runtime phase = %v, want proposal before handoff", got.Phase)
	}
}


func TestConsensusRecoveryProposalToRuntimeFinalityHandoff(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store)
	if err != nil { t.Fatal(err) }

	validators, power := recoveryValidatorConfig(t)
	authority, err := consensus.NewValidatorAuthoritySet(0, validators, map[string][]byte{
		"validator-a": []byte("key-a"),
		"validator-b": []byte("key-b"),
		"validator-c": []byte("key-c"),
	})
	if err != nil { t.Fatal(err) }
	authorityResolver, err := authority.ConsensusAuthorityResolver()
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
	if err := recovery.Runtime.AcceptBlockProposal(proposal); err != nil {
		t.Fatal(err)
	}

	proposer := proposal.Candidate.Header.Proposer
	signer := mustTestSigner(t, 23)
	prevote := consensus.Message{
		ProtocolVersion: recovery.State.ProtocolVersion,
		ChainID:         recovery.State.ChainID,
		Epoch:           recovery.State.Epoch,
		Height:          recovery.State.Height,
		Round:           recovery.State.Round,
		Sender:          proposer,
		Type:            consensus.MessageTypePrevote,
		Payload:         proposal.MessagePayload(),
	}
	prevote, err := prevote.Sign(signer)
	if err != nil { t.Fatal(err) }
	if err := recovery.Runtime.AddVote(prevote); err != nil {
		t.Fatal(err)
	}
	if recovery.Runtime.State().Phase != consensus.PhasePrecommit {
		t.Fatalf("runtime phase = %v, want precommit after prevote quorum", recovery.Runtime.State().Phase)
	}
	precommit := consensus.Message{
		ProtocolVersion: recovery.State.ProtocolVersion,
		ChainID:         recovery.State.ChainID,
		Epoch:           recovery.State.Epoch,
		Height:          recovery.State.Height,
		Round:           recovery.State.Round,
		Sender:          proposer,
		Type:            consensus.MessageTypePrecommit,
		Payload:         proposal.MessagePayload(),
	}
	precommit, err = precommit.Sign(signer)
	if err != nil { t.Fatal(err) }
	if err := recovery.Runtime.AddVote(precommit); err != nil {
		t.Fatal(err)
	}
	certificate, err := recovery.Runtime.FinalizeProposal(authorityResolver)
	if err != nil { t.Fatal(err) }

	ctx, err := recovery.NextBlockContext()
	if err != nil { t.Fatal(err) }
	beforeHeight := n.Head.Header.Height
	if err := n.CommitFinalizedBlock(
		ctx,
		proposal.Candidate,
		certificate,
		validators,
		power,
		validatorAuthorityResolver{publicKey: []byte("key-a")},
		senderAuthorityResolver{publicKey: []byte("key-a")},
	); err != nil {
		t.Fatal(err)
	}
	if n.Head.Header.Height != beforeHeight+1 {
		t.Fatalf("head height = %d, want %d", n.Head.Header.Height, beforeHeight+1)
	}
	if n.State.Root() != proposal.Candidate.Header.StateRoot {
		t.Fatalf("committed state root = %x, want candidate %x", n.State.Root(), proposal.Candidate.Header.StateRoot)
	}
	if n.HeadHash == (types.Hash{}) {
		t.Fatal("canonical head hash is empty after finalized handoff")
	}
}
