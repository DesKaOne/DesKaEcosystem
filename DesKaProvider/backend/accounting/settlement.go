package accounting

import (
	"context"
	"errors"
	"fmt"
	"time"
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
	CreatedAt     time.Time
	Entries       []Entry
}

func (r SettlementPostingRequest) Validate() error {
	if r.TransactionID == "" || r.ReferenceID == "" || r.SourceType == "" || r.SourceID == "" ||
		r.Currency == "" || r.CreatedAt.IsZero() || len(r.Entries) == 0 {
		return ErrInvalidSettlement
	}
	if r.ProviderStatus != ProviderStatusSuccess {
		return ErrSettlementNotPostable
	}
	tx := LedgerTransaction{
		ID: r.TransactionID, ReferenceID: r.ReferenceID, SourceType: r.SourceType,
		SourceID: r.SourceID, Currency: r.Currency, Description: r.Description,
		CreatedAt: r.CreatedAt,
		Entries: r.Entries,
	}
	if err := tx.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSettlement, err)
	}
	return nil
}


type SettlementPersistenceOutcome string

const (
	SettlementPersistenceApplied    SettlementPersistenceOutcome = "applied"
	SettlementPersistenceNotApplied SettlementPersistenceOutcome = "not_applied"
	SettlementPersistenceConflict   SettlementPersistenceOutcome = "conflict"
	SettlementPersistenceUnknown    SettlementPersistenceOutcome = "unknown"
)

func classifySettlementPersistenceOutcome(ledgerFound bool, auditFound bool, ledgerMatches bool, auditMatches bool) SettlementPersistenceOutcome {
	if ledgerFound && auditFound && ledgerMatches && auditMatches {
		return SettlementPersistenceApplied
	}
	if !ledgerFound && !auditFound {
		return SettlementPersistenceNotApplied
	}
	return SettlementPersistenceConflict
}

type SettlementPersistenceOutcomeReader interface {
	ResolveSettlementPersistenceOutcome(ctx context.Context, ledger LedgerTransaction, audit SettlementAudit) (SettlementPersistenceOutcome, error)
}

type SettlementPoster struct {
	ledger SettlementStore
}

func NewSettlementPoster(ledger SettlementStore) (*SettlementPoster, error) {
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
		Currency: req.Currency, Description: req.Description, CreatedAt: req.CreatedAt,
		Entries: append([]Entry(nil), req.Entries...),
	}
	audit := SettlementAudit{
		EventID: "settlement-audit:" + req.TransactionID,
		TransactionID: tx.ID,
		ReferenceID: tx.ReferenceID,
		SourceType: tx.SourceType,
		SourceID: tx.SourceID,
		Status: ProviderStatusSuccess,
		CreatedAt: tx.CreatedAt,
	}
	// Ledger and audit are persisted through one settlement boundary. This method deliberately
	// performs no provider retry, failover, resubmission, funding, or balance mutation.
	err := p.ledger.AppendSettlement(ctx, tx, audit)
	if err == nil {
		return nil
	}
	if !errors.Is(err, ErrSettlementPersistenceAmbiguous) {
		return err
	}
	reader, ok := p.ledger.(SettlementPersistenceOutcomeReader)
	if !ok {
		return err
	}
	outcome, resolveErr := reader.ResolveSettlementPersistenceOutcome(ctx, tx, audit)
	if resolveErr != nil {
		return fmt.Errorf("%w; durable outcome resolution failed: %v", err, resolveErr)
	}
	if outcome == SettlementPersistenceApplied {
		return nil
	}
	// not_applied, conflict, and unknown remain unresolved. No settlement retry
	// or repair is attempted because the commit boundary is intentionally not
	// promoted into a speculative second write.
	return err
}
