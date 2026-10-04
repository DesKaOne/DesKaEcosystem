package operational

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidOperationalSnapshot = errors.New("invalid provider operational snapshot")
	ErrOperationalTimestampOrder = errors.New("operational timestamps are inconsistent")
	ErrOperationalHealthState     = errors.New("operational health state is inconsistent")
	ErrOperationalFailureCount    = errors.New("operational failure count is inconsistent")
)

// ValidateSnapshot enforces provider-neutral consistency for persisted
// operational observations. It is observational state only and never
// authorizes routing or financial activity.
func ValidateSnapshot(snapshot Snapshot) error {
	if snapshot.ProviderName == "" {
		return fmt.Errorf("%w: provider name is required", ErrInvalidOperationalSnapshot)
	}
	if snapshot.Currency == "" {
		return fmt.Errorf("%w: currency is required", ErrInvalidOperationalSnapshot)
	}
	if snapshot.LastCheckedAt.IsZero() {
		return fmt.Errorf("%w: last checked timestamp is required", ErrInvalidOperationalSnapshot)
	}
	if !snapshot.LastSuccessAt.IsZero() && snapshot.LastSuccessAt.After(snapshot.LastCheckedAt) {
		return fmt.Errorf("%w: %w", ErrInvalidOperationalSnapshot, ErrOperationalTimestampOrder)
	}
	if snapshot.ConsecutiveFailures < 0 {
		return fmt.Errorf("%w: %w", ErrInvalidOperationalSnapshot, ErrOperationalFailureCount)
	}

	switch snapshot.Health {
	case HealthUnknown:
		if snapshot.ConsecutiveFailures != 0 || snapshot.LastError != "" {
			return fmt.Errorf("%w: unknown health cannot carry failure state", ErrOperationalHealthState)
		}
	case HealthHealthy:
		if snapshot.ConsecutiveFailures != 0 || snapshot.LastError != "" {
			return fmt.Errorf("%w: healthy health cannot carry failure state", ErrOperationalHealthState)
		}
		if snapshot.LastSuccessAt.IsZero() || !snapshot.LastSuccessAt.Equal(snapshot.LastCheckedAt) {
			return fmt.Errorf("%w: healthy health requires last success to equal last checked", ErrOperationalTimestampOrder)
		}
	case HealthDegraded, HealthUnhealthy:
		if snapshot.ConsecutiveFailures < 1 || snapshot.LastError == "" {
			return fmt.Errorf("%w: degraded or unhealthy health requires failure evidence", ErrOperationalHealthState)
		}
	default:
		return fmt.Errorf("%w: unsupported health %q", ErrInvalidOperationalSnapshot, snapshot.Health)
	}
	return nil
}
