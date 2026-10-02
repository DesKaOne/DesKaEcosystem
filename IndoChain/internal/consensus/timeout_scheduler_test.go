package consensus

import (
	"sync"
	"testing"
	"time"
)

type schedulerTestTimer struct {
	mu      sync.Mutex
	stopped bool
	fireFn  func()
}

func (t *schedulerTestTimer) Stop() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	wasActive := !t.stopped
	t.stopped = true
	return wasActive
}

func (t *schedulerTestTimer) Fire() {
	t.mu.Lock()
	if t.stopped {
		t.mu.Unlock()
		return
	}
	fn := t.fireFn
	t.stopped = true
	t.mu.Unlock()
	fn()
}

type schedulerTestClock struct {
	mu     sync.Mutex
	timers []*schedulerTestTimer
}

func (c *schedulerTestClock) AfterFunc(_ time.Duration, fn func()) ConsensusTimeoutTimer {
	timer := &schedulerTestTimer{fireFn: fn}
	c.mu.Lock()
	c.timers = append(c.timers, timer)
	c.mu.Unlock()
	return timer
}

func (c *schedulerTestClock) Timer(index int) *schedulerTestTimer {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.timers[index]
}

func TestConsensusTimeoutSchedulerFencesReplacedTimer(t *testing.T) {
	clock := &schedulerTestClock{}
	scheduler, err := NewConsensusTimeoutScheduler(clock.AfterFunc)
	if err != nil {
		t.Fatal(err)
	}

	first := TimeoutToken{Height: 7, Round: 0, Phase: PhaseProposal, Generation: 1}
	second := TimeoutToken{Height: 7, Round: 0, Phase: PhaseProposal, Generation: 2}
	var got []TimeoutToken
	if err := scheduler.Schedule(first, time.Second, func(token TimeoutToken) { got = append(got, token) }); err != nil {
		t.Fatal(err)
	}
	if err := scheduler.Schedule(second, time.Second, func(token TimeoutToken) { got = append(got, token) }); err != nil {
		t.Fatal(err)
	}

	clock.Timer(0).Fire()
	if len(got) != 0 {
		t.Fatalf("replaced timer invoked handler: %+v", got)
	}
	clock.Timer(1).Fire()
	if len(got) != 1 || got[0] != second {
		t.Fatalf("active timer result = %+v", got)
	}
}

func TestConsensusTimeoutSchedulerCancelFencesTimer(t *testing.T) {
	clock := &schedulerTestClock{}
	scheduler, err := NewConsensusTimeoutScheduler(clock.AfterFunc)
	if err != nil {
		t.Fatal(err)
	}
	token := TimeoutToken{Height: 7, Round: 1, Phase: PhasePrevote, Generation: 3}
	called := false
	if err := scheduler.Schedule(token, time.Second, func(TimeoutToken) { called = true }); err != nil {
		t.Fatal(err)
	}
	scheduler.Cancel()
	clock.Timer(0).Fire()
	if called {
		t.Fatal("cancelled timeout invoked handler")
	}
	if scheduler.Armed() {
		t.Fatal("scheduler remained armed after cancel")
	}
}

func TestConsensusTimeoutSchedulerStopFencesPreRestartTimer(t *testing.T) {
	clock := &schedulerTestClock{}
	oldScheduler, err := NewConsensusTimeoutScheduler(clock.AfterFunc)
	if err != nil {
		t.Fatal(err)
	}
	oldToken := TimeoutToken{Height: 9, Round: 2, Phase: PhasePrecommit, Generation: 1}
	oldCalled := false
	if err := oldScheduler.Schedule(oldToken, time.Second, func(TimeoutToken) { oldCalled = true }); err != nil {
		t.Fatal(err)
	}

	oldScheduler.Stop()

	newScheduler, err := NewConsensusTimeoutScheduler(clock.AfterFunc)
	if err != nil {
		t.Fatal(err)
	}
	newToken := oldToken
	newCalled := false
	if err := newScheduler.Schedule(newToken, time.Second, func(TimeoutToken) { newCalled = true }); err != nil {
		t.Fatal(err)
	}

	clock.Timer(0).Fire()
	if oldCalled {
		t.Fatal("pre-restart timer escaped scheduler stop fence")
	}
	clock.Timer(1).Fire()
	if !newCalled {
		t.Fatal("new scheduler did not deliver its active timer")
	}
}
