package node

import (
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
)

// CommitFinalityEvidence is the explicit consensus-evidence → canonical-commit
// admission boundary. Evidence is validated and bound to the candidate before
// the existing atomic execution/storage commit path is entered.
//
// This method deliberately does not require a live ValidatorRuntime: a node
// may receive finality evidence from another peer after the originating
// runtime has moved on. Canonical context, validator authority, transaction
// sender authority, execution, and durable commit remain node-owned.
func (n *Node) CommitFinalityEvidence(
	ctx consensus.BlockProductionContext,
	candidate block.Block,
	certificate consensus.FinalityCertificate,
	validators consensus.ValidatorSet,
	votingPower consensus.VotingPowerSet,
	validatorResolver ValidatorAuthorityResolver,
	senderResolver TransactionAuthorityResolver,
) error {
	if n == nil || n.Store == nil || n.State == nil {
		return ErrNilStore
	}
	if validatorResolver == nil || senderResolver == nil {
		return errors.New("missing finalized-block authority resolver")
	}
	// Keep this admission boundary explicit even when the downstream commit
	// function performs the same checks. No storage mutation occurs before
	// canonical context + evidence validation has succeeded.
	if err := validateCanonicalConsensusContext(n, ctx); err != nil {
		return err
	}
	if _, err := consensus.ValidateFinalizedBlockWithAuthority(
		ctx, candidate, certificate, validators, votingPower, validatorResolver,
	); err != nil {
		return err
	}
	return n.CommitFinalizedBlock(
		ctx, candidate, certificate, validators, votingPower,
		validatorResolver, senderResolver,
	)
}
