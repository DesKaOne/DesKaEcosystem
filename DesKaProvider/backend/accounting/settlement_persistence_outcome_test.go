package accounting

import "testing"

func TestClassifySettlementPersistenceOutcome(t *testing.T) {
	tests := []struct{
		name string
		ledgerFound bool
		auditFound bool
		ledgerMatches bool
		auditMatches bool
		want SettlementPersistenceOutcome
	}{
		{"fully applied", true, true, true, true, SettlementPersistenceApplied},
		{"fully absent", false, false, false, false, SettlementPersistenceNotApplied},
		{"ledger only", true, false, true, false, SettlementPersistenceConflict},
		{"audit only", false, true, false, true, SettlementPersistenceConflict},
		{"identity conflict", true, true, false, true, SettlementPersistenceConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifySettlementPersistenceOutcome(tt.ledgerFound, tt.auditFound, tt.ledgerMatches, tt.auditMatches); got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}
