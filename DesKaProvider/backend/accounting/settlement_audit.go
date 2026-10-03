package accounting

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type SettlementAudit struct {
	EventID       string
	TransactionID string
	ReferenceID   string
	SourceType    string
	SourceID      string
	Status        string
	CreatedAt     time.Time
}

var ErrInvalidSettlementAudit = errors.New("invalid settlement audit")

func (a SettlementAudit) Validate() error {
	if a.EventID == "" || a.TransactionID == "" || a.ReferenceID == "" ||
		a.SourceType == "" || a.SourceID == "" || a.Status == "" || a.CreatedAt.IsZero() {
		return fmt.Errorf("%w: event id, transaction id, reference id, source type, source id, status, and created at are required", ErrInvalidSettlementAudit)
	}
	return nil
}

var ErrSettlementPersistenceAmbiguous = errors.New("settlement persistence outcome is ambiguous")
var ErrSettlementAuditConflict = errors.New("settlement audit identity conflict")

type SettlementStore interface {
	AppendSettlement(context.Context, LedgerTransaction, SettlementAudit) error
	GetSettlementAudit(context.Context, string) (SettlementAudit, bool, error)
}

type SettlementAuditReader interface {
	GetSettlementAudit(context.Context, string) (SettlementAudit, bool, error)
}

type ContextSettlementAuditReader interface {
	AllSettlementAudits(context.Context) ([]SettlementAudit, error)
}
