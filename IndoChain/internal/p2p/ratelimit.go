package p2p

import (
	"errors"
	"sync"
)

var (
	ErrInvalidRateLimit = errors.New("invalid rate limit")
	ErrRateLimited = errors.New("peer rate limited")
)

type RateLimit struct {
	MaxMessages uint64
}

type RateLimiter struct {
	mu sync.Mutex
	limits map[PeerID]uint64
	config RateLimit
}

func NewRateLimiter(config RateLimit) (*RateLimiter, error) {
	if config.MaxMessages == 0 {
		return nil, ErrInvalidRateLimit
	}
	return &RateLimiter{limits: make(map[PeerID]uint64), config: config}, nil
}

func (r *RateLimiter) Allow(peer PeerID) error {
	if r == nil || peer == "" {
		return ErrInvalidRateLimit
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.limits[peer] >= r.config.MaxMessages {
		return ErrRateLimited
	}
	r.limits[peer]++
	return nil
}

func (r *RateLimiter) Reset(peer PeerID) {
	if r == nil || peer == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.limits, peer)
}
