package consensus

import "errors"

var (
	ErrConsensusAuthorityMissing = errors.New("consensus authority resolver missing")
)

// ValidatePrecommitCertificateWithAuthority validates precommit evidence and
// authenticates every validator signature before the evidence can be treated
// as a cryptographic proof-of-lock.
func ValidatePrecommitCertificateWithAuthority(
	certificate PrecommitCertificate,
	state RoundState,
	validators ValidatorSet,
	votingPower VotingPowerSet,
	resolver TimeoutAuthorityResolver,
) error {
	if resolver == nil {
		return ErrConsensusAuthorityMissing
	}
	if err := ValidatePrecommitCertificate(certificate, state, validators, votingPower); err != nil {
		return err
	}
	for i, vote := range certificate.Votes {
		if vote.Type != MessageTypePrecommit {
			return ErrInvalidPrecommitCertificate
		}
		if err := verifyValidatorMessageSignature(vote, resolver); err != nil {
			return errWithEvidenceIndex("precommit", i, err)
		}
	}
	return nil
}

// ValidateLockProofWithAuthority validates the lock proof and authenticates
// every precommit signature carried by its nested certificate.
func ValidateLockProofWithAuthority(
	proof LockProof,
	state RoundState,
	validators ValidatorSet,
	votingPower VotingPowerSet,
	resolver TimeoutAuthorityResolver,
) error {
	if resolver == nil {
		return ErrConsensusAuthorityMissing
	}
	if proof.LockedRound > state.Round {
		return ErrInvalidTimeoutRound
	}
	lockState := state
	lockState.Round = proof.LockedRound
	if err := ValidateLockProof(proof, lockState, validators, votingPower); err != nil {
		return err
	}
	return ValidatePrecommitCertificateWithAuthority(
		proof.Certificate,
		lockState,
		validators,
		votingPower,
		resolver,
	)
}

// ValidateFinalityCertificateWithAuthority validates finality evidence as
// explicit precommit evidence and authenticates every validator signature.
// The legacy structural validator remains available as a development
// compatibility boundary; this function is the authenticated path.
func ValidateFinalityCertificateWithAuthority(
	certificate FinalityCertificate,
	state RoundState,
	validators ValidatorSet,
	votingPower VotingPowerSet,
	resolver TimeoutAuthorityResolver,
) error {
	if resolver == nil {
		return ErrConsensusAuthorityMissing
	}
	if err := ValidateFinalityCertificate(certificate, state, validators, votingPower); err != nil {
		return err
	}
	for i, vote := range certificate.Votes {
		if vote.Type != MessageTypePrecommit {
			return ErrInvalidFinalityCertificate
		}
		if err := verifyValidatorMessageSignature(vote, resolver); err != nil {
			return errWithEvidenceIndex("finality", i, err)
		}
	}
	return nil
}

func verifyValidatorMessageSignature(
	message Message,
	resolver TimeoutAuthorityResolver,
) error {
	sender := append([]byte(nil), message.Sender...)
	publicKey, err := resolver.PublicKeyForValidator(sender)
	if err != nil {
		return err
	}
	if len(publicKey) == 0 {
		return ErrInvalidSignature
	}
	publicKey = append([]byte(nil), publicKey...)
	return VerifyMessageSignature(message, publicKey)
}

func errWithEvidenceIndex(kind string, index int, err error) error {
	return errors.New(kind + " evidence signature validation failed at index " + itoa(index) + ": " + err.Error())
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
