package consensus

// ProcessMessage routes one authenticated message and automatically creates
// and consumes this validator's own vote for each phase transition. Generated
// messages are returned for P2P dissemination; they are also applied locally
// exactly once, so the local validator contributes its voting power without
// requiring a network loopback.
func (e *ConsensusEngine) ProcessMessage(msg Message) ([]Message, error) {
	if e == nil || e.runtime == nil || e.driver == nil { return nil, ErrNilConsensusEngine }
	if err := e.driver.HandleMessage(msg); err != nil { return nil, err }
	var out []Message
	for {
		switch e.runtime.State().Phase {
		case PhasePrevote:
			if !hasLocalVote(e.runtime.prevotes, e.validator) {
				vote, err := e.localVote(MessageTypePrevote); if err != nil { return nil, err }
				if err := e.driver.HandleMessage(vote); err != nil { return nil, err }
				out = append(out, vote)
				if e.runtime.State().Phase == PhasePrevote { _, _, _ = e.ArmTimeout(); return out, nil }
				continue
			}
			_, _, _ = e.ArmTimeout()
			return out, nil

		case PhasePrecommit:
			if _, err := e.runtime.FinalizedCertificate(); err == nil { e.CancelTimeout(); return out, nil }
			if !hasLocalVote(e.runtime.precommits, e.validator) {
				vote, err := e.localVote(MessageTypePrecommit); if err != nil { return nil, err }
				if err := e.driver.HandleMessage(vote); err != nil { return nil, err }
				out = append(out, vote)
			}
			if _, err := e.runtime.FinalizeProposal(e.driver.authority); err == nil { e.CancelTimeout(); return out, nil }
			_, _, _ = e.ArmTimeout()
			return out, nil

		case PhaseFinalized:
			e.CancelTimeout(); return out, nil
		default:
			_, _, _ = e.ArmTimeout(); return out, nil
		}
	}
}

func hasLocalVote(aggregator *VoteAggregator, validator []byte) bool {
	if aggregator == nil { return false }
	for _, vote := range aggregator.Votes {
		if bytes.Equal(vote.Sender, validator) { return true }
	}
	return false
}

func (e *ConsensusEngine) localVote(kind MessageType) (Message, error) {
	if e == nil || e.runtime == nil { return Message{}, ErrNilConsensusEngine }
	if kind != MessageTypePrevote && kind != MessageTypePrecommit { return Message{}, ErrInvalidRuntimeVoteType }
	state := e.runtime.State()
	if len(e.runtime.proposal) == 0 { return Message{}, ErrInvalidConsensusRuntime }
	msg := Message{ProtocolVersion: state.ProtocolVersion, ChainID: state.ChainID, Epoch: state.Epoch, Height: state.Height, Round: state.Round, Sender: append([]byte(nil), e.validator...), Type: kind, Payload: append([]byte(nil), e.runtime.proposal...)}
	return msg.Sign(e.signer)
}
