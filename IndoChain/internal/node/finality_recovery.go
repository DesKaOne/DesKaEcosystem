package node

import (
	"errors"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var (
	ErrFinalityRecoveryCandidateRequired = errors.New("finality recovery candidate required")
	ErrFinalityRecoveryFutureHeight       = errors.New("finality recovery candidate is from a future height")
	ErrFinalityRecoveryStaleHeight        = errors.New("finality recovery candidate is stale")
)

// FinalityRecoveryResult describes the explicit crash-recovery outcome.
// AlreadyCommitted means the canonical store already contains the exact
// finalized block and no mutation was attempted. Committed means this call
// completed the pending canonical commit and consensus publication.
type FinalityRecoveryResult struct {
	Committed       bool
	AlreadyCommitted bool
	Height          types.Height
	BlockHash       types.Hash
}

// ResumeFinalityCommit is the explicit crash boundary for durable finality
// evidence that may have survived a process restart before canonical commit.
//
// The method does not infer a candidate block from the evidence store. The
// candidate must be supplied by durable block storage or P2P recovery and is
// validated against the recovered canonical context and finality certificate
// before any canonical mutation occurs.
//
// The runtime is restored to Finalized only after certificate validation. The
// canonical store is then committed through the existing node-owned path, and
// consensus is advanced only after the durable commit succeeds.
func (n *Node) ResumeFinalityCommit(
	recovery ConsensusRecovery,
	candidate block.Block,
	certificate consensus.FinalityCertificate,
	validators consensus.ValidatorSet,
	votingPower consensus.VotingPowerSet,
	validatorResolver ValidatorAuthorityResolver,
	senderResolver TransactionAuthorityResolver,
	authority consensus.TimeoutAuthorityResolver,
) (FinalityRecoveryResult, error) {
	if n == nil || n.Store == nil || n.State == nil {
		return FinalityRecoveryResult{}, ErrNilStore
	}
	if recovery.Runtime == nil {
		return FinalityRecoveryResult{}, consensus.ErrInvalidConsensusRuntime
	}
	if validatorResolver == nil || senderResolver == nil || authority == nil {
		return FinalityRecoveryResult{}, errors.New("missing finality recovery authority")
	}

	ctx, err := recovery.NextBlockContext()
	if err != nil {
		return FinalityRecoveryResult{}, err
	}

	candidateHash, err := block.Hash(candidate)
	if err != nil {
		return FinalityRecoveryResult{}, fmt.Errorf("hash recovery candidate: %w", err)
	}
	result := FinalityRecoveryResult{
		Height: candidate.Header.Height,
		BlockHash: candidateHash,
	}

	// If canonical storage already contains the exact certificate payload,
	// recovery is idempotent: never execute the block twice.
	if candidate.Header.Height <= n.Head.Header.Height {
		if candidate.Header.Height == n.Head.Header.Height && candidateHash == n.HeadHash {
			if candidateHash != finalityPayloadHash(certificate) {
				return FinalityRecoveryResult{}, consensus.ErrCanonicalCommitPublicationContextMismatch
			}
			return FinalityRecoveryResult{AlreadyCommitted: true, Height: candidate.Header.Height, BlockHash: candidateHash}, nil
		}
		return FinalityRecoveryResult{}, ErrFinalityRecoveryStaleHeight
	}
	if candidate.Header.Height != n.Head.Header.Height+1 {
		return FinalityRecoveryResult{}, ErrFinalityRecoveryFutureHeight
	}

	if err := validateRecoveryCertificate(recovery.State, certificate, validators, votingPower, authority); err != nil {
		return FinalityRecoveryResult{}, err
	}
	if candidateHash != finalityPayloadHash(certificate) {
		return FinalityRecoveryResult{}, consensus.ErrCanonicalCommitPublicationContextMismatch
	}

	if err := consensus.ValidateFinalizedBlockWithAuthority(
		ctx, candidate, certificate, validators, votingPower, validatorResolver,
	); err != nil {
		return FinalityRecoveryResult{}, err
	}

	// Restore only enough runtime state to make the existing publication
	// boundary valid. This is not ordinary evidence replay.
	if err := recovery.Runtime.RestoreFinalizedEvidence(certificate, authority); err != nil {
		return FinalityRecoveryResult{}, err
	}
	publication := consensus.CanonicalCommitPublication{
		Height: candidate.Header.Height,
		BlockHash: candidateHash,
		StateRoot: candidate.Header.StateRoot,
	}
	if err := consensus.ValidateCanonicalCommitPublication(recovery.Runtime, publication); err != nil {
		return FinalityRecoveryResult{}, err
	}

	if err := n.CommitFinalityEvidence(
		ctx, candidate, certificate, validators, votingPower,
		validatorResolver, senderResolver,
	); err != nil {
		return FinalityRecoveryResult{}, err
	}

	if n.Head.Header.Height != candidate.Header.Height || n.HeadHash != candidateHash {
		return FinalityRecoveryResult{}, consensus.ErrCanonicalCommitPublicationContextMismatch
	}
	if err := recovery.Runtime.PublishCanonicalCommit(publication); err != nil {
		return FinalityRecoveryResult{}, err
	}

	result.Committed = true
	return result, nil
}

func validateRecoveryCertificate(
	state consensus.RoundState,
	certificate consensus.FinalityCertificate,
	validators consensus.ValidatorSet,
	votingPower consensus.VotingPowerSet,
	authority consensus.TimeoutAuthorityResolver,
) error {
	if err := consensus.ValidateFinalityCertificate(certificate, state, validators, votingPower); err != nil {
		return err
	}
	for _, vote := range certificate.Votes {
		if vote.Type != consensus.MessageTypePrecommit {
			return consensus.ErrInvalidFinalityEvidence
		}
		if err := verifyRecoveryVoteSignature(vote, authority); err != nil {
			return err
		}
	}
	return nil
}

func verifyRecoveryVoteSignature(vote consensus.Message, authority consensus.TimeoutAuthorityResolver) error {
	key, err := authority.PublicKeyForValidator(vote.Sender)
	if err != nil {
		return err
	}
	return consensus.VerifyMessageSignature(vote, key)
}

func finalityPayloadHash(certificate consensus.FinalityCertificate) types.Hash {
	var hash types.Hash
	copy(hash[:], certificate.Payload)
	return hash
}
