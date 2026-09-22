package ledger

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryRepositoryStoresImmutablePostings(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()

	credit, err := NewPosting("p-credit", "tx-1", "acct-1", AssetDIDR, PostingCredit, Money{BaseUnits: 1000}, "topup")
	if err != nil {
		t.Fatal(err)
	}
	debit, err := NewPosting("p-debit", "tx-1", "acct-2", AssetDIDR, PostingDebit, Money{BaseUnits: 1000}, "topup")
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.CreatePosting(ctx, credit); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreatePosting(ctx, debit); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreatePosting(ctx, credit); !errors.Is(err, ErrDuplicatePosting) {
		t.Fatalf("expected duplicate posting error, got %v", err)
	}

	postings, err := repo.ListPostings(ctx, "tx-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(postings) != 2 {
		t.Fatalf("expected 2 postings, got %d", len(postings))
	}
	if postings[0].ID != credit.ID || postings[1].ID != debit.ID {
		t.Fatalf("unexpected posting order: %#v", postings)
	}
}
