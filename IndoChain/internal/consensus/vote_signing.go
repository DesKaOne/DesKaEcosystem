package consensus

import (
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
)

var ErrInvalidVoteSigningContext = errors.New("invalid vote signing context")

// BuildSignedVoteMessage builds authenticated phase-specific vote evidence for
// the exact current consensus context and supplied proposal payload.
func BuildSignedVoteMessage(
	state RoundState,
	sender []byte,
	voteType MessageType,
	payload []byte,
	signer crypto.Signer,
) (Message, error) {
	if err := state.Validate(); err != nil {
		return Message{}, err
	}
	if signer == nil || len(sender) == 0 || len(payload) == 0 {
		return Message{}, ErrInvalidVoteSigningContext
	}
	if voteType != MessageTypePrevote && voteType != MessageTypePrecommit {
		return Message{}, ErrInvalidRuntimeVoteType
	}

	msg := Message{
		ProtocolVersion: state.ProtocolVersion,
		ChainID:         state.ChainID,
		Epoch:           state.Epoch,
		Height:          state.Height,
		Round:           state.Round,
		Sender:          append([]byte(nil), sender...),
		Type:            voteType,
		Payload:         append([]byte(nil), payload...),
	}
	return msg.Sign(signer)
}
