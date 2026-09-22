package ledger

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

func (r *MemoryRepository) GetAccount(id string) (Account, error) {
	account, ok := r.accounts[id]
	if !ok {
		return Account{}, ErrNotFound
	}
	return account, nil
}

func (r *MemoryRepository) SaveAccount(account Account) error {
	r.accounts[account.ID] = account
	return nil
}

func (r *MemoryRepository) GetTransaction(id string) (Transaction, error) {
	tx, ok := r.transactions[id]
	if !ok {
		return Transaction{}, ErrNotFound
	}
	return tx, nil
}

func (r *MemoryRepository) CreateTransaction(tx Transaction) error {
	if _, exists := r.transactions[tx.ID]; exists {
		return ErrDuplicateTransaction
	}
	r.transactions[tx.ID] = tx
	return nil
}

func (r *MemoryRepository) CreateEntry(entry Entry) error {
	r.entries[entry.AccountID] = append(r.entries[entry.AccountID], entry)
	return nil
}

func (r *MemoryRepository) ListEntries(accountID string) ([]Entry, error) {
	entries := r.entries[accountID]
	result := make([]Entry, len(entries))
	copy(result, entries)
	return result, nil
}
