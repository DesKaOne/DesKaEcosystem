package ledger

import "testing"

func TestNewPosting(t *testing.T) {
	posting, err := NewPosting(
		"posting-1",
		"tx-1",
		"acct-1",
		AssetDIDR,
		PostingCredit,
		Money{BaseUnits: 1000},
		"topup",
	)
	if err != nil {
		t.Fatal(err)
	}
	if posting.Type != PostingCredit {
		t.Fatalf("expected credit posting, got %s", posting.Type)
	}
	if posting.Amount.BaseUnits != 1000 {
		t.Fatalf("expected 1000 base units, got %d", posting.Amount.BaseUnits)
	}
}

func TestNewPostingRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		posting Posting
	}{
		{name: "missing id", posting: Posting{TransactionID: "tx-1", AccountID: "acct-1", Asset: AssetDIDR, Type: PostingCredit, Amount: Money{BaseUnits: 1}}},
		{name: "missing transaction", posting: Posting{ID: "p-1", AccountID: "acct-1", Asset: AssetDIDR, Type: PostingCredit, Amount: Money{BaseUnits: 1}}},
		{name: "missing account", posting: Posting{ID: "p-1", TransactionID: "tx-1", Asset: AssetDIDR, Type: PostingCredit, Amount: Money{BaseUnits: 1}}},
		{name: "missing asset", posting: Posting{ID: "p-1", TransactionID: "tx-1", AccountID: "acct-1", Type: PostingCredit, Amount: Money{BaseUnits: 1}}},
		{name: "zero amount", posting: Posting{ID: "p-1", TransactionID: "tx-1", AccountID: "acct-1", Asset: AssetDIDR, Type: PostingCredit}},
		{name: "negative amount", posting: Posting{ID: "p-1", TransactionID: "tx-1", AccountID: "acct-1", Asset: AssetDIDR, Type: PostingCredit, Amount: Money{BaseUnits: -1}}},
		{name: "invalid type", posting: Posting{ID: "p-1", TransactionID: "tx-1", AccountID: "acct-1", Asset: AssetDIDR, Amount: Money{BaseUnits: 1}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewPosting(tc.posting.ID, tc.posting.TransactionID, tc.posting.AccountID, tc.posting.Asset, tc.posting.Type, tc.posting.Amount, tc.posting.Reference); err != ErrInvalidPosting {
				t.Fatalf("expected ErrInvalidPosting, got %v", err)
			}
		})
	}
}


func TestValidateBalancedPostings(t *testing.T) {
	postings := []Posting{
		{
			ID: "debit-1", TransactionID: "tx-balanced", AccountID: "source",
			Asset: AssetIDR, Type: PostingDebit, Amount: Money{BaseUnits: 10_000},
		},
		{
			ID: "credit-1", TransactionID: "tx-balanced", AccountID: "destination",
			Asset: AssetIDR, Type: PostingCredit, Amount: Money{BaseUnits: 10_000},
		},
	}
	if err := ValidateBalancedPostings(postings); err != nil {
		t.Fatalf("expected balanced postings, got %v", err)
	}
}

func TestValidateBalancedPostingsRejectsUnbalancedSets(t *testing.T) {
	tests := []struct {
		name     string
		postings []Posting
	}{
		{
			name: "missing debit",
			postings: []Posting{
				{ID: "credit-1", TransactionID: "tx-1", AccountID: "account", Asset: AssetIDR, Type: PostingCredit, Amount: Money{BaseUnits: 100}},
			},
		},
		{
			name: "missing credit",
			postings: []Posting{
				{ID: "debit-1", TransactionID: "tx-1", AccountID: "account", Asset: AssetIDR, Type: PostingDebit, Amount: Money{BaseUnits: 100}},
				{ID: "debit-2", TransactionID: "tx-1", AccountID: "account-2", Asset: AssetIDR, Type: PostingDebit, Amount: Money{BaseUnits: 100}},
			},
		},
		{
			name: "different totals",
			postings: []Posting{
				{ID: "debit-1", TransactionID: "tx-1", AccountID: "source", Asset: AssetIDR, Type: PostingDebit, Amount: Money{BaseUnits: 100}},
				{ID: "credit-1", TransactionID: "tx-1", AccountID: "destination", Asset: AssetIDR, Type: PostingCredit, Amount: Money{BaseUnits: 90}},
			},
		},
		{
			name: "different assets",
			postings: []Posting{
				{ID: "debit-1", TransactionID: "tx-1", AccountID: "source", Asset: AssetIDR, Type: PostingDebit, Amount: Money{BaseUnits: 100}},
				{ID: "credit-1", TransactionID: "tx-1", AccountID: "destination", Asset: AssetDIDR, Type: PostingCredit, Amount: Money{BaseUnits: 100}},
			},
		},
		{
			name: "different transactions",
			postings: []Posting{
				{ID: "debit-1", TransactionID: "tx-1", AccountID: "source", Asset: AssetIDR, Type: PostingDebit, Amount: Money{BaseUnits: 100}},
				{ID: "credit-1", TransactionID: "tx-2", AccountID: "destination", Asset: AssetIDR, Type: PostingCredit, Amount: Money{BaseUnits: 100}},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateBalancedPostings(tc.postings); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestValidateBalancedPostingsRejectsInvalidPosting(t *testing.T) {
	postings := []Posting{
		{ID: "debit-1", TransactionID: "tx-1", AccountID: "source", Asset: AssetIDR, Type: PostingDebit, Amount: Money{BaseUnits: 0}},
		{ID: "credit-1", TransactionID: "tx-1", AccountID: "destination", Asset: AssetIDR, Type: PostingCredit, Amount: Money{BaseUnits: 0}},
	}
	if err := ValidateBalancedPostings(postings); err != ErrInvalidPosting {
		t.Fatalf("expected ErrInvalidPosting, got %v", err)
	}
}
