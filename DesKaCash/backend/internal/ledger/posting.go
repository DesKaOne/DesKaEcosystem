package ledger

import (
	"errors"
	"math"
)

type PostingType string

const (
	PostingDebit  PostingType = "debit"
	PostingCredit PostingType = "credit"
)

var (
	ErrInvalidPosting = errors.New("invalid ledger posting")
	ErrUnbalancedPostings = errors.New("unbalanced ledger postings")
)

// Posting is an immutable financial effect attached to a ledger transaction.
// A balanced transaction must contain at least one debit and one credit posting.
type Posting struct {
	ID            string
	TransactionID string
	AccountID     string
	Asset         string
	Type          PostingType
	Amount        Money
	Reference     string
}

func NewPosting(id, transactionID, accountID, asset string, postingType PostingType, amount Money, reference string) (Posting, error) {
	if id == "" || transactionID == "" || accountID == "" || asset == "" || amount.BaseUnits <= 0 {
		return Posting{}, ErrInvalidPosting
	}
	switch postingType {
	case PostingDebit, PostingCredit:
	default:
		return Posting{}, ErrInvalidPosting
	}
	return Posting{
		ID: id,
		TransactionID: transactionID,
		AccountID: accountID,
		Asset: asset,
		Type: postingType,
		Amount: amount,
		Reference: reference,
	}, nil
}

// ValidateBalancedPostings verifies that a transaction has a debit and credit
// side with equal totals in the same asset. Postings are immutable once stored,
// so an unbalanced set must be rejected before persistence.
func ValidateBalancedPostings(postings []Posting) error {
	if len(postings) < 2 {
		return ErrUnbalancedPostings
	}

	transactionID := postings[0].TransactionID
	asset := postings[0].Asset
	var debits, credits int64

	for _, posting := range postings {
		if posting.ID == "" || posting.TransactionID == "" || posting.AccountID == "" ||
			posting.Asset == "" || posting.Amount.BaseUnits <= 0 {
			return ErrInvalidPosting
		}
		if posting.TransactionID != transactionID || posting.Asset != asset {
			return ErrUnbalancedPostings
		}

		switch posting.Type {
		case PostingDebit:
			if debits > math.MaxInt64-posting.Amount.BaseUnits {
				return ErrUnbalancedPostings
			}
			debits += posting.Amount.BaseUnits
		case PostingCredit:
			if credits > math.MaxInt64-posting.Amount.BaseUnits {
				return ErrUnbalancedPostings
			}
			credits += posting.Amount.BaseUnits
		default:
			return ErrInvalidPosting
		}
	}

	if debits == 0 || credits == 0 || debits != credits {
		return ErrUnbalancedPostings
	}
	return nil
}
