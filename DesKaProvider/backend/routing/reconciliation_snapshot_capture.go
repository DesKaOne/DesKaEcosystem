package routing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
)

var ErrReconciliationSnapshotTokenMismatch = errors.New("reconciliation snapshot token mismatch")

func transactionReconciliationSnapshotFingerprint(states []TransactionState) (string, error) {
	sorted := append([]TransactionState(nil), states...)
	sort.SliceStable(sorted, func(i, j int) bool {
		ri, rj := transactionReferenceID(sorted[i]), transactionReferenceID(sorted[j])
		if ri != rj { return ri < rj }
		if sorted[i].Version != sorted[j].Version { return sorted[i].Version < sorted[j].Version }
		return string(normalizeTransactionKind(sorted[i].Kind)) < string(normalizeTransactionKind(sorted[j].Kind))
	})
	payload, err := json.Marshal(sorted)
	if err != nil { return "", err }
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func verifyTransactionReconciliationSnapshot(expected string, states []TransactionState) error {
	actual, err := transactionReconciliationSnapshotFingerprint(states)
	if err != nil { return err }
	if actual != expected { return ErrReconciliationSnapshotTokenMismatch }
	return nil
}

func (s *MemoryTransactionStore) CaptureReconciliationSnapshot(ctx context.Context) ([]TransactionState, string, error) {
	states, err := s.AllContextE(ctx)
	if err != nil { return nil, "", err }
	token, err := transactionReconciliationSnapshotFingerprint(states)
	if err != nil { return nil, "", err }
	return states, token, nil
}

func (s *MemoryTransactionStore) VerifyReconciliationSnapshot(ctx context.Context, expected string) error {
	states, err := s.AllContextE(ctx)
	if err != nil { return err }
	return verifyTransactionReconciliationSnapshot(expected, states)
}

func (s *PostgresTransactionStore) CaptureReconciliationSnapshot(ctx context.Context) ([]TransactionState, string, error) {
	states, err := s.AllContextE(ctx)
	if err != nil { return nil, "", err }
	token, err := transactionReconciliationSnapshotFingerprint(states)
	if err != nil { return nil, "", err }
	return states, token, nil
}

func (s *PostgresTransactionStore) VerifyReconciliationSnapshot(ctx context.Context, expected string) error {
	states, err := s.AllContextE(ctx)
	if err != nil { return err }
	return verifyTransactionReconciliationSnapshot(expected, states)
}
