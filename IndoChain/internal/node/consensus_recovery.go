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

	runtime, err := consensus.NewValidatorRuntime(consensus.RuntimeConfig{
		Rules: consensus.ValidationRules{
			ProtocolVersion: recovered.Config.ProtocolVersion,
			ChainID:         recovered.Config.ChainID,
			RequireSender:   true,
		},
		State: state,
		Validators: validators,
		VotingPower: votingPower,
		Threshold: threshold,
		Proposer: proposer,
	})
	if err != nil {
		return ConsensusRecovery{}, err
	}

	return ConsensusRecovery{
		Runtime:       runtime,
		State:         state,
		PreviousHash:  recovered.HeadHash,
		CanonicalHash: recovered.HeadHash,
		StateRoot:     recovered.State.Root(),
		Height:        recovered.Head.Header.Height,
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
		State: stateForNextProposal(r.State),
		PreviousHash: r.PreviousHash,
		Proposer: proposer,
	}, nil
}

func stateForNextProposal(s consensus.RoundState) consensus.RoundState {
	return s
}
