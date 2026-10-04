package accounting

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"bytes"
	"errors"
	"time"
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
	ReconciliationDuplicateReference ReconciliationStatus = "DUPLICATE_REFERENCE"
	ReconciliationDuplicateAuditIdentity ReconciliationStatus = "DUPLICATE_AUDIT_IDENTITY"
)

type TransactionReconciliation struct {
	ReferenceID string
	ProviderStatus string
	LedgerTransactionID string
	SettlementAuditEventID string
	Status                    ReconciliationStatus
	LedgerTransactionIDs      []string
	SettlementAuditEventIDs []string
}

type ReconciliationSnapshotMetadata struct {
	CaptureStartedAt          time.Time
	CaptureCompletedAt        time.Time
	CapturedAt                time.Time
	ProviderTransactionCount  int
	LedgerTransactionCount    int
	SettlementAuditCount      int
	ProviderReader            string
	LedgerReader              string
	SettlementAuditReader     string
	SnapshotConsistency       string
	SnapshotFingerprint       string
	SnapshotWindowMillis      int64
}

type ReconciliationReport struct {
	Items    []TransactionReconciliation
	Snapshot ReconciliationSnapshotMetadata
}

// Economic agreement is required before a successful provider state can be correlated.
type ReconciliationReader interface {
	Reconcile(context.Context) (ReconciliationReport, error)
}

// ReconciliationSnapshotCapture is an optional capability for a composed
// reader that can materialize provider, ledger, and audit datasets under one
// consistency boundary. Implementations must remain read-only.
type ReconciliationSnapshotCapture interface {
	CaptureReconciliationSnapshot(context.Context) (reconciliationSnapshot, error)
}

// Stable snapshot-capture classifications. Callers may use errors.Is to
// distinguish which observational dataset boundary failed without depending
// on error strings or treating the failure as a financial mutation outcome.
var (
	ErrReconciliationProviderRead   = errors.New("reconciliation provider transaction read failed")
	ErrReconciliationLedgerRead     = errors.New("reconciliation ledger read failed")
	ErrReconciliationAuditRead      = errors.New("reconciliation settlement audit read failed")
	ErrReconciliationFingerprint    = errors.New("reconciliation snapshot fingerprint failed")
)

type reconciliationSnapshotFingerprinter func([]routing.TransactionState, []LedgerTransaction, []SettlementAudit) (string, error)

const (
	ReconciliationSnapshotConsistencyCaptured = "captured"
	ReconciliationSnapshotConsistencyLegacyMixed = "legacy_mixed"
)

type SettlementReconciler struct {
	transactions routing.ContextReadTransactionStore
	ledger interface{}
	audit SettlementAuditReader
	fingerprint reconciliationSnapshotFingerprinter
}

type reconciliationSnapshot struct {
	states  []routing.TransactionState
	ledger  []LedgerTransaction
	audits  []SettlementAudit
	metadata ReconciliationSnapshotMetadata
}

func (r *SettlementReconciler) readSnapshot(ctx context.Context) (reconciliationSnapshot, error) {
	captureStartedAt := time.Now().UTC()
	states, err := r.transactions.AllContextE(ctx)
	if err != nil {
		return reconciliationSnapshot{}, fmt.Errorf("%w: %w", ErrReconciliationProviderRead, err)
	}
	ledgerReader := "legacy-memory-or-unsupported"
	if _, ok := r.ledger.(ContextLedgerReader); ok {
		ledgerReader = "context-ledger"
	} else if _, ok := r.ledger.(Store); ok {
		ledgerReader = "memory-store"
	}
	ledgerTransactions, err := readLedgerTransactions(ctx, r.ledger)
	if err != nil {
		return reconciliationSnapshot{}, fmt.Errorf("%w: %w", ErrReconciliationLedgerRead, err)
	}

	var audits []SettlementAudit
	auditReader := "legacy-per-ledger"
	if reader, ok := r.audit.(ContextSettlementAuditReader); ok {
		auditReader = "context-bulk"
		audits, err = reader.AllSettlementAudits(ctx)
		if err != nil {
			return reconciliationSnapshot{}, fmt.Errorf("%w: %w", ErrReconciliationAuditRead, err)
		}
	} else {
		audits = make([]SettlementAudit, 0, len(ledgerTransactions))
		for _, tx := range ledgerTransactions {
			audit, ok, err := r.audit.GetSettlementAudit(ctx, tx.ID)
			if err != nil {
				return reconciliationSnapshot{}, fmt.Errorf("%w: %s: %w", ErrReconciliationAuditRead, tx.ID, err)
			}
			if ok {
				audits = append(audits, audit)
			}
		}
	}

	// Copy the materialized datasets so the reconciliation pass never observes
	// later mutations through caller-owned backing arrays.
	states = append([]routing.TransactionState(nil), states...)
	ledgerTransactions = append([]LedgerTransaction(nil), ledgerTransactions...)
	audits = append([]SettlementAudit(nil), audits...)
	fingerprinter := r.fingerprint
	if fingerprinter == nil {
		fingerprinter = reconciliationSnapshotFingerprint
	}
	fingerprint, err := fingerprinter(states, ledgerTransactions, audits)
	if err != nil {
		return reconciliationSnapshot{}, fmt.Errorf("%w: %w", ErrReconciliationFingerprint, err)
	}
	captureCompletedAt := time.Now().UTC()
	return reconciliationSnapshot{
		states: states,
		ledger: ledgerTransactions,
		audits: audits,
		metadata: ReconciliationSnapshotMetadata{
			CaptureStartedAt: captureStartedAt,
			CaptureCompletedAt: captureCompletedAt,
			CapturedAt: captureCompletedAt,
			ProviderTransactionCount: len(states),
			LedgerTransactionCount: len(ledgerTransactions),
			SettlementAuditCount: len(audits),
			ProviderReader: "context-all",
			LedgerReader: ledgerReader,
			SettlementAuditReader: auditReader,
			SnapshotConsistency: func() string {
				if auditReader == "legacy-per-ledger" {
					return ReconciliationSnapshotConsistencyLegacyMixed
				}
				return ReconciliationSnapshotConsistencyCaptured
			}(),
			SnapshotFingerprint: fingerprint,
			SnapshotWindowMillis: captureCompletedAt.Sub(captureStartedAt).Milliseconds(),
		},
	}, nil
}

func NewSettlementReconciler(
	transactions routing.ContextReadTransactionStore,
	ledger interface{},
	audit SettlementAuditReader,
) (*SettlementReconciler, error) {
	if transactions == nil || ledger == nil || audit == nil {
		return nil, errors.New("reconciliation dependencies are required")
	}
	return &SettlementReconciler{transactions: transactions, ledger: ledger, audit: audit, fingerprint: reconciliationSnapshotFingerprint}, nil
}

func (r *SettlementReconciler) Reconcile(ctx context.Context) (ReconciliationReport, error) {
	snapshot, err := r.readSnapshot(ctx)
	if err != nil {
		return ReconciliationReport{}, err
	}
	states := snapshot.states
	ledgerTransactions := snapshot.ledger
	allAudits := snapshot.audits

	auditsByTransaction := make(map[string][]SettlementAudit, len(allAudits))
	duplicateAuditTransactions, duplicateAuditEvents := duplicateAuditIndexes(allAudits)
	for _, audit := range allAudits {
		auditsByTransaction[audit.TransactionID] = append(auditsByTransaction[audit.TransactionID], audit)
	}
	for transactionID := range auditsByTransaction {
		sort.Slice(auditsByTransaction[transactionID], func(i, j int) bool {
			return auditsByTransaction[transactionID][i].EventID < auditsByTransaction[transactionID][j].EventID
		})
	}
	byReference := make(map[string][]LedgerTransaction, len(ledgerTransactions))
	byLedgerID := make(map[string]LedgerTransaction, len(ledgerTransactions))
	for _, tx := range ledgerTransactions {
		byReference[tx.ReferenceID] = append(byReference[tx.ReferenceID], tx)
		byLedgerID[tx.ID] = tx
	}
	for referenceID := range byReference {
		sort.Slice(byReference[referenceID], func(i, j int) bool {
			return byReference[referenceID][i].ID < byReference[referenceID][j].ID
		})
	}

	report := ReconciliationReport{Items: make([]TransactionReconciliation, 0, len(states))}
	providerReferences := make(map[string]struct{}, len(states))
	providerReferenceCounts := make(map[string]int, len(states))
	for _, state := range states {
		referenceID := state.Request.ReferenceID
		if state.Kind == routing.TransactionKindPayment && state.Payment != nil {
			referenceID = state.Payment.ReferenceID
		}
		providerReferences[referenceID] = struct{}{}
		providerReferenceCounts[referenceID]++
	}
	for _, state := range states {
		referenceID := state.Request.ReferenceID
		if state.Kind == routing.TransactionKindPayment && state.Payment != nil {
			referenceID = state.Payment.ReferenceID
		}
		status := providerStatusFromTransaction(state)
		item := TransactionReconciliation{ReferenceID: referenceID, ProviderStatus: status}

		if status != ProviderStatusSuccess {
			item.Status = ReconciliationNotSettleable
			if candidates := byReference[referenceID]; len(candidates) > 0 {
				item.LedgerTransactionIDs = make([]string, len(candidates))
				for i, candidate := range candidates {
					item.LedgerTransactionIDs[i] = candidate.ID
				}
			}
			if len(item.LedgerTransactionIDs) == 1 {
				auditCandidates := auditsByTransaction[item.LedgerTransactionIDs[0]]
				if len(auditCandidates) > 0 {
					item.SettlementAuditEventIDs = make([]string, len(auditCandidates))
					for i, candidate := range auditCandidates {
						item.SettlementAuditEventIDs[i] = candidate.EventID
					}
					sort.Strings(item.SettlementAuditEventIDs)
				}
			}
			report.Items = append(report.Items, item)
			continue
		}
		if providerReferenceCounts[referenceID] > 1 {
			item.Status = ReconciliationDuplicateReference
			report.Items = append(report.Items, item)
			continue
		}

		candidates := byReference[referenceID]
		if len(candidates) == 0 {
			item.Status = ReconciliationLedgerMissing
			report.Items = append(report.Items, item)
			continue
		}
		if len(candidates) > 1 {
			item.Status = ReconciliationDuplicateReference
			item.LedgerTransactionIDs = make([]string, len(candidates))
			for i, candidate := range candidates {
				item.LedgerTransactionIDs[i] = candidate.ID
			}
			report.Items = append(report.Items, item)
			continue
		}
		tx := candidates[0]
		item.LedgerTransactionID = tx.ID

		auditCandidates := auditsByTransaction[tx.ID]
		auditOK := len(auditCandidates) > 0
		var audit SettlementAudit
		if auditOK {
			audit = auditCandidates[0]
		}
		if !auditOK {
			item.Status = ReconciliationAuditMissing
			report.Items = append(report.Items, item)
			continue
		}
		item.SettlementAuditEventID = audit.EventID
		if len(auditCandidates) != 1 || duplicateAuditTransactions[tx.ID] || duplicateAuditEvents[audit.EventID] {
			item.Status = ReconciliationDuplicateAuditIdentity
			item.SettlementAuditEventIDs = make([]string, 0, len(auditCandidates))
			for _, candidate := range auditCandidates {
				item.SettlementAuditEventIDs = append(item.SettlementAuditEventIDs, candidate.EventID)
			}
			sort.Strings(item.SettlementAuditEventIDs)
			item.LedgerTransactionIDs = []string{tx.ID}
			report.Items = append(report.Items, item)
			continue
		}
		if audit.ReferenceID != referenceID || audit.TransactionID != tx.ID ||
			audit.SourceType != tx.SourceType || audit.SourceID != tx.SourceID ||
			audit.Status != ProviderStatusSuccess ||
			!reconciliationEconomicAgreement(state, tx) {
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
		auditCandidates := auditsByTransaction[tx.ID]
		item := TransactionReconciliation{
			ReferenceID: tx.ReferenceID,
			ProviderStatus: "UNKNOWN",
			LedgerTransactionID: tx.ID,
			Status: ReconciliationOrphanedLedger,
		}
		if len(auditCandidates) > 0 {
			if len(auditCandidates) != 1 || duplicateAuditTransactions[tx.ID] || duplicateAuditEvents[auditCandidates[0].EventID] {
				item.Status = ReconciliationDuplicateAuditIdentity
				for _, candidate := range auditCandidates {
					item.SettlementAuditEventIDs = append(item.SettlementAuditEventIDs, candidate.EventID)
				}
				sort.Strings(item.SettlementAuditEventIDs)
			} else {
				audit := auditCandidates[0]
				item.SettlementAuditEventID = audit.EventID
				if !settlementAuditMatchesLedger(audit, tx) {
					item.Status = ReconciliationCorrelationConflict
				}
			}
		}
		report.Items = append(report.Items, item)
	}

	{
		eventGroups := make(map[string][]SettlementAudit, len(allAudits))
		transactionGroups := make(map[string][]SettlementAudit, len(allAudits))
		for _, audit := range allAudits {
			eventGroups[audit.EventID] = append(eventGroups[audit.EventID], audit)
			transactionGroups[audit.TransactionID] = append(transactionGroups[audit.TransactionID], audit)
		}
		eventIDs := make([]string, 0, len(eventGroups))
		for eventID := range eventGroups {
			eventIDs = append(eventIDs, eventID)
		}
		sort.Strings(eventIDs)
		for _, eventID := range eventIDs {
			group := eventGroups[eventID]
			if len(group) < 2 {
				continue
			}
			ids := make([]string, 0, len(group))
			for _, audit := range group {
				ids = append(ids, audit.TransactionID)
			}
			sort.Strings(ids)
			report.Items = append(report.Items, TransactionReconciliation{
				ReferenceID: group[0].ReferenceID,
				ProviderStatus: "UNKNOWN",
				SettlementAuditEventID: eventID,
				Status: ReconciliationDuplicateAuditIdentity,
				SettlementAuditEventIDs: []string{eventID},
				LedgerTransactionIDs: ids,
			})
		}
		transactionIDs := make([]string, 0, len(transactionGroups))
		for transactionID := range transactionGroups {
			transactionIDs = append(transactionIDs, transactionID)
		}
		sort.Strings(transactionIDs)
		for _, transactionID := range transactionIDs {
			group := transactionGroups[transactionID]
			if len(group) < 2 {
				continue
			}
			eventIDs := make([]string, 0, len(group))
			for _, audit := range group {
				eventIDs = append(eventIDs, audit.EventID)
			}
			sort.Strings(eventIDs)
			report.Items = append(report.Items, TransactionReconciliation{
				ReferenceID: group[0].ReferenceID,
				ProviderStatus: "UNKNOWN",
				SettlementAuditEventID: eventIDs[0],
				Status: ReconciliationDuplicateAuditIdentity,
				SettlementAuditEventIDs: eventIDs,
			})
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

	sort.SliceStable(report.Items, func(i, j int) bool {
		return reconciliationItemKey(report.Items[i]) < reconciliationItemKey(report.Items[j])
	})
	report.Snapshot = snapshot.metadata
	return report, nil
}

const reconciliationSnapshotFingerprintVersion = "v1"

func reconciliationSnapshotFingerprint(states []routing.TransactionState, ledgerTransactions []LedgerTransaction, audits []SettlementAudit) (string, error) {
	canonicalize := func(values any, length int) ([]json.RawMessage, error) {
		raw := make([]json.RawMessage, 0, length)
		switch items := values.(type) {
		case []routing.TransactionState:
			for _, item := range items {
				b, err := json.Marshal(item)
				if err != nil { return nil, err }
				raw = append(raw, b)
			}
		case []LedgerTransaction:
			for _, item := range items {
				b, err := json.Marshal(item)
				if err != nil { return nil, err }
				raw = append(raw, b)
			}
		case []SettlementAudit:
			for _, item := range items {
				b, err := json.Marshal(item)
				if err != nil { return nil, err }
				raw = append(raw, b)
			}
		default:
			return nil, fmt.Errorf("unsupported snapshot dataset type %T", values)
		}
		sort.Slice(raw, func(i, j int) bool { return bytes.Compare(raw[i], raw[j]) < 0 })
		return raw, nil
	}
	provider, err := canonicalize(states, len(states))
	if err != nil { return "", err }
	ledger, err := canonicalize(ledgerTransactions, len(ledgerTransactions))
	if err != nil { return "", err }
	auditsJSON, err := canonicalize(audits, len(audits))
	if err != nil { return "", err }
	payload, err := json.Marshal(struct {
		SchemaVersion string `json:"schema_version"`
		Provider []json.RawMessage `json:"provider"`
		Ledger []json.RawMessage `json:"ledger"`
		Audits []json.RawMessage `json:"audits"`
	}{SchemaVersion: reconciliationSnapshotFingerprintVersion, Provider: provider, Ledger: ledger, Audits: auditsJSON})
	if err != nil { return "", err }
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}
func settlementAuditMatchesLedger(audit SettlementAudit, tx LedgerTransaction) bool {
	return audit.TransactionID == tx.ID &&
		audit.ReferenceID == tx.ReferenceID &&
		audit.SourceType == tx.SourceType &&
		audit.SourceID == tx.SourceID
}

func reconciliationItemKey(item TransactionReconciliation) string {
	ledgerIDs := append([]string(nil), item.LedgerTransactionIDs...)
	auditIDs := append([]string(nil), item.SettlementAuditEventIDs...)
	sort.Strings(ledgerIDs)
	sort.Strings(auditIDs)
	return strings.Join([]string{
		item.ReferenceID,
		item.ProviderStatus,
		string(item.Status),
		item.LedgerTransactionID,
		item.SettlementAuditEventID,
		strings.Join(ledgerIDs, "\x00"),
		strings.Join(auditIDs, "\x00"),
	}, "\x00")
}

func readLedgerTransactions(ctx context.Context, store interface{}) ([]LedgerTransaction, error) {
	if durable, ok := store.(ContextLedgerReader); ok {
		return durable.AllContext(ctx)
	}
	if memory, ok := store.(Store); ok {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return memory.All(), nil
	}
	return nil, errors.New("ledger dependency does not implement a supported read interface")
}

var _ ReconciliationReader = (*SettlementReconciler)(nil)
