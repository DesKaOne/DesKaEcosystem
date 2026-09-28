package routing

import (
	"context"
	"testing"
	"time"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
	mock "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/Mock"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
)

type stubOperationalInputReader struct {
	input OperationalInput
	calls int
}

func (s *stubOperationalInputReader) ReadOperationalInput(ctx context.Context, providerName string, maxAge time.Duration, now time.Time) (OperationalInput, error) {
	s.calls++
	return s.input, nil
}

func TestRouterConsumesReadOnlyOperationalInputBoundary(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", mock.New(mock.Config{
		Products: []provider.Product{{Code: "xld10", Name: "Test"}},
	})); err != nil {
		t.Fatal(err)
	}

	store := operational.NewMemoryStore()
	if err := store.Put(operational.Snapshot{
		ProviderName:  "mock",
		Balance:       100000,
		Currency:      "IDR",
		Health:        operational.HealthHealthy,
		LastCheckedAt: time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	before, ok := store.Get("mock")
	if !ok {
		t.Fatal("expected operational snapshot")
	}

	reader := &stubOperationalInputReader{
		input: OperationalInput{
			Snapshot:  before,
			Freshness: operational.FreshnessFresh,
		},
	}
	router, err := New(registry, store, nil)
	if err != nil {
		t.Fatal(err)
	}
	router.OperationalInput = reader
	router.Now = func() time.Time { return before.LastCheckedAt }

	got, err := router.Select(context.Background(), Request{ProductCode: "xld10", Amount: 50000})
	if err != nil {
		t.Fatal(err)
	}
	if got != "mock" {
		t.Fatalf("expected mock provider, got %q", got)
	}
	if reader.calls != 1 {
		t.Fatalf("expected one read-only operational input, got %d", reader.calls)
	}

	after, ok := store.Get("mock")
	if !ok {
		t.Fatal("expected operational snapshot after routing")
	}
	if after != before {
		t.Fatalf("routing mutated operational state: before=%#v after=%#v", before, after)
	}
}

func TestStoreOperationalInputReaderDoesNotExposeMutation(t *testing.T) {
	store := operational.NewMemoryStore()
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	want := operational.Snapshot{
		ProviderName:  "mock",
		Balance:       125000,
		Currency:      "IDR",
		Health:        operational.HealthHealthy,
		LastCheckedAt: now,
	}
	if err := store.Put(want); err != nil {
		t.Fatal(err)
	}

	reader, err := NewStoreOperationalInputReader(store)
	if err != nil {
		t.Fatal(err)
	}
	input, err := reader.ReadOperationalInput(context.Background(), "mock", time.Minute, now)
	if err != nil {
		t.Fatal(err)
	}
	if input.Snapshot != want {
		t.Fatalf("unexpected operational handoff: got=%#v want=%#v", input.Snapshot, want)
	}
	if input.Freshness != operational.FreshnessFresh {
		t.Fatalf("expected fresh operational handoff, got %q", input.Freshness)
	}

	got, ok := store.Get("mock")
	if !ok || got != want {
		t.Fatalf("read-only handoff changed operational state: got=%#v ok=%v", got, ok)
	}
}
