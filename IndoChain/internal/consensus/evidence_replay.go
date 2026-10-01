package consensus

import (
	"bytes"
	"errors"
	"sort"
)

var (
	ErrNilReplayRuntime       = errors.New("nil consensus replay runtime")
	ErrReplayRoundGap        = errors.New("consensus evidence replay round gap")
	ErrReplayMissingProposal = errors.New("consensus evidence replay missing proposal")
	ErrReplayConflictingProposal = errors.New("consensus evidence replay conflicting proposal")
	ErrReplayUnsupportedEvidence = errors.New("consensus evidence is not replayable")
)

// EvidenceReplayResult describes what a restart replay actually applied.
// FinalityEvidence is counted separately because it is validated but does not
// mutate ValidatorRuntime.
type EvidenceReplayResult struct {
	AppliedProposals uint64
	AppliedVotes     uint64
	AppliedTimeouts  uint64
	ValidatedFinality uint64
}

// ReplayAuthenticatedEvidence deterministically replays authenticated evidence
// into a fresh ValidatorRuntime. Only operational round evidence mutates the
// runtime: proposals, phase-specific votes, and quorum-backed timeout changes.
// Finality evidence is validated against the resulting runtime but never causes
// an implicit finalize/commit transition.
func ReplayAuthenticatedEvidence(
	runtime *ValidatorRuntime,
	evidence []Message,
	authority TimeoutAuthorityResolver,
) (EvidenceReplayResult, error) {
	if runtime == nil {
		return EvidenceReplayResult{}, ErrNilReplayRuntime
	}
	if authority == nil {
		return EvidenceReplayResult{}, ErrAuthenticatedConsensusAuthorityMissing
	}

	ordered := cloneVotes(evidence)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Round != ordered[j].Round {
			return ordered[i].Round < ordered[j].Round
		}
		return replayMessageOrder(ordered[i].Type) < replayMessageOrder(ordered[j].Type)
	})

	result := EvidenceReplayResult{}
	byRound := make(map[uint64][]Message)
	rounds := make([]uint64, 0)
	seenRound := make(map[uint64]bool)
	for _, msg := range ordered {
		if msg.Type == MessageTypeValidatorSetUpdate {
			return result, ErrReplayUnsupportedEvidence
		}
		if msg.Round < runtime.state.Round {
			return result, ErrReplayRoundGap
		}
		if !seenRound[msg.Round] {
			seenRound[msg.Round] = true
			rounds = append(rounds, msg.Round)
		}
		byRound[msg.Round] = append(byRound[msg.Round], msg)
	}
	sort.Slice(rounds, func(i, j int) bool { return rounds[i] < rounds[j] })

	currentRound := runtime.state.Round
	for _, round := range rounds {
		if round != currentRound {
			return result, ErrReplayRoundGap
		}
		messages := byRound[round]

		proposals := make([]Message, 0, 1)
		timeouts := make([]Message, 0)
		finality := make([]Message, 0)
		for _, msg := range messages {
			switch msg.Type {
			case MessageTypeProposal:
				proposals = append(proposals, msg)
			case MessageTypePrevote, MessageTypePrecommit, MessageTypeVote:
				if err := runtime.AddAuthenticatedVote(msg, authority); err != nil {
					// Votes are intentionally processed only after the proposal
					// below; defer them until the proposal phase is entered.
				}
			case MessageTypeTimeout:
				timeouts = append(timeouts, msg)
			case MessageTypeFinalityEvidence:
				finality = append(finality, msg)
			default:
				return result, ErrReplayUnsupportedEvidence
			}
		}

		if len(proposals) > 1 {
			for i := 1; i < len(proposals); i++ {
				if !bytes.Equal(proposals[0].Payload, proposals[i].Payload) {
					return result, ErrReplayConflictingProposal
				}
			}
		}
		if len(proposals) > 0 {
			if err := runtime.AcceptAuthenticatedProposal(proposals[0], authority); err != nil {
				return result, err
			}
			result.AppliedProposals++
		}

		// Re-run vote messages in canonical phase order. The pre-scan above
		// deliberately does not mutate because the runtime must see Proposal
		// before any vote.
		for _, msg := range messages {
			switch msg.Type {
			case MessageTypePrevote, MessageTypeVote, MessageTypePrecommit:
				if err := runtime.AddAuthenticatedVote(msg, authority); err != nil {
					return result, err
				}
				result.AppliedVotes++
			}
		}

		for _, msg := range finality {
			if _, err := runtime.AcceptAuthenticatedFinalityEvidence(msg, authority); err != nil {
				return result, err
			}
			result.ValidatedFinality++
		}

		if len(timeouts) > 0 {
			if _, err := runtime.AdvanceRoundWithTimeoutEvidence(timeouts, authority); err != nil {
				return result, err
			}
			result.AppliedTimeouts += uint64(len(timeouts))
			currentRound = runtime.state.Round
		}
	}

	return result, nil
}

func replayMessageOrder(t MessageType) int {
	switch t {
	case MessageTypeProposal:
		return 10
	case MessageTypePrevote, MessageTypeVote:
		return 20
	case MessageTypePrecommit:
		return 30
	case MessageTypeFinalityEvidence:
		return 40
	case MessageTypeTimeout:
		return 50
	default:
		return 100
	}
}
