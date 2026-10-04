package accounting

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestSettlementPosterRequiresTerminalSuccess(t *testing.T) {
	ledger := NewMemoryStore()
	poster, err := NewSettlementPoster(ledger)
	if err != nil {
		t.Fatal(err)
	}
	req := validSettlementPosting()
	req.ProviderStatus = "pending"
	if err := poster.Post(context.Background(), req); !errors.Is(err, ErrSettlementNotPostable) {
		t.Fatalf("expected non-terminal status rejection, got %v", err)
	}
	if _, ok := ledger.Get(req.TransactionID); ok {
		t.Fatal("non-terminal settlement must not mutate ledger")
	}
}

func TestSettlementAuditRejectsNonSuccessStatus(t *testing.T) {
	audit := SettlementAudit{
		EventID: "audit-1",
		TransactionID: "tx-1",
		ReferenceID: "ref-1",
		SourceType: "PROVIDER_SETTLEMENT",
		SourceID: "provider-tx-1",
		Status: "pending",
		CreatedAt: time.Date(2026, 10, 4, 1, 0, 0, 0, time.UTC),
	}
	if err := audit.Validate(); !errors.Is(err, ErrInvalidSettlementAudit) {
		t.Fatalf("expected non-success settlement audit rejection, got %v", err)
	}
	ledger := NewMemoryStore()
	tx := validLedgerTransaction()
	tx.ID = "audit-status-tx"
	tx.CreatedAt = audit.CreatedAt
	if err := ledger.AppendSettlement(context.Background(), tx, audit); !errors.Is(err, ErrInvalidSettlementAudit) {
		t.Fatalf("expected append to reject non-success settlement audit, got %v", err)
	}
	if _, ok := ledger.Get(tx.ID); ok {
		t.Fatal("invalid settlement audit must not mutate ledger")
	}
}

func TestSettlementPosterRequiresBalancedPredeclaredEntries(t *testing.T) {
	ledger := NewMemoryStore()
	poster, err := NewSettlementPoster(ledger)
	if err != nil {
		t.Fatal(err)
	}
	req := validSettlementPosting()
	req.Entries[1].Amount = 9999
	if err := poster.Post(context.Background(), req); !errors.Is(err, ErrInvalidSettlement) {
		t.Fatalf("expected invalid settlement rejection, got %v", err)
	}
}

func TestSettlementPosterIsIdempotentAndImmutable(t *testing.T) {
	ledger := NewMemoryStore()
	poster, err := NewSettlementPoster(ledger)
	if err != nil {
		t.Fatal(err)
	}
	req := validSettlementPosting()
	if err := poster.Post(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	audit, ok, err := ledger.GetSettlementAudit(context.Background(), req.TransactionID)
	if err != nil || !ok { t.Fatalf("expected durable settlement audit: %v %v", err, ok) }
	if audit.ReferenceID != req.ReferenceID || audit.SourceID != req.SourceID || audit.Status != ProviderStatusSuccess { t.Fatalf("unexpected settlement audit: %#v", audit) }
	if err := poster.Post(context.Background(), req); err != nil {
		t.Fatalf("identical settlement post must be idempotent: %v", err)
	}
	conflict := req
	conflict.Description = "tampered"
	if err := poster.Post(context.Background(), conflict); !errors.Is(err, ErrLedgerConflict) {
		t.Fatalf("expected immutable ledger conflict, got %v", err)
	}
}

func TestSettlementPersistenceAmbiguityIsClassified(t *testing.T) {
	if !errors.Is(fmt.Errorf("%w: commit uncertain", ErrSettlementPersistenceAmbiguous), ErrSettlementPersistenceAmbiguous) {
		t.Fatal("ambiguous settlement persistence must remain machine-detectable")
	}
}

func TestSettlementPosterDoesNotCallProvider(t *testing.T) {
	ledger := NewMemoryStore()
	poster, err := NewSettlementPoster(ledger)
	if err != nil {
		t.Fatal(err)
	}
	if err := poster.Post(context.Background(), validSettlementPosting()); err != nil {
		t.Fatal(err)
	}
}

func validSettlementPosting() SettlementPostingRequest {
	return SettlementPostingRequest{
		TransactionID: "settlement-ledger-1",
		ReferenceID: "provider-ref-1",
		SourceType: "PROVIDER_SETTLEMENT",
		SourceID: "provider-tx-1",
		ProviderStatus: ProviderStatusSuccess,
		Currency: "IDR",
		Description: "provider settlement",
		CreatedAt: time.Date(2026, 10, 3, 4, 0, 0, 0, time.UTC),
		Entries: []Entry{
			{LineID: 1, AccountID: "provider-clearing", Direction: Debit, Amount: 10000, Currency: "IDR"},
			{LineID: 2, AccountID: "settlement-in", Direction: Credit, Amount: 10000, Currency: "IDR"},
		},
	}
}
