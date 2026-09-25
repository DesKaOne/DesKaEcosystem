package routing

import (
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "sync"
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
    if state.Request.ReferenceID == "" {
        return errors.New("transaction reference ID is required")
    }
    if state.Execution.ProviderName == "" {
        return errors.New("transaction provider name is required")
    }

    s.mu.Lock()
    defer s.mu.Unlock()
    if previous, ok := s.transactions[state.Request.ReferenceID]; ok {
        if err := validateTransactionTransition(previous, state); err != nil {
            return err
        }
    }
    s.transactions[state.Request.ReferenceID] = state
    return s.persistLocked()
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
