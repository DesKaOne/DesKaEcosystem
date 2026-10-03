package node

import (
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var (
	ErrConsensusRecoveryMismatch = errors.New("consensus recovery context mismatch")
)

// ConsensusRecovery is the deterministic handoff reconstructed from durable
// canonical node state. Runtime owns consensus protocol state; the node remains
// the source of canonical block/hash/state-root.
type ConsensusRecovery struct {
	Runtime       *consensus.ValidatorRuntime
	State         consensus.RoundState
	PreviousHash  types.Hash
	CanonicalHash types.Hash
	StateRoot     types.Hash
	Height        types.Height
	Evidence      []consensus.Message
}

// ReconstructConsensusRuntime rebuilds a fresh consensus runtime from the
// durable canonical store. Ephemeral proposal/vote/lock/finality evidence is
// deliberately not restored.
func (n *Node) ReconstructConsensusRuntime(
	epoch uint64,
	validators consensus.ValidatorSet,
	votingPower consensus.VotingPowerSet,
	threshold consensus.QuorumThreshold,
	proposer consensus.ProposerSelector,
) (ConsensusRecovery, error) {
	return n.reconstructConsensusRuntime(epoch, validators, votingPower, threshold, proposer, nil, nil)
}

// ReconstructConsensusRuntimeWithAuthority rebuilds a fresh consensus runtime
// from durable canonical state while binding the runtime to the supplied
// epoch-bound validator authority snapshot.
func (n *Node) ReconstructConsensusRuntimeWithAuthority(
	epoch uint64,
	validators consensus.ValidatorSet,
	votingPower consensus.VotingPowerSet,
	threshold consensus.QuorumThreshold,
	proposer consensus.ProposerSelector,
	authority consensus.ValidatorAuthoritySet,
) (ConsensusRecovery, error) {
	return n.reconstructConsensusRuntime(epoch, validators, votingPower, threshold, proposer, &authority, nil)
}

// ReconstructConsensusRuntimeWithAuthorityAndEvidence rebuilds canonical consensus state,
// binds the recovered runtime to the supplied validator authority snapshot, and separately
// recovers authenticated evidence using the same immutable authority snapshot.
func (n *Node) ReconstructConsensusRuntimeWithAuthorityAndEvidence(
	epoch uint64,
	validators consensus.ValidatorSet,
	votingPower consensus.VotingPowerSet,
	threshold consensus.QuorumThreshold,
	proposer consensus.ProposerSelector,
	evidenceStore consensus.EvidenceStore,
	authority consensus.ValidatorAuthoritySet,
) (ConsensusRecovery, error) {
	if evidenceStore == nil {
		return ConsensusRecovery{}, consensus.ErrNilEvidenceStore
	}
	resolver, err := authority.ConsensusAuthorityResolver()
	if err != nil {
		return ConsensusRecovery{}, err
	}
	return n.reconstructConsensusRuntime(epoch, validators, votingPower, threshold, proposer, &authority, func(state consensus.RoundState) ([]consensus.Message, error) {
		return consensus.RecoverAuthenticatedEvidence(evidenceStore, state, validators, resolver)
	})
}

// ReconstructConsensusRuntimeWithEvidence rebuilds the runtime from durable
// canonical state and separately recovers authenticated evidence for the same
// canonical height. Evidence is returned as validated data; this boundary does
// not replay it into ValidatorRuntime automatically.
func (n *Node) ReconstructConsensusRuntimeWithEvidence(
	epoch uint64,
	validators consensus.ValidatorSet,
	votingPower consensus.VotingPowerSet,
	threshold consensus.QuorumThreshold,
	proposer consensus.ProposerSelector,
	evidenceStore consensus.EvidenceStore,
	authority consensus.TimeoutAuthorityResolver,
) (ConsensusRecovery, error) {
	if evidenceStore == nil {
		return ConsensusRecovery{}, consensus.ErrNilEvidenceStore
	}
	return n.reconstructConsensusRuntime(epoch, validators, votingPower, threshold, proposer, nil, func(state consensus.RoundState) ([]consensus.Message, error) {
		return consensus.RecoverAuthenticatedEvidence(evidenceStore, state, validators, authority)
	})
}

// ReconstructConsensusRuntimeWithAuthorityAndContextReplay rebuilds the runtime
// from canonical node state, derives the persistence context from that runtime,
// recovers evidence bound to the same context, and replays it without canonical
// chain/state mutation.
func (n *Node) ReconstructConsensusRuntimeWithAuthorityAndContextReplay(
	epoch uint64,
	validators consensus.ValidatorSet,
	votingPower consensus.VotingPowerSet,
	threshold consensus.QuorumThreshold,
	proposer consensus.ProposerSelector,
	evidenceStore consensus.EvidenceStore,
	authority consensus.ValidatorAuthoritySet,
	votingPowerDigest [32]byte,
	proposerPolicy, proposerPolicyVersion string,
) (ConsensusRecovery, consensus.PersistenceContext, error) {
	if evidenceStore == nil {
		return ConsensusRecovery{}, consensus.PersistenceContext{}, consensus.ErrNilEvidenceStore
	}
	recovery, err := n.ReconstructConsensusRuntimeWithAuthority(
		epoch, validators, votingPower, threshold, proposer, authority,
	)
	if err != nil {
		return ConsensusRecovery{}, consensus.PersistenceContext{}, err
	}
	context, err := recovery.Runtime.PersistenceContext(
		votingPowerDigest, votingPowerDigest, proposerPolicy, proposerPolicyVersion,
	)
	if err != nil {
		return ConsensusRecovery{}, consensus.PersistenceContext{}, err
	}
	resolver, err := recovery.Runtime.ConsensusAuthority()
	if err != nil {
		return ConsensusRecovery{}, consensus.PersistenceContext{}, err
	}
	evidence, err := consensus.RecoverAuthenticatedEvidenceWithContext(
		evidenceStore, recovery.State, validators, resolver, context,
	)
	if err != nil {
		return ConsensusRecovery{}, consensus.PersistenceContext{}, err
	}
	if err := recovery.Runtime.ReplayRecoveredEvidence(evidence); err != nil {
		return ConsensusRecovery{}, consensus.PersistenceContext{}, err
	}
	recovery.Evidence = evidence
	return recovery, context, nil
}

// ReconstructConsensusRuntimeAndReplayEvidence rebuilds canonical consensus
// state and then applies only the deterministic operational subset of
// authenticated evidence. Failed replay returns no partially-mutated runtime.
func (n *Node) ReconstructConsensusRuntimeAndReplayEvidence(
	epoch uint64,
	validators consensus.ValidatorSet,
	votingPower consensus.VotingPowerSet,
	threshold consensus.QuorumThreshold,
	proposer consensus.ProposerSelector,
	evidenceStore consensus.EvidenceStore,
	authority consensus.TimeoutAuthorityResolver,
) (ConsensusRecovery, consensus.EvidenceReplayResult, error) {
	recovery, err := n.ReconstructConsensusRuntimeWithEvidence(
		epoch, validators, votingPower, threshold, proposer, evidenceStore, authority,
	)
	if err != nil {
		return ConsensusRecovery{}, consensus.EvidenceReplayResult{}, err
	}
	result, err := consensus.ReplayAuthenticatedEvidence(recovery.Runtime, recovery.Evidence, authority)
	if err != nil {
		return ConsensusRecovery{}, consensus.EvidenceReplayResult{}, err
	}
	return recovery, result, nil
}

func (n *Node) reconstructConsensusRuntime(
	epoch uint64,
	validators consensus.ValidatorSet,
	votingPower consensus.VotingPowerSet,
	threshold consensus.QuorumThreshold,
	proposer consensus.ProposerSelector,
	authority *consensus.ValidatorAuthoritySet,
	recoverEvidence func(consensus.RoundState) ([]consensus.Message, error),
) (ConsensusRecovery, error) {
	if n == nil || n.Store == nil {
		return ConsensusRecovery{}, ErrNilStore
	}

	recovered, err := OpenDevnet(n.Store)
	if err != nil {
		return ConsensusRecovery{}, err
	}
	if recovered.Config.ProtocolVersion != n.Config.ProtocolVersion ||
		recovered.Config.ChainID != n.Config.ChainID {
		return ConsensusRecovery{}, ErrConsensusRecoveryMismatch
	}

	state, err := consensus.NewRoundState(
		recovered.Config.ProtocolVersion,
		recovered.Config.ChainID,
		epoch,
		recovered.Head.Header.Height,
	)
	if err != nil {
		return ConsensusRecovery{}, err
	}

	runtimeConfig := consensus.RuntimeConfig{
		Rules: consensus.ValidationRules{
			ProtocolVersion: recovered.Config.ProtocolVersion,
			ChainID:         recovered.Config.ChainID,
			RequireSender:   true,
		},
		State:       state,
		Validators:  validators,
		VotingPower: votingPower,
		Threshold:   threshold,
		Proposer:    proposer,
		Authority:   authority,
	}
	runtime, err := consensus.NewValidatorRuntime(runtimeConfig)
	if err != nil {
		return ConsensusRecovery{}, err
	}

	var evidence []consensus.Message
	if recoverEvidence != nil {
		evidence, err = recoverEvidence(state)
		if err != nil {
			return ConsensusRecovery{}, err
		}
	}

	return ConsensusRecovery{
		Runtime:       runtime,
		State:         state,
		PreviousHash:  recovered.HeadHash,
		CanonicalHash: recovered.HeadHash,
		StateRoot:     recovered.State.Root(),
		Height:        recovered.Head.Header.Height,
		Evidence:      evidence,
	}, nil
}

// NextBlockContext derives the exact context a recovered runtime must use for
// the next block. It reads the canonical previous hash from the recovery
// snapshot, never from ephemeral consensus evidence.
func (r ConsensusRecovery) NextBlockContext() (consensus.BlockProductionContext, error) {
	if r.Runtime == nil {
		return consensus.BlockProductionContext{}, ErrConsensusRecoveryMismatch
	}
	if err := r.State.Validate(); err != nil {
		return consensus.BlockProductionContext{}, err
	}
	if r.Height != r.State.Height || r.CanonicalHash != r.PreviousHash || r.StateRoot == (types.Hash{}) {
		return consensus.BlockProductionContext{}, ErrConsensusRecoveryMismatch
	}
	proposer, err := r.Runtime.ExpectedProposer()
	if err != nil {
		return consensus.BlockProductionContext{}, err
	}
	return consensus.BlockProductionContext{
		State:        r.State,
		PreviousHash: r.PreviousHash,
		Proposer:     proposer,
	}, nil
}
