package accounting

import "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/routing"

// reconciliationEconomicAgreement requires a successful payment provider state
// to agree with the persisted ledger's economically material settlement facts.
// A mismatch is observationally reported as a correlation conflict; it never
// authorizes repair, reversal, or resubmission.
func reconciliationEconomicAgreement(state routing.TransactionState, tx LedgerTransaction) bool {
	if state.Kind != routing.TransactionKindPayment || state.Payment == nil {
		return true
	}
	if tx.Currency != state.Payment.Currency {
		return false
	}

	var debit, credit int64
	for _, entry := range tx.Entries {
		switch entry.Direction {
		case Debit:
			var err error
			debit, err = checkedAmountAdd(debit, entry.Amount)
			if err != nil {
				return false
			}
		case Credit:
			var err error
			credit, err = checkedAmountAdd(credit, entry.Amount)
			if err != nil {
				return false
			}
		default:
			return false
		}
	}
	return debit == credit && debit == state.Payment.Amount
}
