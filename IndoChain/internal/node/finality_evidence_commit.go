package node

import (
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
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


func (n *Node) PersistFinalizedCandidateAndEvidence(
	candidateStore storage.CandidateStore,
	evidenceStore consensus.EvidenceStore,
	ctx consensus.BlockProductionContext,
	candidate block.Block,
	certificate consensus.FinalityCertificate,
	validators consensus.ValidatorSet,
	votingPower consensus.VotingPowerSet,
	validatorResolver ValidatorAuthorityResolver,
	senderResolver TransactionAuthorityResolver,
	persistenceContext consensus.PersistenceContext,
	signer crypto.Signer,
	evidenceSender []byte,
) (storage.CandidateKey, string, error) {
	if n == nil || n.Store == nil || n.State == nil {
		return storage.CandidateKey{}, "", ErrNilStore
	}
	if candidateStore == nil {
		return storage.CandidateKey{}, "", ErrNilCandidateStore
	}
	if evidenceStore == nil {
		return storage.CandidateKey{}, "", consensus.ErrNilEvidenceStore
	}
	if validatorResolver == nil || senderResolver == nil || signer == nil || len(evidenceSender) == 0 {
		return storage.CandidateKey{}, "", errors.New("missing finalized-evidence persistence authority")
	}
	if err := validateCanonicalConsensusContext(n, ctx); err != nil {
		return storage.CandidateKey{}, "", err
	}
	if _, err := consensus.ValidateFinalizedBlockWithAuthority(
		ctx, candidate, certificate, validators, votingPower, validatorResolver,
	); err != nil {
		return storage.CandidateKey{}, "", err
	}
	if persistenceContext.Phase != uint8(consensus.PhaseFinalized) {
		return storage.CandidateKey{}, "", consensus.ErrEvidencePersistenceContextMismatch
	}

	key, err := n.PersistCandidateForFinality(candidateStore, candidate)
	if err != nil {
		return storage.CandidateKey{}, "", err
	}

	evidenceKey, err := consensus.PersistFinalityCertificateWithContext(
		evidenceStore,
		certificate,
		ctx.State,
		validators,
		votingPower,
		validatorResolver,
		persistenceContext,
		signer,
		evidenceSender,
	)
	if err != nil {
		// The candidate intentionally remains durable. Evidence failure must
		// never trigger candidate deletion: retry/recovery can safely resume
		// from the already-persisted full block candidate.
		return key, "", err
	}
	return key, evidenceKey, nil
}
