package node

import (
	"errors"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

var (
	ErrNilCandidateStore = errors.New("nil candidate store")
	ErrCandidateNotPending = errors.New("candidate is not the next pending block")
)

// PersistCandidateForFinality stores a non-canonical next-block candidate as a
// crash-recovery artifact. It deliberately uses a separate CandidateStore so
// saving a pending candidate can never advance the canonical chain head.
func (n *Node) PersistCandidateForFinality(store storage.CandidateStore, candidate block.Block) (storage.CandidateKey, error) {
	if n == nil || n.State == nil || n.Store == nil { return storage.CandidateKey{}, ErrNilStore }
	if store == nil { return storage.CandidateKey{}, ErrNilCandidateStore }
	if candidate.Header.Height != n.Head.Header.Height+1 || candidate.Header.PreviousHash != n.HeadHash {
		return storage.CandidateKey{}, ErrCandidateNotPending
	}
	hash, err := block.Hash(candidate)
	if err != nil { return storage.CandidateKey{}, fmt.Errorf("hash candidate: %w", err) }
	key := storage.CandidateKey{Height:candidate.Header.Height, Hash:hash}
	if err := store.SaveCandidate(key, candidate); err != nil { return storage.CandidateKey{}, err }
	return key, nil
}

// ResumeFinalityCommitFromCandidateStore resolves the pending candidate
// directly from durable candidate storage using the finality certificate's
// exact height/hash identity, then delegates to the existing explicit
// ResumeFinalityCommit boundary.
func (n *Node) ResumeFinalityCommitFromCandidateStore(
	recovery ConsensusRecovery,
	store storage.CandidateStore,
	certificate consensus.FinalityCertificate,
	validators consensus.ValidatorSet,
	votingPower consensus.VotingPowerSet,
	validatorResolver ValidatorAuthorityResolver,
	senderResolver TransactionAuthorityResolver,
	authority consensus.TimeoutAuthorityResolver,
) (FinalityRecoveryResult, error) {
	if store == nil { return FinalityRecoveryResult{}, ErrNilCandidateStore }
	if len(certificate.Payload) != 32 { return FinalityRecoveryResult{}, consensus.ErrInvalidFinalityEvidence }
	var hash types.Hash
	copy(hash[:], certificate.Payload)
	key := storage.CandidateKey{Height:certificate.Height, Hash:hash}
	candidate, err := store.GetCandidate(key)
	if err != nil { return FinalityRecoveryResult{}, fmt.Errorf("%w: %v", ErrFinalityRecoveryCandidateRequired, err) }
	candidateHash, err := block.Hash(candidate)
	if err != nil { return FinalityRecoveryResult{}, err }
	if candidate.Header.Height != certificate.Height || candidateHash != hash {
		return FinalityRecoveryResult{}, consensus.ErrCanonicalCommitPublicationContextMismatch
	}
	return n.ResumeFinalityCommit(recovery, candidate, certificate, validators, votingPower, validatorResolver, senderResolver, authority)
}
