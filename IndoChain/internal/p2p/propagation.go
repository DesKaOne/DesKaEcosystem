package p2p

import "errors"

var (
	ErrInvalidPropagation = errors.New("invalid propagation policy")
	ErrDuplicateMessage = errors.New("duplicate gossip message")
)

type PropagationPolicy struct {
	MaxPeers int
}

type Propagator struct {
	policy PropagationPolicy
	seen map[string]struct{}
}

func NewPropagator(policy PropagationPolicy) (*Propagator, error) {
	if policy.MaxPeers <= 0 {
		return nil, ErrInvalidPropagation
	}
	return &Propagator{
		policy: policy,
		seen: make(map[string]struct{}),
	}, nil
}

func (p *Propagator) MarkSeen(messageID string) error {
	if p == nil || messageID == "" {
		return ErrInvalidPropagation
	}
	if _, ok := p.seen[messageID]; ok {
		return ErrDuplicateMessage
	}
	p.seen[messageID] = struct{}{}
	return nil
}

func (p *Propagator) Fanout(peerCount int) int {
	if p == nil || peerCount <= 0 {
		return 0
	}
	if peerCount < p.policy.MaxPeers {
		return peerCount
	}
	return p.policy.MaxPeers
}
