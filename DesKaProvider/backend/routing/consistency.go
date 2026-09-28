package routing

import (
	"errors"
	"fmt"
	"time"

	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/catalog"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
)

var (
	ErrRoutingInputInconsistent = errors.New("routing eligibility inputs are inconsistent")
	ErrOperationalInputMismatch = errors.New("operational input provider mismatch")
	ErrOperationalFreshnessMismatch = errors.New("operational input freshness mismatch")
	ErrCatalogInputMismatch = errors.New("catalog input provider mismatch")
	ErrCatalogFreshnessMismatch = errors.New("catalog input freshness mismatch")
	ErrInvalidRoutingPriority = errors.New("invalid routing priority")
)

type routingCandidateInput struct {
	ProviderName string
	Operational OperationalInput
	Catalog     *catalog.Snapshot
	Priority    int
}

func validateRoutingCandidateInput(input routingCandidateInput, now time.Time, operationalMaxAge, catalogMaxAge time.Duration) error {
	if input.ProviderName == "" {
		return fmt.Errorf("%w: provider name is required", ErrRoutingInputInconsistent)
	}
	if input.Operational.Snapshot.ProviderName != input.ProviderName {
		return errors.Join(ErrRoutingInputInconsistent, ErrOperationalInputMismatch)
	}
	if operationalMaxAge > 0 {
		expected, err := input.Operational.Snapshot.EvaluateFreshness(now, operationalMaxAge)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrRoutingInputInconsistent, err)
		}
		if input.Operational.Freshness != expected {
			return errors.Join(ErrRoutingInputInconsistent, ErrOperationalFreshnessMismatch)
		}
	}
	if input.Catalog != nil {
		if input.Catalog.ProviderName != input.ProviderName {
			return errors.Join(ErrRoutingInputInconsistent, ErrCatalogInputMismatch)
		}
		if catalogMaxAge > 0 {
			expected := operational.FreshnessUnknown
			if input.Catalog.SyncedAt.IsZero() {
				return errors.Join(ErrRoutingInputInconsistent, ErrCatalogFreshnessMismatch)
			}
			age := now.Sub(input.Catalog.SyncedAt)
			if age < 0 {
				expected = operational.FreshnessStale
			} else if age <= catalogMaxAge {
				expected = operational.FreshnessFresh
			} else {
				expected = operational.FreshnessStale
			}
			if expected == operational.FreshnessUnknown {
				return errors.Join(ErrRoutingInputInconsistent, ErrCatalogFreshnessMismatch)
			}
		}
	}
	if input.Priority < -1_000_000_000 || input.Priority > 1_000_000_000 {
		return errors.Join(ErrRoutingInputInconsistent, ErrInvalidRoutingPriority)
	}
	return nil
}
