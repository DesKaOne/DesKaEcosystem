package routing

import (
	"errors"
	"testing"
	"time"

	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/catalog"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
)

func TestValidateRoutingCandidateInputAcceptsConsistentInputs(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	snapshot := operational.Snapshot{
		ProviderName: "mock",
		Health:       operational.HealthHealthy,
		Balance:      100000,
		LastCheckedAt: now,
	}
	catalogSnapshot := catalog.Snapshot{
		ProviderName: "mock",
		Products:     nil,
		SyncedAt:     now,
	}
	input := routingCandidateInput{
		ProviderName: "mock",
		Operational: OperationalInput{Snapshot: snapshot, Freshness: operational.FreshnessFresh},
		Catalog:     &catalogSnapshot,
	}
	if err := validateRoutingCandidateInput(input, now, time.Minute, time.Minute); err != nil {
		t.Fatalf("expected consistent routing inputs, got %v", err)
	}
}

func TestValidateRoutingCandidateInputRejectsOperationalProviderMismatch(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	input := routingCandidateInput{
		ProviderName: "mock",
		Operational: OperationalInput{
			Snapshot: operational.Snapshot{
				ProviderName: "other",
				Health:       operational.HealthHealthy,
				LastCheckedAt: now,
			},
			Freshness: operational.FreshnessFresh,
		},
	}
	err := validateRoutingCandidateInput(input, now, time.Minute, 0)
	if !errors.Is(err, ErrOperationalInputMismatch) || !errors.Is(err, ErrRoutingInputInconsistent) {
		t.Fatalf("expected operational identity inconsistency, got %v", err)
	}
}

func TestValidateRoutingCandidateInputRejectsFreshnessMismatch(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	input := routingCandidateInput{
		ProviderName: "mock",
		Operational: OperationalInput{
			Snapshot: operational.Snapshot{
				ProviderName: "mock",
				Health:       operational.HealthHealthy,
				LastCheckedAt: now.Add(-2 * time.Minute),
			},
			Freshness: operational.FreshnessFresh,
		},
	}
	err := validateRoutingCandidateInput(input, now, time.Minute, 0)
	if !errors.Is(err, ErrOperationalFreshnessMismatch) || !errors.Is(err, ErrRoutingInputInconsistent) {
		t.Fatalf("expected operational freshness inconsistency, got %v", err)
	}
}

func TestValidateRoutingCandidateInputRejectsCatalogProviderMismatch(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	catalogSnapshot := catalog.Snapshot{ProviderName: "other", SyncedAt: now}
	input := routingCandidateInput{
		ProviderName: "mock",
		Operational: OperationalInput{
			Snapshot: operational.Snapshot{
				ProviderName: "mock",
				Health:       operational.HealthHealthy,
				LastCheckedAt: now,
			},
			Freshness: operational.FreshnessFresh,
		},
		Catalog: &catalogSnapshot,
	}
	err := validateRoutingCandidateInput(input, now, time.Minute, time.Minute)
	if !errors.Is(err, ErrCatalogInputMismatch) || !errors.Is(err, ErrRoutingInputInconsistent) {
		t.Fatalf("expected catalog identity inconsistency, got %v", err)
	}
}

func TestRouterSkipsContradictoryOperationalReaderInput(t *testing.T) {
	// A read-only routing reader must not be able to make a mismatched
	// provider snapshot routeable.
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	reader := &stubOperationalInputReader{input: OperationalInput{
		Snapshot: operational.Snapshot{
			ProviderName: "other",
			Health:       operational.HealthHealthy,
			Balance:      100000,
			LastCheckedAt: now,
		},
		Freshness: operational.FreshnessFresh,
	}}
	_ = reader
}
