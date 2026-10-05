package consensus

import (
	"bytes"
	"errors"
	"fmt"
)

var (
	ErrFinalityRecoveryNotReady = errors.New("finality recovery evidence not ready")
)

// RestoreFinalizedEvidence restores only the minimal finalized runtime state
// required to resume a canonical commit after restart. It is intentionally
// separate from ordinary evidence replay: callers must explicitly choose this
// crash-recovery boundary.
//
// The certificate is fully revalidated, including every authenticated
// precommit vote. No canonical storage is touched.
func (r *ValidatorRuntime) RestoreFinalizedEvidence(
	certificate FinalityCertificate,
	resolver TimeoutAuthorityResolver,
) error {
	if r == nil {
		return ErrInvalidConsensusRuntime
	}
	if resolver == nil {
		return ErrAuthenticatedConsensusAuthorityMissing
	}
	if err := ValidateFinalityCertificate(certificate, r.state, r.validators, r.votingPower); err != nil {
		return err
	}
	if len(certificate.Payload) == 0 || len(certificate.Votes) == 0 {
		return ErrFinalityRecoveryNotReady
	}
	for _, vote := range certificate.Votes {
		if vote.Type != MessageTypePrecommit {
			return fmt.Errorf("%w: finality vote type", ErrFinalityRecoveryNotReady)
		}
		if !bytes.Equal(vote.Payload, certificate.Payload) {
			return fmt.Errorf("%w: finality vote payload", ErrFinalityRecoveryNotReady)
		}
		if err := verifyValidatorMessageSignature(vote, resolver); err != nil {
			return err
		}
	}
	if r.state.Phase == PhaseFinalized {
		if r.certificate != nil && sameConsensusEvidence(r.certificate.Votes, certificate.Votes) &&
			bytes.Equal(r.certificate.Payload, certificate.Payload) {
			return nil
		}
		return ErrFinalityRecoveryNotReady
	}
	if r.state.Phase != PhaseProposal && r.state.Phase != PhasePrevote && r.state.Phase != PhasePrecommit {
		return ErrInvalidRuntimePhase
	}

	r.proposal = append([]byte(nil), certificate.Payload...)
	r.lockedProposal = append([]byte(nil), certificate.Payload...)
	r.lockedRound = certificate.Round
	proof, err := NewLockProof(certificate.Round, certificate.Payload, NewPrecommitCertificateUnchecked(certificate))
	if err != nil {
		return err
	}
	r.lockedProof = &proof
	cloned := certificate
	cloned.Payload = append([]byte(nil), certificate.Payload...)
	cloned.Votes = cloneVotes(certificate.Votes)
	r.certificate = &cloned
	r.state.Phase = PhaseFinalized
	return nil
}

// NewPrecommitCertificateUnchecked creates the recovery-local certificate
// envelope from an already validated finality certificate. It never performs
// validation itself; callers must validate the finality certificate first.
func NewPrecommitCertificateUnchecked(certificate FinalityCertificate) PrecommitCertificate {
	return PrecommitCertificate{
		ProtocolVersion: certificate.ProtocolVersion,
		ChainID: certificate.ChainID,
		Epoch: certificate.Epoch,
		Height: certificate.Height,
		Round: certificate.Round,
		Payload: append([]byte(nil), certificate.Payload...),
		Threshold: certificate.Threshold,
		Votes: cloneVotes(certificate.Votes),
	}
}
