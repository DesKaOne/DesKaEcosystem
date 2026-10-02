package accounting

import (
	"context"
	"errors"
	"fmt"
)

const ProviderStatusSuccess = "success"

var (
	ErrSettlementNotPostable = errors.New("provider transaction is not eligible for settlement posting")
	ErrInvalidSettlement     = errors.New("invalid settlement posting")
)

type SettlementPostingRequest struct {
	TransactionID string
	ReferenceID   string
	SourceType    string
	SourceID      string
	ProviderStatus string
	Currency      string
	Description   string
	Entries       []Entry
}

func (r SettlementPostingRequest) Validate() error {
	if r.TransactionID == "" || r.ReferenceID == "" || r.SourceType == "" || r.SourceID == "" ||
		r.Currency == "" || len(r.Entries) == 0 {
		return ErrInvalidSettlement
	}
	if r.ProviderStatus != ProviderStatusSuccess {
		return ErrSettlementNotPostable
	}
	tx := LedgerTransaction{
		ID: r.TransactionID, ReferenceID: r.ReferenceID, SourceType: r.SourceType,
		SourceID: r.SourceID, Currency: r.Currency, Description: r.Description,
		Entries: r.Entries,
	}
	if err := tx.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSettlement, err)
	}
	return nil
}

type ContextLedgerStore interface {
	Append(context.Context, LedgerTransaction) error
	Get(context.Context, string) (LedgerTransaction, bool, error)
}

type SettlementPoster struct {
	ledger ContextLedgerStore
}

func NewSettlementPoster(ledger ContextLedgerStore) (*SettlementPoster, error) {
	if ledger == nil {
		return nil, errors.New("settlement ledger store is required")
	}
	return &SettlementPoster{ledger: ledger}, nil
}

func (p *SettlementPoster) Post(ctx context.Context, req SettlementPostingRequest) error {
	if err := req.Validate(); err != nil {
		return err
	}
	tx := LedgerTransaction{
		ID: req.TransactionID, ReferenceID: req.ReferenceID,
		SourceType: req.SourceType, SourceID: req.SourceID,
		Currency: req.Currency, Description: req.Description,
		Entries: append([]Entry(nil), req.Entries...),
	}
	// Append owns idempotency and immutable identity. This method deliberately
	// performs no provider retry, failover, resubmission, funding, or balance mutation.
	return p.ledger.Append(ctx, tx)
}
