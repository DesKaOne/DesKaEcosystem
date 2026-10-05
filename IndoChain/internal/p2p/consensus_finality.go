package p2p

import (
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
)

var ErrInvalidFinalityEvidenceSender = errors.New("invalid finality evidence sender")

// PublishFinalityEvidence derives authenticated precommit quorum evidence and
// publishes it as an explicitly signed consensus finality-evidence message.
// Evidence publication does not finalize local runtime state.
func (d *ConsensusRoundDriver) PublishFinalityEvidence(
	peer PeerID,
	sender []byte,
	signer crypto.Signer,
) (consensus.Message, consensus.FinalityCertificate, error) {
	if d == nil || d.driver == nil {
		return consensus.Message{}, consensus.FinalityCertificate{}, ErrNilConsensusRoundDriver
	}
	if len(sender) == 0 || signer == nil {
		return consensus.Message{}, consensus.FinalityCertificate{}, ErrInvalidFinalityEvidenceSender
	}
	certificate, err := d.driver.Runtime().BuildFinalityEvidence(d.driver.Authority())
	if err != nil {
		return consensus.Message{}, consensus.FinalityCertificate{}, err
	}
	payload, err := consensus.EncodeFinalityCertificate(certificate)
	if err != nil {
		return consensus.Message{}, consensus.FinalityCertificate{}, err
	}
	state := d.driver.Runtime().State()
	msg := consensus.Message{
		ProtocolVersion: state.ProtocolVersion,
		ChainID:         state.ChainID,
		Epoch:           state.Epoch,
		Height:          state.Height,
		Round:           state.Round,
		Sender:          append([]byte(nil), sender...),
		Type:            consensus.MessageTypeFinalityEvidence,
		Payload:         payload,
	}
	msg, err = msg.Sign(signer)
	if err != nil {
		return consensus.Message{}, consensus.FinalityCertificate{}, err
	}
	if err := d.Publish(peer, msg); err != nil {
		return consensus.Message{}, consensus.FinalityCertificate{}, err
	}
	return msg, certificate, nil
}

// AcceptFinalityEvidence validates one transport-delivered finality-evidence
// message. It returns the evidence without changing phase or canonical state.
func (d *ConsensusRoundDriver) AcceptFinalityEvidence(
	msg consensus.Message,
) (consensus.FinalityCertificate, error) {
	if d == nil || d.driver == nil {
		return consensus.FinalityCertificate{}, ErrNilConsensusRoundDriver
	}
	return d.driver.Runtime().AcceptAuthenticatedFinalityEvidence(msg, d.driver.Authority())
}
