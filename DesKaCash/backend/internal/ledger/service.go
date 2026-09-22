package ledger

import "errors"

var (
	ErrDuplicateTransaction = errors.New("duplicate transaction")
	ErrInvalidAmount         = errors.New("invalid amount")
)

// Service applies ledger mutations while keeping transaction identity separate
// from provider and IndoChain identifiers.
type Service struct {
	accounts     map[string]*Account
	transactions map[string]Transaction
	entries      map[string][]Entry
}

func NewService() *Service {
	return &Service{
		accounts:     make(map[string]*Account),
		transactions: make(map[string]Transaction),
		entries:      make(map[string][]Entry),
	}
}

func (s *Service) RegisterAccount(account Account) error {
	if account.ID == "" || account.UserID == "" || account.Asset != AssetDIDR {
		return ErrInvalidAccount
	}
	if _, exists := s.accounts[account.ID]; exists {
		return ErrInvalidAccount
	}

	copy := account
	s.accounts[account.ID] = &copy
	return nil
}

func (s *Service) GetAccount(id string) (Account, bool) {
	account, ok := s.accounts[id]
	if !ok {
		return Account{}, false
	}
	return *account, true
}

func (s *Service) ApplyCredit(tx Transaction, reference string) error {
	if tx.Amount.BaseUnits <= 0 {
		return ErrInvalidAmount
	}
	if _, exists := s.transactions[tx.ID]; exists {
		return ErrDuplicateTransaction
	}

	account, ok := s.accounts[tx.AccountID]
	if !ok {
		return ErrInvalidAccount
	}

	if err := account.Credit(tx.Amount); err != nil {
		return err
	}

	tx.Status = StatusSucceeded
	tx.Asset = AssetDIDR
	s.transactions[tx.ID] = tx
	s.entries[tx.AccountID] = append(s.entries[tx.AccountID], Entry{
		ID:            tx.ID + ":credit",
		AccountID:     tx.AccountID,
		TransactionID: tx.ID,
		Type:          EntryCredit,
		Asset:         AssetDIDR,
		Amount:        tx.Amount,
		Reference:     reference,
		CreatedAt:     tx.CreatedAt,
	})

	return nil
}

func (s *Service) Entries(accountID string) []Entry {
	entries := s.entries[accountID]
	result := make([]Entry, len(entries))
	copy(result, entries)
	return result
}
