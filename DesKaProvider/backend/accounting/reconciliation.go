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
	ReconciliationOrphanedLedger      ReconciliationStatus = "ORPHANED_LEDGER"
	ReconciliationOrphanedAudit       ReconciliationStatus = "ORPHANED_AUDIT"
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
	ledger interface{}
	audit SettlementAuditReader
}

func NewSettlementReconciler(
	transactions routing.ContextReadTransactionStore,
	ledger interface{},
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
	ledgerTransactions, err := readLedgerTransactions(ctx, r.ledger)
	if err != nil {
		return ReconciliationReport{}, fmt.Errorf("read ledger transactions: %w", err)
	}
	byReference := make(map[string]LedgerTransaction, len(ledgerTransactions))
	byLedgerID := make(map[string]LedgerTransaction, len(ledgerTransactions))
	for _, tx := range ledgerTransactions {
		byReference[tx.ReferenceID] = tx
		byLedgerID[tx.ID] = tx
	}

	report := ReconciliationReport{Items: make([]TransactionReconciliation, 0, len(states))}
	providerReferences := make(map[string]struct{}, len(states))
	for _, state := range states {
		referenceID := state.Request.ReferenceID
		if state.Kind == routing.TransactionKindPayment && state.Payment != nil {
			referenceID = state.Payment.ReferenceID
		}
		providerReferences[referenceID] = struct{}{}
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

	for _, tx := range ledgerTransactions {
		if _, ok := providerReferences[tx.ReferenceID]; ok {
			continue
		}
		audit, auditOK, err := r.audit.GetSettlementAudit(ctx, tx.ID)
		if err != nil {
			return ReconciliationReport{}, fmt.Errorf("read settlement audit for orphaned ledger %s: %w", tx.ID, err)
		}
		item := TransactionReconciliation{
			ReferenceID: tx.ReferenceID,
			ProviderStatus: "UNKNOWN",
			LedgerTransactionID: tx.ID,
			Status: ReconciliationOrphanedLedger,
		}
		if auditOK {
			item.SettlementAuditEventID = audit.EventID
		}
		report.Items = append(report.Items, item)
	}

	if audits, ok := r.audit.(ContextSettlementAuditReader); ok {
		allAudits, err := audits.AllSettlementAudits(ctx)
		if err != nil {
			return ReconciliationReport{}, fmt.Errorf("read settlement audits: %w", err)
		}
		for _, audit := range allAudits {
			if _, ledgerOK := byLedgerID[audit.TransactionID]; ledgerOK {
				continue
			}
			report.Items = append(report.Items, TransactionReconciliation{
				ReferenceID: audit.ReferenceID,
				ProviderStatus: "UNKNOWN",
				SettlementAuditEventID: audit.EventID,
				Status: ReconciliationOrphanedAudit,
			})
		}
	}
	return report, nil
}

func readLedgerTransactions(ctx context.Context, store interface{}) ([]LedgerTransaction, error) {
	if durable, ok := store.(ContextLedgerReader); ok {
		return durable.All(ctx)
	}
	if memory, ok := store.(Store); ok {
		return memory.All(), nil
	}
	return nil, errors.New("ledger dependency does not implement a supported read interface")
}

var _ ReconciliationReader = (*SettlementReconciler)(nil)
