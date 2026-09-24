package operational

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrSyncWorkerRunning = errors.New("sync worker is already running")

// SyncWorkerLifecycle owns the startup and shutdown of a SyncService worker.
// It keeps cancellation and goroutine ownership outside the synchronization
// implementation while allowing the worker to be restarted after shutdown.
type SyncWorkerLifecycle struct {
	service  *SyncService
	interval time.Duration

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	done    chan struct{}
	err     error
}

func NewSyncWorkerLifecycle(service *SyncService, interval time.Duration) (*SyncWorkerLifecycle, error) {
	if service == nil {
		return nil, errors.New("sync service is required")
	}
	if interval <= 0 {
		return nil, errors.New("sync interval must be greater than zero")
	}
	return &SyncWorkerLifecycle{service: service, interval: interval}, nil
}

// Start starts one owned synchronization worker. The worker performs its
// immediate synchronization before waiting for the periodic interval.
func (l *SyncWorkerLifecycle) Start(parent context.Context) error {
	if parent == nil {
		return errors.New("parent context is required")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.running {
		return ErrSyncWorkerRunning
	}

	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	l.cancel = cancel
	l.done = done
	l.err = nil
	l.running = true

	go func() {
		err := l.service.Run(ctx, l.interval)

		l.mu.Lock()
		l.err = err
		l.running = false
		close(done)
		l.mu.Unlock()
	}()

	return nil
}

// Shutdown requests cancellation and waits for the owned worker to exit.
// Normal context cancellation is treated as a clean shutdown.
func (l *SyncWorkerLifecycle) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return errors.New("shutdown context is required")
	}

	l.mu.Lock()
	if !l.running {
		err := l.err
		l.mu.Unlock()
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}
	cancel := l.cancel
	done := l.done
	l.mu.Unlock()

	cancel()

	select {
	case <-done:
		l.mu.Lock()
		err := l.err
		l.mu.Unlock()
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
