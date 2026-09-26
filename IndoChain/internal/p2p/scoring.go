package p2p

import (
	"errors"
	"sort"
)

var ErrInvalidPeerScore = errors.New("invalid peer score")

type PeerScore struct {
	ID    PeerID
	Score int64
}

type PeerSelector struct {
	MinScore int64
	MaxPeers int
}

func (s PeerSelector) Select(peers []PeerScore) ([]PeerID, error) {
	if s.MaxPeers <= 0 {
		return nil, ErrInvalidPeerScore
	}
	eligible := make([]PeerScore, 0, len(peers))
	seen := make(map[PeerID]struct{}, len(peers))
	for _, peer := range peers {
		if peer.ID == "" || peer.Score < s.MinScore {
			continue
		}
		if _, ok := seen[peer.ID]; ok {
			continue
		}
		seen[peer.ID] = struct{}{}
		eligible = append(eligible, peer)
	}
	sort.SliceStable(eligible, func(i, j int) bool {
		if eligible[i].Score != eligible[j].Score {
			return eligible[i].Score > eligible[j].Score
		}
		return eligible[i].ID < eligible[j].ID
	})
	if len(eligible) > s.MaxPeers {
		eligible = eligible[:s.MaxPeers]
	}
	out := make([]PeerID, len(eligible))
	for i := range eligible {
		out[i] = eligible[i].ID
	}
	return out, nil
}
