package routing

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

func TestMemoryTransactionStoreContextCancellationDoesNotMutateState(t *testing.T) {
	store := NewMemoryTransactionStore()
	pending := postgresPendingState()
	if err := store.Put(pending); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, ok := store.GetContext(ctx, pending.Request.ReferenceID); ok {
		t.Fatal("canceled GetContext must not expose transaction state")
	}
	if got := store.AllContext(ctx); got != nil {
		t.Fatalf("canceled AllContext must not expose transaction state, got %#v", got)
	}

	next := pending
	next.Execution.Result.Status = provider.StatusSuccess
	if err := store.PutContext(ctx, next); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled PutContext, got %v", err)
	}
	if err := store.PutIfCurrentContext(ctx, pending.Request.ReferenceID, pending, next); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled PutIfCurrentContext, got %v", err)
	}

	current, ok := store.Get(pending.Request.ReferenceID)
	if !ok {
		t.Fatal("transaction disappeared after canceled persistence operations")
	}
	if current != pending {
		t.Fatalf("canceled persistence mutated durable state: %#v != %#v", current, pending)
	}
}

func TestMemoryTransactionStoreContextDeadlineDoesNotMutateState(t *testing.T) {
	store := NewMemoryTransactionStore()
	pending := postgresPendingState()
	if err := store.Put(pending); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer cancel()

	next := pending
	next.Execution.Result.Status = provider.StatusFailed
	if err := store.PutContext(ctx, next); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded from PutContext, got %v", err)
	}
	if err := store.PutIfCurrentContext(ctx, pending.Request.ReferenceID, pending, next); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded from PutIfCurrentContext, got %v", err)
	}

	current, ok := store.Get(pending.Request.ReferenceID)
	if !ok {
		t.Fatal("transaction disappeared after deadline persistence operations")
	}
	if current != pending {
		t.Fatalf("deadline persistence mutated durable state: %#v != %#v", current, pending)
	}
}

type contextCancellationDBStub struct {
	called bool
}

func (s *contextCancellationDBStub) ExecContext(ctx context.Context, _ string, _ ...any) (sql.Result, error) {
	s.called = true
	return nil, ctx.Err()
}

func (s *contextCancellationDBStub) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, errors.New("not used")
}

func (s *contextCancellationDBStub) QueryRowContext(context.Context, string, ...any) *sql.Row {
	panic("not used")
}

func TestPostgresTransactionStoreContextCancellationPropagatesToDB(t *testing.T) {
	stub := &contextCancellationDBStub{}
	store, err := NewPostgresTransactionStore(stub)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	pending := postgresPendingState()
	next := pending
	next.Execution.Result.Status = provider.StatusSuccess

	err = store.PutIfCurrentContext(ctx, pending.Request.ReferenceID, pending, next)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation from PostgreSQL persistence, got %v", err)
	}
	if !stub.called {
		t.Fatal("expected context-aware persistence to reach the database boundary")
	}
}


func TestMemoryTransactionStoreContextReadReturnsCancellationError(t *testing.T) {
	store := NewMemoryTransactionStore()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, ok, err := store.GetContextE(ctx, "missing"); !errors.Is(err, context.Canceled) || ok {
		t.Fatalf("expected canceled GetContextE, got ok=%v err=%v", ok, err)
	}
	if states, err := store.AllContextE(ctx); !errors.Is(err, context.Canceled) || states != nil {
		t.Fatalf("expected canceled AllContextE, got states=%#v err=%v", states, err)
	}
}

func TestPostgresTransactionStoreContextReadPreservesDatabaseError(t *testing.T) {
	stub := &contextReadDBStub{err: errors.New("database unavailable")}
	store, err := NewPostgresTransactionStore(stub)
	if err != nil {
		t.Fatal(err)
	}

	if states, err := store.AllContextE(context.Background()); err == nil || states != nil {
		t.Fatalf("expected database read error, got states=%#v err=%v", states, err)
	}
}

type contextReadDBStub struct {
	err error
}

func (s *contextReadDBStub) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return nil, s.err
}

func (s *contextReadDBStub) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, s.err
}

func (s *contextReadDBStub) QueryRowContext(context.Context, string, ...any) *sql.Row {
	panic("not used")
}
