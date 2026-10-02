package routing

import (
	"context"
	"errors"
	"time"

	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
)

// OperationalInput is an immutable, read-only handoff from the operational
// observation boundary to routing. Routing owns the eligibility decision;
// this value never authorizes a financial side effect.
type OperationalInput struct {
	Snapshot  operational.Snapshot
	Freshness operational.Freshness
}

// OperationalInputReader exposes only the operational data routing needs.
// Implementations must not mutate operational state as part of a read.
type OperationalInputReader interface {
	ReadOperationalInput(ctx context.Context, providerName string, maxAge time.Duration, now time.Time) (OperationalInput, error)
}

// errorAwareOperationalStore is an optional read-error-aware extension of the
// operational Store contract. It preserves the base Store API while allowing
// routing to distinguish a persistence failure from a genuine snapshot miss.
type errorAwareOperationalStore interface {
	operational.Store
	GetWithError(string) (operational.Snapshot, bool, error)
}

// StoreOperationalInputReader adapts the existing operational Store into the
// explicit read-only routing boundary without exposing Store mutation methods.
type StoreOperationalInputReader struct {
	Store operational.Store
}

func NewStoreOperationalInputReader(store operational.Store) (*StoreOperationalInputReader, error) {
	if store == nil {
		return nil, errors.New("operational store is required")
	}
	return &StoreOperationalInputReader{Store: store}, nil
}

func (r *StoreOperationalInputReader) ReadOperationalInput(ctx context.Context, providerName string, maxAge time.Duration, now time.Time) (OperationalInput, error) {
	if err := ctx.Err(); err != nil {
		return OperationalInput{}, err
	}
	var snapshot operational.Snapshot
	var ok bool
	var readErr error
	if aware, supported := r.Store.(errorAwareOperationalStore); supported {
		snapshot, ok, readErr = aware.GetWithError(providerName)
	} else {
		snapshot, ok = r.Store.Get(providerName)
	}
	if readErr != nil {
		return OperationalInput{}, readErr
	}
	if !ok {
		return OperationalInput{}, operational.ErrOperationalSnapshotNotFound
	}
	freshness := operational.FreshnessUnknown
	if maxAge > 0 {
		var err error
		freshness, err = snapshot.EvaluateFreshness(now, maxAge)
		if err != nil {
			return OperationalInput{}, err
		}
	}
	return OperationalInput{Snapshot: snapshot, Freshness: freshness}, nil
}
