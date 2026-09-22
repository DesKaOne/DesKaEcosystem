package state

import (
	"errors"
	"math"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/transaction"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var (
	ErrBalanceOverflow = errors.New("balance overflow")
)

// ExecutionRules defines the development transaction-to-state execution boundary.
// Fee charging and VM execution are intentionally deferred until their protocol
// specifications are frozen.
type ExecutionRules struct {
	Validation transaction.ValidationRules
	PublicKey  []byte
}

// ApplyTransaction validates and applies one transaction atomically.
//
// The current development execution supports native-value transfers only.
// A failed transaction leaves the supplied state unchanged.
func ApplyTransaction(s *State, tx transaction.Transaction, rules ExecutionRules) error {
	if s == nil {
		return errors.New("nil state")
	}
	if err := transaction.ValidateAndVerify(tx, rules.Validation, rules.PublicKey); err != nil {
		return err
	}
	if len(tx.Sender) == 0 || len(tx.Recipient) == 0 {
		return errors.New("transaction requires sender and recipient")
	}

	working := s.Snapshot()
	if err := working.Transfer(tx.Sender, tx.Recipient, tx.Value, tx.Nonce); err != nil {
		return err
	}

	s.Replace(working)
	return nil
}

// CheckTransferOverflow reports whether adding amount to balance would overflow
// the development uint64 balance representation.
func CheckTransferOverflow(balance, amount uint64) bool {
	return amount > math.MaxUint64-balance
}

// ApplyNativeTransfer is a low-level execution helper for callers that already
// performed transaction validation. It is not a replacement for ApplyTransaction.
func ApplyNativeTransfer(s *State, sender, recipient types.Address, amount uint64, nonce types.Nonce) error {
	if s == nil {
		return errors.New("nil state")
	}
	return s.Transfer(sender, recipient, amount, nonce)
}
