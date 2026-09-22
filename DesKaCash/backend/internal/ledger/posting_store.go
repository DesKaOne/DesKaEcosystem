package ledger

import "context"

// CreatePosting stores a posting exactly as supplied.
// Posting IDs are immutable identifiers; corrections must be represented by new transactions.
func (r *MemoryRepository) CreatePosting(ctx context.Context, posting Posting) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if posting.ID == "" || posting.TransactionID == "" || posting.AccountID == "" ||
		posting.Asset == "" || posting.Amount.BaseUnits <= 0 {
		return ErrInvalidPosting
	}
	if _, exists := r.postings[posting.ID]; exists {
		return ErrDuplicatePosting
	}
	r.postings[posting.ID] = posting
	return nil
}

// ListPostings returns immutable postings for a transaction in insertion order.
func (r *MemoryRepository) ListPostings(ctx context.Context, transactionID string) ([]Posting, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ids := r.postingIDs[transactionID]
	result := make([]Posting, 0, len(ids))
	for _, id := range ids {
		result = append(result, r.postings[id])
	}
	return result, nil
}
