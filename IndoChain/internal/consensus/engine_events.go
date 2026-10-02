package consensus

// ProcessMessage routes one authenticated message and returns any locally
// generated authenticated consensus messages required by the resulting phase.
// This is the production event bridge: proposal -> local prevote, prevote
// quorum -> local precommit, precommit quorum -> finality.
//
// Returned messages are intentionally not sent by this method. The node/P2P
// layer owns dissemination and canonical commit.
func (e *ConsensusEngine) ProcessMessage(msg Message) ([]Message, error) {
	if e == nil || e.runtime == nil || e.driver == nil {
		return nil, ErrNilConsensusEngine
	}
	before := e.runtime.State()
	if err := e.driver.HandleMessage(msg); err != nil {
		return nil, err
	}
	after := e.runtime.State()

	var out []Message
	if before != after {
		switch after.Phase {
		case PhasePrevote:
			vote, err := e.localVote(MessageTypePrevote)
			if err != nil {
				return nil, err
			}
			out = append(out, vote)
		case PhasePrecommit:
			vote, err := e.localVote(MessageTypePrecommit)
			if err != nil {
				return nil, err
			}
			out = append(out, vote)
		case PhaseFinalized:
			// Finality is already authenticated by ValidatorRuntime. No
			// additional message is required; the node consumes the
			// FinalizedCertificate through its canonical commit boundary.
		}
		_, _, _ = e.ArmTimeout()
	}
	if after.Phase == PhasePrecommit {
		// A local precommit can itself complete the quorum. The runtime does
		// not finalize until authenticated evidence is explicitly checked.
		if _, err := e.runtime.FinalizedCertificate(); err != nil {
			if _, err := e.runtime.FinalizeProposal(e.driver.authority); err == nil {
				e.CancelTimeout()
			} else if err != ErrInvalidRuntimePhase {
				// Keep normal "not enough precommits yet" behavior silent.
			}
		}
	}
	return out, nil
}

func (e *ConsensusEngine) localVote(kind MessageType) (Message, error) {
	if e == nil || e.runtime == nil {
		return Message{}, ErrNilConsensusEngine
	}
	if kind != MessageTypePrevote && kind != MessageTypePrecommit {
		return Message{}, ErrInvalidRuntimeVoteType
	}
	state := e.runtime.State()
	if len(e.runtime.proposal) == 0 {
		return Message{}, ErrInvalidConsensusRuntime
	}
	msg := Message{
		ProtocolVersion: state.ProtocolVersion,
		ChainID:         state.ChainID,
		Epoch: state.Epoch,
		Height: state.Height,
		Round: state.Round,
		Sender: append([]byte(nil), e.validator...),
		Type: kind,
		Payload: append([]byte(nil), e.runtime.proposal...),
	}
	signed, err := msg.Sign(e.signer)
	if err != nil {
		return Message{}, err
	}
	return signed, nil
}
