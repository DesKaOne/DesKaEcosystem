package consensus

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrNilConsensusTimeoutScheduler = errors.New("nil consensus timeout scheduler")
	ErrInvalidConsensusTimeoutDuration = errors.New("invalid consensus timeout duration")
)

type ConsensusTimeoutTimer interface {
	Stop() bool
}

type ConsensusTimeoutTimerFactory func(time.Duration, func()) ConsensusTimeoutTimer

func defaultConsensusTimeoutTimerFactory(duration time.Duration, callback func()) ConsensusTimeoutTimer {
	return time.AfterFunc(duration, callback)
}

// ConsensusTimeoutScheduler owns wall-clock timer lifecycle outside the
// consensus state machine. It fences cancelled/replaced timers with its own
// lifecycle epoch and waits for an in-flight callback before Stop returns.
type ConsensusTimeoutScheduler struct {
	mu      sync.Mutex
	wait    sync.WaitGroup
	factory ConsensusTimeoutTimerFactory
	timer   ConsensusTimeoutTimer
	token   TimeoutToken
	epoch   uint64
	armed   bool
}

func NewConsensusTimeoutScheduler(factory ConsensusTimeoutTimerFactory) (*ConsensusTimeoutScheduler, error) {
	if factory == nil {
		factory = defaultConsensusTimeoutTimerFactory
	}
	return &ConsensusTimeoutScheduler{factory: factory}, nil
}

// Schedule replaces any previously scheduled timer. The supplied token must
// have been produced by ConsensusEngine.ArmTimeout. The scheduler never
// interprets consensus state; it only delivers the exact token to the caller.
func (s *ConsensusTimeoutScheduler) Schedule(token TimeoutToken, duration time.Duration, handler func(TimeoutToken)) error {
	if s == nil {
		return ErrNilConsensusTimeoutScheduler
	}
	if duration <= 0 || handler == nil {
		return ErrInvalidConsensusTimeoutDuration
	}

	s.mu.Lock()
	if s.timer != nil {
		s.timer.Stop()
	}
	s.epoch++
	epoch := s.epoch
	s.armed = true
	s.token = token
	s.timer = nil
	s.mu.Unlock()

	timer := s.factory(duration, func() {
		s.mu.Lock()
		if !s.armed || s.epoch != epoch {
			s.mu.Unlock()
			return
		}
		s.armed = false
		s.timer = nil
		s.wait.Add(1)
		s.mu.Unlock()

		defer s.wait.Done()
		handler(token)
	})

	s.mu.Lock()
	if !s.armed || s.epoch != epoch {
		timer.Stop()
	} else {
		s.timer = timer
	}
	s.mu.Unlock()
	return nil
}

// Cancel fences the current timer and never invokes its handler.
func (s *ConsensusTimeoutScheduler) Cancel() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.epoch++
	s.armed = false
	timer := s.timer
	s.timer = nil
	s.mu.Unlock()
	if timer != nil {
		timer.Stop()
	}
}

// Stop is a lifecycle fence intended for node/session shutdown or restart.
// It prevents future callbacks and waits until an already-delivered callback
// has completed before returning.
func (s *ConsensusTimeoutScheduler) Stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.epoch++
	s.armed = false
	timer := s.timer
	s.timer = nil
	s.mu.Unlock()
	if timer != nil {
		timer.Stop()
	}
	s.wait.Wait()
}

func (s *ConsensusTimeoutScheduler) Armed() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.armed
}
