package routing

import (
	"testing"
	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

func TestClassifyPersistenceOutcome(t *testing.T) {
	expected := postgresPendingState()
	applied := expected
	applied.Execution.Result.Status = provider.StatusSuccess

	tests := []struct{
		name string
		found bool
		current TransactionState
		want PersistenceOutcome
	}{
		{"missing", false, TransactionState{}, PersistenceOutcomeNotApplied},
		{"same durable result", true, applied, PersistenceOutcomeApplied},
		{"different durable result", true, expected, PersistenceOutcomeConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyPersistenceOutcome(tt.current, tt.found, applied); got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}
