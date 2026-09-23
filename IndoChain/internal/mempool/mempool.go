package mempool

import (
	"errors"
	"sort"
	"sync"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/transaction"
)

var (
	ErrFull      = errors.New("mempool is full")
	ErrDuplicate = errors.New("transaction already exists")
)

type Config struct {
	MaxTransactions int
}

type Pool struct {
	mu  sync.RWMutex
	cfg Config
	txs map[string]transaction.Transaction
}

func New(cfg Config) *Pool {
	if cfg.MaxTransactions <= 0 {
		cfg.MaxTransactions = 10000
	}
	return &Pool{cfg: cfg, txs: make(map[string]transaction.Transaction)}
}

func (p *Pool) Add(tx transaction.Transaction) error {
	hash := transaction.Hash(tx).String()

	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.txs[hash]; exists {
		return ErrDuplicate
	}
	if len(p.txs) >= p.cfg.MaxTransactions {
		return ErrFull
	}
	p.txs[hash] = tx
	return nil
}

func (p *Pool) Get(hash string) (transaction.Transaction, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	tx, ok := p.txs[hash]
	return tx, ok
}

func (p *Pool) Remove(hash string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.txs[hash]; !ok {
		return false
	}
	delete(p.txs, hash)
	return true
}

func (p *Pool) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.txs)
}

// Snapshot returns the current transactions without promising an iteration order.
// Callers that construct a block must use SnapshotSorted or another explicit
// protocol ordering rule before committing transactions to a block.
func (p *Pool) Snapshot() []transaction.Transaction {
	p.mu.RLock()
	defer p.mu.RUnlock()

	out := make([]transaction.Transaction, 0, len(p.txs))
	for _, tx := range p.txs {
		out = append(out, tx)
	}
	return out
}

// SnapshotSorted returns a deterministic transaction snapshot ordered by the
// transaction hash bytes. This is a development ordering primitive only; it
// does not freeze the final block transaction-selection or fee-priority policy.
func (p *Pool) SnapshotSorted() []transaction.Transaction {
	p.mu.RLock()
	defer p.mu.RUnlock()

	type entry struct {
		hash string
		tx   transaction.Transaction
	}
	entries := make([]entry, 0, len(p.txs))
	for hash, tx := range p.txs {
		entries = append(entries, entry{hash: hash, tx: tx})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].hash < entries[j].hash
	})

	out := make([]transaction.Transaction, len(entries))
	for i, entry := range entries {
		out[i] = entry.tx
	}
	return out
}
