package operational

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrSyncWorkerRunning = errors.New("sync worker is already running")
var ErrSyncWorkerExited = errors.New("sync worker exited unexpectedly")

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

// Wait blocks until the current worker exits or ctx is canceled. It is safe
// to call after Start and does not alter worker ownership.
// Done returns the completion signal for the current worker. Before Start it returns nil.
// The channel is closed exactly once for that worker generation.
func (l *SyncWorkerLifecycle) Done() <-chan struct{} {
	if l == nil { return nil }
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.done
}

func (l *SyncWorkerLifecycle) Wait(ctx context.Context) error {
	if ctx == nil {
		return errors.New("wait context is required")
	}

	l.mu.Lock()
	if !l.running {
		err := l.err
		l.mu.Unlock()
		return normalizeWorkerExitError(err)
	}
	done := l.done
	l.mu.Unlock()

	select {
	case <-done:
		l.mu.Lock()
		err := l.err
		l.mu.Unlock()
		return normalizeWorkerExitError(err)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Shutdown requests cancellation and waits for the owned worker to exit.
// Normal context cancellation is treated as a clean shutdown.
func (l *SyncWorkerLifecycle) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return errors.New("shutdown context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	l.mu.Lock()
	if !l.running {
		err := l.err
		l.mu.Unlock()
		return normalizeWorkerExitError(err)
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
		return normalizeWorkerExitError(err)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (l *SyncWorkerLifecycle) Running() bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.running
}

func normalizeWorkerExitError(err error) error {
	if errors.Is(err, context.Canceled) {
		return nil
	}
	if err == nil {
		return ErrSyncWorkerExited
	}
	return err
}
