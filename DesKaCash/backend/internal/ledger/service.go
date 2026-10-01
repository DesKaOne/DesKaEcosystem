package ledger

import (
	"context"
	"errors"
)

var (
	ErrDuplicateAccount     = errors.New("duplicate account")
	ErrDuplicateTransaction = errors.New("duplicate transaction")
	ErrInvalidAmount        = errors.New("invalid amount")
)

// Service applies ledger business rules through a persistence repository.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) RegisterAccount(ctx context.Context, account Account) error {
	if account.ID == "" || account.UserID == "" || !IsSupportedAsset(account.Asset) {
		return ErrInvalidAccount
	}
	return s.repo.CreateAccount(ctx, account)
}

func (s *Service) GetAccount(ctx context.Context, id string) (Account, error) {
	return s.repo.GetAccount(ctx, id)
}

func (s *Service) ApplyCredit(ctx context.Context, tx Transaction, reference string) error {
	if tx.Amount.BaseUnits <= 0 {
		return ErrInvalidAmount
	}
	if !IsSupportedAsset(tx.Asset) {
		return ErrInvalidAsset
	}
	return s.repo.ApplyCredit(ctx, tx, reference)
}

func (s *Service) ApplyDebit(ctx context.Context, tx Transaction, reference string) error {
	if tx.Amount.BaseUnits <= 0 {
		return ErrInvalidAmount
	}
	if !IsSupportedAsset(tx.Asset) {
		return ErrInvalidAsset
	}
	return s.repo.ApplyDebit(ctx, tx, reference)
}

func (s *Service) Entries(ctx context.Context, accountID string) ([]Entry, error) {
	return s.repo.ListEntries(ctx, accountID)
}
