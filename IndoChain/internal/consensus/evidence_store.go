package consensus

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
)

var (
	ErrNilEvidenceStore          = errors.New("nil consensus evidence store")
	ErrInvalidEvidenceKey        = errors.New("invalid consensus evidence key")
	ErrUnsupportedEvidenceType   = errors.New("unsupported durable consensus evidence type")
	ErrEvidenceContextMismatch   = errors.New("durable consensus evidence context mismatch")
	ErrConflictingEvidence       = errors.New("conflicting consensus evidence")
)

// EvidenceStore is the persistence boundary for authenticated consensus
// evidence. It is intentionally separate from ChainStore so ephemeral
// consensus records never become canonical block/state storage.
type EvidenceStore interface {
	PutConsensusEvidence(key string, encoded []byte) error
	LoadConsensusEvidence() (map[string][]byte, error)
	DeleteConsensusEvidence(key string) error
}

// DeleteConsensusEvidence removes one exact evidence identity. Implementations
// must treat an already-absent identity as a successful no-op so post-commit
// cleanup remains idempotent across crash/restart retries.
func DeleteConsensusEvidence(store EvidenceStore, key string) error {
	if store == nil {
		return ErrNilEvidenceStore
	}
	if key == "" {
		return ErrInvalidEvidenceKey
	}
	return store.DeleteConsensusEvidence(key)
}

// ConsensusEvidenceKey identifies one validator's message in one exact
// consensus context/type. Payload and signature are excluded so conflicting
// replay at the same identity is detectable rather than silently duplicated.
func ConsensusEvidenceKey(msg Message) (string, error) {
	if len(msg.Sender) == 0 {
		return "", ErrInvalidEvidenceKey
	}
	switch msg.Type {
	case MessageTypeProposal, MessageTypeVote, MessageTypeFinalityEvidence,
		MessageTypeTimeout, MessageTypePrevote, MessageTypePrecommit:
	default:
		return "", ErrUnsupportedEvidenceType
	}
	var b bytes.Buffer
	putU16(&b, uint16(msg.ProtocolVersion))
	putU64(&b, uint64(msg.ChainID))
	putU64(&b, msg.Epoch)
	putU64(&b, uint64(msg.Height))
	putU64(&b, msg.Round)
	putBytes(&b, msg.Sender)
	b.WriteByte(byte(msg.Type))
	sum := sha256.Sum256(b.Bytes())
	return hex.EncodeToString(sum[:]), nil
}

// ValidateDurableEvidenceMessage validates an authenticated message before
// persistence. It does not mutate consensus state or storage.
func ValidateDurableEvidenceMessage(
	msg Message,
	state RoundState,
	validators ValidatorSet,
	authority TimeoutAuthorityResolver,
) error {
	switch msg.Type {
	case MessageTypeProposal, MessageTypeVote, MessageTypeFinalityEvidence,
		MessageTypeTimeout, MessageTypePrevote, MessageTypePrecommit:
	default:
		return ErrUnsupportedEvidenceType
	}
	if authority == nil {
		return ErrAuthenticatedConsensusAuthorityMissing
	}
	if err := ValidateConsensusMessage(msg, MessageValidationContext{
		Rules: ValidationRules{
			ProtocolVersion: state.ProtocolVersion,
			ChainID:         state.ChainID,
			RequireSender:   true,
			RequireSignature: true,
		},
		State: state,
		Validators: validators,
	}); err != nil {
		return err
	}
	return verifyValidatorMessageSignature(msg, authority)
}

// PersistAuthenticatedEvidence validates first, then performs one idempotent
// persistence operation. Identical replay succeeds; same identity with
// different bytes is rejected as conflicting evidence.
func PersistAuthenticatedEvidence(
	store EvidenceStore,
	msg Message,
	state RoundState,
	validators ValidatorSet,
	authority TimeoutAuthorityResolver,
) error {
	if store == nil {
		return ErrNilEvidenceStore
	}
	if err := ValidateDurableEvidenceMessage(msg, state, validators, authority); err != nil {
		return err
	}
	key, err := ConsensusEvidenceKey(msg)
	if err != nil {
		return err
	}
	encoded, err := EncodeMessage(msg, ValidationRules{
		ProtocolVersion: state.ProtocolVersion,
		ChainID: state.ChainID,
		RequireSender: true,
		RequireSignature: true,
	})
	if err != nil {
		return err
	}
	existing, err := store.LoadConsensusEvidence()
	if err != nil {
		return err
	}
	if prior, ok := existing[key]; ok {
		if bytes.Equal(prior, encoded) {
			return nil
		}
		return ErrConflictingEvidence
	}
	return store.PutConsensusEvidence(key, encoded)
}

// RecoverAuthenticatedEvidence loads and validates all persisted evidence for
// one exact canonical consensus context. Messages are returned deterministically
// by round, type, then sender. Recovery is non-mutating and does not replay
// evidence into ValidatorRuntime automatically.
func RecoverAuthenticatedEvidence(
	store EvidenceStore,
	state RoundState,
	validators ValidatorSet,
	authority TimeoutAuthorityResolver,
) ([]Message, error) {
	if store == nil {
		return nil, ErrNilEvidenceStore
	}
	records, err := store.LoadConsensusEvidence()
	if err != nil {
		return nil, err
	}
	rules := ValidationRules{
		ProtocolVersion: state.ProtocolVersion,
		ChainID: state.ChainID,
		RequireSender: true,
		RequireSignature: true,
	}
	messages := make([]Message, 0, len(records))
	for key, encoded := range records {
		msg, err := DecodeMessage(encoded, rules)
		if err != nil {
			return nil, ErrConflictingEvidence
		}
		computed, err := ConsensusEvidenceKey(msg)
		if err != nil || computed != key {
			return nil, ErrConflictingEvidence
		}
		if msg.Epoch != state.Epoch || msg.Height != state.Height {
			return nil, ErrEvidenceContextMismatch
		}
		messageState := state
		messageState.Round = msg.Round
		messageState.Phase = PhaseProposal
		if err := ValidateDurableEvidenceMessage(msg, messageState, validators, authority); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	sort.Slice(messages, func(i, j int) bool {
		if messages[i].Round != messages[j].Round {
			return messages[i].Round < messages[j].Round
		}
		if messages[i].Type != messages[j].Type {
			return messages[i].Type < messages[j].Type
		}
		return bytes.Compare(messages[i].Sender, messages[j].Sender) < 0
	})
	return messages, nil
}
