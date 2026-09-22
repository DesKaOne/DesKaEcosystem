package ledger

import "context"

// MemoryRepository is a test/development repository implementation.
// Production persistence will be provided by PostgreSQL.
type MemoryRepository struct {
	accounts     map[string]Account
	transactions map[string]Transaction
	entries      map[string][]Entry
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		accounts:     make(map[string]Account),
		transactions: make(map[string]Transaction),
		entries:      make(map[string][]Entry),
	}
}

func (r *MemoryRepository) GetAccount(ctx context.Context, id string) (Account, error) {
	if err := ctx.Err(); err != nil {
		return Account{}, err
	}
	account, ok := r.accounts[id]
	if !ok {
		return Account{}, ErrNotFound
	}
	return account, nil
}

func (r *MemoryRepository) CreateAccount(ctx context.Context, account Account) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, exists := r.accounts[account.ID]; exists {
		return ErrInvalidAccount
	}
	r.accounts[account.ID] = account
	return nil
}

func (r *MemoryRepository) SaveAccount(ctx context.Context, account Account) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.accounts[account.ID] = account
	return nil
}

func (r *MemoryRepository) GetTransaction(ctx context.Context, id string) (Transaction, error) {
	if err := ctx.Err(); err != nil {
		return Transaction{}, err
	}
	tx, ok := r.transactions[id]
	if !ok {
		return Transaction{}, ErrNotFound
	}
	return tx, nil
}

func (r *MemoryRepository) CreateTransaction(ctx context.Context, tx Transaction) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, exists := r.transactions[tx.ID]; exists {
		return ErrDuplicateTransaction
	}
	r.transactions[tx.ID] = tx
	return nil
}

func (r *MemoryRepository) CreateEntry(ctx context.Context, entry Entry) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.entries[entry.AccountID] = append(r.entries[entry.AccountID], entry)
	return nil
}

func (r *MemoryRepository) ListEntries(ctx context.Context, accountID string) ([]Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries := r.entries[accountID]
	result := make([]Entry, len(entries))
	copy(result, entries)
	return result, nil
}
