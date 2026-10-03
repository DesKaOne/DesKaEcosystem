package accounting

import (
	"errors"
	"testing"
	"time"
)

func TestValidatePersistedLedgerTransactionRejectsPartialRows(t *testing.T) {
	tx := validLedgerTransaction()
	tx.CreatedAt = time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	tx.Entries = tx.Entries[:1]

	err := validatePersistedLedgerTransaction(tx)
	if err == nil {
		t.Fatal("expected partial persisted ledger transaction to be rejected")
	}
	if !errors.Is(err, ErrInvalidLedgerTransaction) {
		t.Fatalf("expected invalid ledger transaction classification, got %v", err)
	}
}

func TestValidatePersistedLedgerTransactionAcceptsCompleteRows(t *testing.T) {
	tx := validLedgerTransaction()
	tx.CreatedAt = time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)

	if err := validatePersistedLedgerTransaction(tx); err != nil {
		t.Fatalf("expected complete persisted ledger transaction to validate: %v", err)
	}
}

func TestValidatePersistedSettlementAuditRejectsIncompleteRows(t *testing.T) {
	audit := SettlementAudit{
		EventID:       "event-1",
		TransactionID: "tx-1",
		ReferenceID:   "ref-1",
		SourceType:    "purchase",
		SourceID:      "source-1",
		Status:        "",
		CreatedAt:     time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC),
	}

	err := validatePersistedSettlementAudit(audit)
	if err == nil {
		t.Fatal("expected incomplete persisted settlement audit to be rejected")
	}
	if !errors.Is(err, errors.New("invalid settlement audit")) {
		t.Fatalf("expected settlement audit validation failure, got %v", err)
	}
}

func TestValidatePersistedSettlementAuditAcceptsCompleteRows(t *testing.T) {
	audit := SettlementAudit{
		EventID:       "event-1",
		TransactionID: "tx-1",
		ReferenceID:   "ref-1",
		SourceType:    "purchase",
		SourceID:      "source-1",
		Status:        ProviderStatusSuccess,
		CreatedAt:     time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC),
	}

	if err := validatePersistedSettlementAudit(audit); err != nil {
		t.Fatalf("expected complete persisted settlement audit to validate: %v", err)
	}
}
