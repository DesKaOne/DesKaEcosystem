package node

import (
	"errors"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
)

// CommitFinalityEvidenceAndPublishConsensus performs the canonical commit
// handoff and then publishes the exact committed height/hash/state-root to a
// live consensus runtime. Runtime publication is preflighted before storage
// mutation and repeated only after the node exposes the committed canonical
// values, so a failed storage commit never advances consensus.
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
	preflight, err := consensus.NewCanonicalCommit(candidate.Header.Height, candidateHash, candidate.Header.StateRoot)
	if err != nil {
		return err
	}
	if err := runtime.ValidateCanonicalCommit(preflight); err != nil {
		return err
	}

	if err := n.CommitFinalityEvidence(
		ctx, candidate, certificate, validators, votingPower,
		validatorResolver, senderResolver,
	); err != nil {
		return err
	}

	committed, err := consensus.NewCanonicalCommit(
		n.Head.Header.Height,
		n.HeadHash,
		n.State.Root(),
	)
	if err != nil {
		return err
	}
	if committed != preflight {
		return consensus.ErrCanonicalCommitContextMismatch
	}
	return runtime.PublishCanonicalCommit(committed)
}
