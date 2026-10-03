package accounting

import "fmt"

func validatePersistedLedgerTransaction(tx LedgerTransaction) error {
	if err := tx.Validate(); err != nil {
		return fmt.Errorf("persisted ledger transaction is invalid: %w", err)
	}
	return nil
}

func validatePersistedSettlementAudit(audit SettlementAudit) error {
	if err := audit.Validate(); err != nil {
		return fmt.Errorf("persisted settlement audit is invalid: %w", err)
	}
	return nil
}
