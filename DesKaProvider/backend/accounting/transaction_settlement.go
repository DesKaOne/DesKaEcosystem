package accounting

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/routing"
)

var ErrSettlementTransactionNotFound = errors.New("provider transaction not found")

// PostPersistedTransactionSettlement reads the durable provider transaction state
// and uses its terminal success state as an explicit authorization input for
// settlement. Accounting entries, currency, description, transaction ID, and
// timestamp remain explicit inputs; they are never inferred from provider state.
//
// This boundary only posts an already-terminal successful transaction. It does
// not call the external provider, transition provider state, retry, fail over,
// resubmit, fund, or mutate customer balances.
func PostPersistedTransactionSettlement(
	ctx context.Context,
	store routing.ContextReadTransactionStore,
	poster *SettlementPoster,
	referenceID string,
	transactionID string,
	sourceType string,
	currency string,
	description string,
	createdAt time.Time,
	entries []Entry,
) error {
	if store == nil || poster == nil {
		return ErrInvalidSettlement
	}
	state, ok, err := store.GetContextE(ctx, referenceID)
	if err != nil {
		return fmt.Errorf("read persisted provider transaction: %w", err)
	}
	if !ok {
		return ErrSettlementTransactionNotFound
	}

	status := providerStatusFromTransaction(state)
	if status != ProviderStatusSuccess {
		return ErrSettlementNotPostable
	}

	return poster.Post(ctx, SettlementPostingRequest{
		TransactionID:  transactionID,
		ReferenceID:    referenceID,
		SourceType:     sourceType,
		SourceID:       referenceID,
		ProviderStatus: status,
		Currency:       currency,
		Description:    description,
		CreatedAt:      createdAt,
		Entries:        entries,
	})
}

func providerStatusFromTransaction(state routing.TransactionState) string {
	if state.Kind == routing.TransactionKindPayment {
		if state.Payment == nil {
			return ""
		}
		return string(state.Payment.Status)
	}
	return string(state.Execution.Result.Status)
}

