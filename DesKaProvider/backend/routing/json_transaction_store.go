package routing

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "sync"
    "syscall"
)

type JSONFileTransactionStore struct {
    mu           sync.RWMutex
    path         string
    transactions map[string]TransactionState
}

type jsonTransactionState struct {
    Transactions map[string]TransactionState `json:"transactions"`
}

func NewJSONFileTransactionStore(path string) (*JSONFileTransactionStore, error) {
    if path == "" {
        return nil, errors.New("transaction store path is required")
    }
    store := &JSONFileTransactionStore{
        path:         path,
        transactions: make(map[string]TransactionState),
    }
    data, err := os.ReadFile(path)
    if errors.Is(err, os.ErrNotExist) {
        return store, nil
    }
    if err != nil {
        return nil, fmt.Errorf("read transaction store: %w", err)
    }
    if len(data) == 0 {
        return store, nil
    }
    var state jsonTransactionState
    if err := json.Unmarshal(data, &state); err != nil {
        return nil, fmt.Errorf("decode transaction store: %w", err)
    }
    if state.Transactions != nil {
        store.transactions = state.Transactions
    }
    return store, nil
}

func (s *JSONFileTransactionStore) CreateIfAbsentContext(ctx context.Context, state TransactionState) (TransactionState, bool, error) {
	if err := ctx.Err(); err != nil {
		return TransactionState{}, false, err
	}
	referenceID := transactionReferenceID(state)
	if referenceID == "" || state.Execution.ProviderName == "" {
		return TransactionState{}, false, ErrReferenceConflict
	}
	if err := validateTransactionState(state); err != nil {
		return TransactionState{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := s.acquireFileLock()
	if err != nil {
		return TransactionState{}, false, err
	}
	defer unlock()
	if err := s.reloadLocked(); err != nil {
		return TransactionState{}, false, err
	}
	if current, ok := s.transactions[referenceID]; ok {
		if !sameTransactionIdentity(current, state) {
			return TransactionState{}, false, ErrReferenceConflict
		}
		return current, false, nil
	}
	s.transactions[referenceID] = state
	if err := s.persistLocked(); err != nil {
		delete(s.transactions, referenceID)
		return TransactionState{}, false, err
	}
	return state, true, nil
}

func sameTransactionState(a, b TransactionState) bool {
	return transactionReferenceID(a) == transactionReferenceID(b) &&
		sameTransactionIdentity(a, b) &&
		a.Execution == b.Execution &&
		a.Version == b.Version
}

func (s *JSONFileTransactionStore) acquireFileLock() (func(), error) {
	lockPath := s.path + ".lock"
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open transaction store lock: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("lock transaction store: %w", err)
	}
	return func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
	}, nil
}

func (s *JSONFileTransactionStore) reloadLocked() error {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		s.transactions = make(map[string]TransactionState)
		return nil
	}
	if err != nil {
		return fmt.Errorf("read transaction store: %w", err)
	}
	if len(data) == 0 {
		s.transactions = make(map[string]TransactionState)
		return nil
	}
	var state jsonTransactionState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("decode transaction store: %w", err)
	}
	if state.Transactions == nil {
		s.transactions = make(map[string]TransactionState)
	} else {
		s.transactions = state.Transactions
	}
	return nil
}

func (s *JSONFileTransactionStore) Get(referenceID string) (TransactionState, bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    state, ok := s.transactions[referenceID]
    return state, ok
}

func (s *JSONFileTransactionStore) All() []TransactionState {
    s.mu.RLock()
    defer s.mu.RUnlock()
    references := make([]string, 0, len(s.transactions))
    for referenceID := range s.transactions {
        references = append(references, referenceID)
    }
    sort.Strings(references)
    result := make([]TransactionState, 0, len(references))
    for _, referenceID := range references {
        result = append(result, s.transactions[referenceID])
    }
    return result
}

func (s *JSONFileTransactionStore) Put(state TransactionState) error {
	referenceID := transactionReferenceID(state)
	if referenceID == "" {
		return errors.New("transaction reference ID is required")
	}
	if state.Execution.ProviderName == "" {
		return errors.New("transaction provider name is required")
	}
	if err := validateTransactionState(state); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := s.acquireFileLock()
	if err != nil {
		return err
	}
	defer unlock()
	if err := s.reloadLocked(); err != nil {
		return err
	}
	previous, existed := s.transactions[referenceID]
	if existed {
		if err := validateTransactionTransition(previous, state); err != nil {
			return err
		}
	}
	s.transactions[referenceID] = state
	if err := s.persistLocked(); err != nil {
		if existed {
			s.transactions[referenceID] = previous
		} else {
			delete(s.transactions, referenceID)
		}
		return err
	}
	return nil
}

func (s *JSONFileTransactionStore) PutIfCurrent(referenceID string, previous, next TransactionState) error {
	if referenceID == "" || transactionReferenceID(next) != referenceID || transactionReferenceID(previous) != referenceID {
		return ErrReferenceConflict
	}
	if err := validateTransactionState(next); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := s.acquireFileLock()
	if err != nil {
		return err
	}
	defer unlock()
	if err := s.reloadLocked(); err != nil {
		return err
	}
	current, ok := s.transactions[referenceID]
	if !ok || !sameTransactionState(current, previous) {
		return ErrTransactionStateConflict
	}
	if err := validateTransactionTransition(previous, next); err != nil {
		return err
	}
	s.transactions[referenceID] = next
	if err := s.persistLocked(); err != nil {
		s.transactions[referenceID] = current
		return err
	}
	return nil
}

func (s *JSONFileTransactionStore) persistLocked() error {
    state := jsonTransactionState{Transactions: s.transactions}
    data, err := json.MarshalIndent(state, "", "  ")
    if err != nil {
        return fmt.Errorf("encode transaction store: %w", err)
    }

    dir := filepath.Dir(s.path)
    if err := os.MkdirAll(dir, 0o750); err != nil {
        return fmt.Errorf("create transaction store directory: %w", err)
    }

    tmp, err := os.CreateTemp(dir, ".transaction-*.tmp")
    if err != nil {
        return fmt.Errorf("create transaction store temp file: %w", err)
    }
    tmpName := tmp.Name()
    cleanup := func() {
        _ = tmp.Close()
        _ = os.Remove(tmpName)
    }
    defer cleanup()

    if err := tmp.Chmod(0o600); err != nil {
        return fmt.Errorf("secure transaction store temp file: %w", err)
    }
    if _, err := tmp.Write(data); err != nil {
        return fmt.Errorf("write transaction store: %w", err)
    }
    if err := tmp.Sync(); err != nil {
        return fmt.Errorf("sync transaction store: %w", err)
    }
    if err := tmp.Close(); err != nil {
        return fmt.Errorf("close transaction store: %w", err)
    }
    if err := os.Rename(tmpName, s.path); err != nil {
        return fmt.Errorf("replace transaction store: %w", err)
    }
    return nil
}