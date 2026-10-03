package accounting

import (
	"context"
	"errors"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/routing"
)

type ReconciliationStatus string

const (
	ReconciliationNotSettleable ReconciliationStatus = "NOT_SETTLEABLE"
	ReconciliationSettlementMissing ReconciliationStatus = "SETTLEMENT_MISSING"
	ReconciliationLedgerMissing ReconciliationStatus = "LEDGER_MISSING"
	ReconciliationAuditMissing ReconciliationStatus = "AUDIT_MISSING"
	ReconciliationCorrelated ReconciliationStatus = "CORRELATED"
	ReconciliationCorrelationConflict ReconciliationStatus = "CORRELATION_CONFLICT"
)

type TransactionReconciliation struct {
	ReferenceID string
	ProviderStatus string
	LedgerTransactionID string
	SettlementAuditEventID string
	Status ReconciliationStatus
}

type ReconciliationReport struct {
	Items []TransactionReconciliation
}

type ReconciliationReader interface {
	Reconcile(context.Context) (ReconciliationReport, error)
}

type SettlementReconciler struct {
	transactions routing.ContextReadTransactionStore
	ledger Store
	audit SettlementAuditReader
}

func NewSettlementReconciler(
	transactions routing.ContextReadTransactionStore,
	ledger Store,
	audit SettlementAuditReader,
) (*SettlementReconciler, error) {
	if transactions == nil || ledger == nil || audit == nil {
		return nil, errors.New("reconciliation dependencies are required")
	}
	return &SettlementReconciler{transactions: transactions, ledger: ledger, audit: audit}, nil
}

func (r *SettlementReconciler) Reconcile(ctx context.Context) (ReconciliationReport, error) {
	states, err := r.transactions.AllContextE(ctx)
	if err != nil {
		return ReconciliationReport{}, fmt.Errorf("read provider transactions: %w", err)
	}
	ledgerTransactions := r.ledger.All()
	byReference := make(map[string]LedgerTransaction, len(ledgerTransactions))
	for _, tx := range ledgerTransactions {
		byReference[tx.ReferenceID] = tx
	}

	report := ReconciliationReport{Items: make([]TransactionReconciliation, 0, len(states))}
	for _, state := range states {
		referenceID := state.Request.ReferenceID
		if state.Kind == routing.TransactionKindPayment && state.Payment != nil {
			referenceID = state.Payment.ReferenceID
		}
		status := providerStatusFromTransaction(state)
		item := TransactionReconciliation{ReferenceID: referenceID, ProviderStatus: status}

		if status != ProviderStatusSuccess {
			item.Status = ReconciliationNotSettleable
			report.Items = append(report.Items, item)
			continue
		}

		tx, ledgerOK := byReference[referenceID]
		if !ledgerOK {
			item.Status = ReconciliationLedgerMissing
			report.Items = append(report.Items, item)
			continue
		}
		item.LedgerTransactionID = tx.ID

		audit, auditOK, err := r.audit.GetSettlementAudit(ctx, tx.ID)
		if err != nil {
			return ReconciliationReport{}, fmt.Errorf("read settlement audit %s: %w", tx.ID, err)
		}
		if !auditOK {
			item.Status = ReconciliationAuditMissing
			report.Items = append(report.Items, item)
			continue
		}
		item.SettlementAuditEventID = audit.EventID
		if audit.ReferenceID != referenceID || audit.TransactionID != tx.ID ||
			audit.SourceType != tx.SourceType || audit.SourceID != tx.SourceID {
			item.Status = ReconciliationCorrelationConflict
			report.Items = append(report.Items, item)
			continue
		}
		item.Status = ReconciliationCorrelated
		report.Items = append(report.Items, item)
	}
	return report, nil
}

var _ ReconciliationReader = (*SettlementReconciler)(nil)
