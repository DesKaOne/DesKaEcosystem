package accounting

// duplicateAuditIndexes builds the cross-table identity conflict indexes used
// by reconciliation. A settlement audit is evidence only when both its
// transaction identity and event identity are unique in the captured snapshot.
func duplicateAuditIndexes(audits []SettlementAudit) (map[string]bool, map[string]bool) {
	byTransaction := make(map[string]int, len(audits))
	byEvent := make(map[string]int, len(audits))
	for _, audit := range audits {
		byTransaction[audit.TransactionID]++
		byEvent[audit.EventID]++
	}
	duplicateTransactions := make(map[string]bool)
	duplicateEvents := make(map[string]bool)
	for id, count := range byTransaction {
		if count > 1 {
			duplicateTransactions[id] = true
		}
	}
	for id, count := range byEvent {
		if count > 1 {
			duplicateEvents[id] = true
		}
	}
	return duplicateTransactions, duplicateEvents
}
