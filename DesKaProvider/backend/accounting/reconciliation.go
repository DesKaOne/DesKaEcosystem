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
	PersistenceEvidence *ReconciliationPersistenceEvidence
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

type reconciliationSnapshotBindingToken struct{}

type ReconciliationSnapshotBindingContext struct {
	fingerprint string
	token       *reconciliationSnapshotBindingToken
}

// SnapshotBindingContext returns the non-exportable binding capability carried
// by a report produced by Reconcile. A context cannot be reconstructed from
// copied snapshot metadata or serialized report fields.
func (r ReconciliationReport) SnapshotBindingContext() ReconciliationSnapshotBindingContext {
	return ReconciliationSnapshotBindingContext{
		fingerprint: r.Snapshot.SnapshotFingerprint,
		token:       r.snapshotBindingToken,
	}
}

type ReconciliationReport struct {
	Items               []TransactionReconciliation
	Snapshot            ReconciliationSnapshotMetadata
	snapshotBindingToken *reconciliationSnapshotBindingToken
}

// Economic agreement is required before a successful provider state can be correlated.
type ReconciliationReader interface {
	Reconcile(context.Context) (ReconciliationReport, error)
}

// Stable snapshot-capture classifications. Callers may use errors.Is to
// distinguish which observational dataset boundary failed without depending
// on error strings or treating the failure as a financial mutation outcome.
var (
	ErrReconciliationProviderRead   = errors.New("reconciliation provider transaction read failed")
	ErrReconciliationLedgerRead     = errors.New("reconciliation ledger read failed")
	ErrReconciliationAuditRead      = errors.New("reconciliation settlement audit read failed")
	ErrReconciliationFingerprint    = errors.New("reconciliation snapshot fingerprint failed")
	ErrReconciliationSnapshotChanged = errors.New("reconciliation snapshot changed during verification")
)

type reconciliationProviderSnapshotCapture interface {
	CaptureReconciliationSnapshot(context.Context) ([]routing.TransactionState, string, error)
	VerifyReconciliationSnapshot(context.Context, string) error
}

type reconciliationLedgerSnapshotCapture interface {
	CaptureLedgerReconciliationSnapshot(context.Context) ([]LedgerTransaction, string, error)
	VerifyLedgerReconciliationSnapshot(context.Context, string) error
}

type reconciliationAuditSnapshotCapture interface {
	CaptureSettlementAuditReconciliationSnapshot(context.Context) ([]SettlementAudit, string, error)
	VerifySettlementAuditReconciliationSnapshot(context.Context, string) error
}

type reconciliationSnapshotFingerprinter func([]routing.TransactionState, []LedgerTransaction, []SettlementAudit) (string, error)

const (
	ReconciliationSnapshotConsistencyCaptured = "captured"
	ReconciliationSnapshotConsistencyCapturedVerified = "captured_verified"
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

	var (
		states []routing.TransactionState
		ledgerTransactions []LedgerTransaction
		audits []SettlementAudit
		providerToken string
		ledgerToken string
		auditToken string
		providerVerified bool
		ledgerVerified bool
		auditVerified bool
	)

	ledgerReader := "legacy-memory-or-unsupported"
	auditReader := "legacy-per-ledger"

	if reader, ok := r.transactions.(reconciliationProviderSnapshotCapture); ok {
		var err error
		states, providerToken, err = reader.CaptureReconciliationSnapshot(ctx)
		if err != nil {
			return reconciliationSnapshot{}, fmt.Errorf("%w: %w", ErrReconciliationProviderRead, err)
		}
		providerVerified = true
	} else {
		var err error
		states, err = r.transactions.AllContextE(ctx)
		if err != nil {
			return reconciliationSnapshot{}, fmt.Errorf("%w: %w", ErrReconciliationProviderRead, err)
		}
	}

	if reader, ok := r.ledger.(reconciliationLedgerSnapshotCapture); ok {
		var err error
		ledgerTransactions, ledgerToken, err = reader.CaptureLedgerReconciliationSnapshot(ctx)
		if err != nil {
			return reconciliationSnapshot{}, fmt.Errorf("%w: %w", ErrReconciliationLedgerRead, err)
		}
		ledgerReader = "snapshot-capture"
		ledgerVerified = true
	} else {
		if _, ok := r.ledger.(ContextLedgerReader); ok {
			ledgerReader = "context-ledger"
		} else if _, ok := r.ledger.(Store); ok {
			ledgerReader = "memory-store"
		}
		var err error
		ledgerTransactions, err = readLedgerTransactions(ctx, r.ledger)
		if err != nil {
			return reconciliationSnapshot{}, fmt.Errorf("%w: %w", ErrReconciliationLedgerRead, err)
		}
	}

	if reader, ok := r.audit.(reconciliationAuditSnapshotCapture); ok {
		var err error
		audits, auditToken, err = reader.CaptureSettlementAuditReconciliationSnapshot(ctx)
		if err != nil {
			return reconciliationSnapshot{}, fmt.Errorf("%w: %w", ErrReconciliationAuditRead, err)
		}
		auditReader = "snapshot-capture"
		auditVerified = true
	} else if reader, ok := r.audit.(ContextSettlementAuditReader); ok {
		var err error
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

	// Re-verify store-local capture tokens after all datasets have been read.
	// A mismatch proves that the observed source changed during the capture
	// window. Reconciliation fails closed rather than classifying mixed-time
	// evidence as current.
	if providerVerified {
		reader := r.transactions.(reconciliationProviderSnapshotCapture)
		if err := reader.VerifyReconciliationSnapshot(ctx, providerToken); err != nil {
			if errors.Is(err, routing.ErrReconciliationSnapshotTokenMismatch) {
				return reconciliationSnapshot{}, fmt.Errorf("%w: provider: %w", ErrReconciliationSnapshotChanged, err)
			}
			return reconciliationSnapshot{}, fmt.Errorf("%w: %w", ErrReconciliationProviderRead, err)
		}
	}
	if ledgerVerified {
		reader := r.ledger.(reconciliationLedgerSnapshotCapture)
		if err := reader.VerifyLedgerReconciliationSnapshot(ctx, ledgerToken); err != nil {
			if errors.Is(err, ErrReconciliationSnapshotTokenMismatch) {
				return reconciliationSnapshot{}, fmt.Errorf("%w: ledger: %w", ErrReconciliationSnapshotChanged, err)
			}
			return reconciliationSnapshot{}, fmt.Errorf("%w: %w", ErrReconciliationLedgerRead, err)
		}
	}
	if auditVerified {
		reader := r.audit.(reconciliationAuditSnapshotCapture)
		if err := reader.VerifySettlementAuditReconciliationSnapshot(ctx, auditToken); err != nil {
			if errors.Is(err, ErrReconciliationSnapshotTokenMismatch) {
				return reconciliationSnapshot{}, fmt.Errorf("%w: audit: %w", ErrReconciliationSnapshotChanged, err)
			}
			return reconciliationSnapshot{}, fmt.Errorf("%w: %w", ErrReconciliationAuditRead, err)
		}
	}

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

	consistency := ReconciliationSnapshotConsistencyCaptured
	if auditReader == "legacy-per-ledger" {
		consistency = ReconciliationSnapshotConsistencyLegacyMixed
	} else if providerVerified && ledgerVerified && auditVerified {
		consistency = ReconciliationSnapshotConsistencyCapturedVerified
	}

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
			ProviderReader: func() string {
				if providerVerified { return "snapshot-capture" }
				return "context-all"
			}(),
			LedgerReader: ledgerReader,
			SettlementAuditReader: auditReader,
			SnapshotConsistency: consistency,
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

func (r ReconciliationReport) WithPersistenceEvidence(referenceID string, evidence ReconciliationPersistenceEvidence) ReconciliationReport {
	result := r
	result.Items = append([]TransactionReconciliation(nil), r.Items...)

	matchIndex := -1
	matchCount := 0
	for i := range result.Items {
		if result.Items[i].ReferenceID != referenceID {
			continue
		}
		matchIndex = i
		matchCount++
	}
	// A reference must uniquely identify the reconciliation item before
	// persistence evidence can be attached. Duplicate references are
	// intentionally left unresolved because one evidence observation cannot
	// safely disambiguate multiple financial records.
	if matchCount != 1 {
		return result
	}

	evidence = evidence.WithSnapshotMetadata(r.Snapshot)
	copy := evidence
	result.Items[matchIndex].PersistenceEvidence = &copy
	return result
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
	report.snapshotBindingToken = &reconciliationSnapshotBindingToken{}
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
