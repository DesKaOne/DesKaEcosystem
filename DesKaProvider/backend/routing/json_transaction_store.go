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

    s.mu.Lock()
    defer s.mu.Unlock()
    if previous, ok := s.transactions[referenceID]; ok {
        if err := validateTransactionTransition(previous, state); err != nil {
            return err
        }
    }
    s.transactions[referenceID] = state
    return s.persistLocked()
}

func (s *JSONFileTransactionStore) PutIfCurrent(referenceID string, previous, next TransactionState) error {
    if referenceID == "" || transactionReferenceID(next) != referenceID || transactionReferenceID(previous) != referenceID {
        return ErrReferenceConflict
    }
    s.mu.Lock()
    defer s.mu.Unlock()
    current, ok := s.transactions[referenceID]
    if !ok || current != previous {
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