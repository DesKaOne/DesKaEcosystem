package accounting

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
)

var ErrReconciliationSnapshotTokenMismatch = errors.New("reconciliation snapshot token mismatch")

func ledgerReconciliationSnapshotFingerprint(values []LedgerTransaction) (string, error) {
	sorted := append([]LedgerTransaction(nil), values...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].ID != sorted[j].ID { return sorted[i].ID < sorted[j].ID }
		return sorted[i].CreatedAt.Before(sorted[j].CreatedAt)
	})
	payload, err := json.Marshal(sorted)
	if err != nil { return "", err }
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func settlementAuditReconciliationSnapshotFingerprint(values []SettlementAudit) (string, error) {
	sorted := append([]SettlementAudit(nil), values...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].EventID != sorted[j].EventID { return sorted[i].EventID < sorted[j].EventID }
		return sorted[i].TransactionID < sorted[j].TransactionID
	})
	payload, err := json.Marshal(sorted)
	if err != nil { return "", err }
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func verifyLedgerReconciliationSnapshot(expected string, values []LedgerTransaction) error {
	actual, err := ledgerReconciliationSnapshotFingerprint(values)
	if err != nil { return err }
	if actual != expected { return ErrReconciliationSnapshotTokenMismatch }
	return nil
}

func verifySettlementAuditReconciliationSnapshot(expected string, values []SettlementAudit) error {
	actual, err := settlementAuditReconciliationSnapshotFingerprint(values)
	if err != nil { return err }
	if actual != expected { return ErrReconciliationSnapshotTokenMismatch }
	return nil
}

func (s *PostgresStore) CaptureLedgerReconciliationSnapshot(ctx context.Context) ([]LedgerTransaction, string, error) {
	values, err := s.AllContext(ctx)
	if err != nil { return nil, "", err }
	token, err := ledgerReconciliationSnapshotFingerprint(values)
	if err != nil { return nil, "", err }
	return values, token, nil
}

func (s *PostgresStore) VerifyLedgerReconciliationSnapshot(ctx context.Context, expected string) error {
	values, err := s.AllContext(ctx)
	if err != nil { return err }
	return verifyLedgerReconciliationSnapshot(expected, values)
}

func (s *PostgresStore) CaptureSettlementAuditReconciliationSnapshot(ctx context.Context) ([]SettlementAudit, string, error) {
	values, err := s.AllSettlementAudits(ctx)
	if err != nil { return nil, "", err }
	token, err := settlementAuditReconciliationSnapshotFingerprint(values)
	if err != nil { return nil, "", err }
	return values, token, nil
}

func (s *PostgresStore) VerifySettlementAuditReconciliationSnapshot(ctx context.Context, expected string) error {
	values, err := s.AllSettlementAudits(ctx)
	if err != nil { return err }
	return verifySettlementAuditReconciliationSnapshot(expected, values)
}
