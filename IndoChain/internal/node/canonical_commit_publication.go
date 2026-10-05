package node

import (
	"errors"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
)

// CommitFinalityEvidenceAndPublishConsensus performs the canonical commit
// handoff and then publishes the exact committed height/hash/state-root to a
// live consensus runtime. Consensus is preflighted before storage mutation;
// publication is emitted only from the node's actual post-commit canonical
// values.
func (n *Node) CommitFinalityEvidenceAndPublishConsensus(
	ctx consensus.BlockProductionContext,
	candidate block.Block,
	certificate consensus.FinalityCertificate,
	validators consensus.ValidatorSet,
	votingPower consensus.VotingPowerSet,
	validatorResolver ValidatorAuthorityResolver,
	senderResolver TransactionAuthorityResolver,
	runtime *consensus.ValidatorRuntime,
) error {
	if runtime == nil {
		return errors.New("nil consensus runtime")
	}
	candidateHash, err := block.Hash(candidate)
	if err != nil {
		return fmt.Errorf("hash candidate: %w", err)
	}
	preflight := consensus.CanonicalCommitPublication{
		Height: candidate.Header.Height,
		BlockHash: candidateHash,
		StateRoot: candidate.Header.StateRoot,
	}
	if err := consensus.ValidateCanonicalCommitPublication(runtime, preflight); err != nil {
		return err
	}

	if err := n.CommitFinalityEvidence(
		ctx, candidate, certificate, validators, votingPower,
		validatorResolver, senderResolver,
	); err != nil {
		return err
	}

	committed := consensus.CanonicalCommitPublication{
		Height: n.Head.Header.Height,
		BlockHash: n.HeadHash,
		StateRoot: n.State.Root(),
	}
	if committed != preflight {
		return consensus.ErrCanonicalCommitPublicationContextMismatch
	}
	return runtime.PublishCanonicalCommit(committed)
}
