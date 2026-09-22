package p2p

import "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"

// NextRequestMessage plans the next sync range and wraps it in the existing
// development P2P message envelope. It returns false when already caught up.
func (s *SyncSession) NextRequestMessage(remoteHeight types.Height, maxLimit uint64, maxPayload uint32) (Message, bool, error) {
	if s == nil {
		return Message{}, false, ErrNilSyncSession
	}
	req, needed, err := s.NextRequest(remoteHeight)
	if err != nil {
		return Message{}, false, err
	}
	if !needed {
		return Message{}, false, nil
	}
	msg, err := BuildBlockRequestMessage(req, maxLimit, maxPayload)
	if err != nil {
		return Message{}, false, err
	}
	return msg, true, nil
}
