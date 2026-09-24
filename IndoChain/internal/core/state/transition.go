package state

import (
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/transaction"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

// ExecutionRules defines the development transaction-to-state execution boundary.
// Fee charging and VM execution are intentionally deferred until their protocol
// specifications are frozen.
type ExecutionRules struct {
	Validation transaction.ValidationRules
	PublicKey  []byte
	PublicKeyResolver PublicKeyResolver
}

// ApplyTransaction validates and applies one transaction atomically.
//
// The current development execution supports native-value transfers only.
// A failed transaction leaves the supplied state unchanged.
func ApplyTransaction(s *State, tx transaction.Transaction, rules ExecutionRules) error {
	if s == nil {
		return errors.New("nil state")
	}
	publicKey := rules.PublicKey
	if rules.PublicKeyResolver != nil {
		sender := append([]byte(nil), tx.Sender...)
		resolved, err := rules.PublicKeyResolver.PublicKeyForSender(sender)
		if err != nil { return err }
		publicKey = resolved
	}
	if err := transaction.ValidateAndVerify(tx, rules.Validation, publicKey); err != nil {
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

// ApplyNativeTransfer is a low-level execution helper for callers that already
// performed transaction validation. It is not a replacement for ApplyTransaction.
func ApplyNativeTransfer(s *State, sender, recipient types.Address, amount uint64, nonce types.Nonce) error {
	if s == nil {
		return errors.New("nil state")
	}
	return s.Transfer(sender, recipient, amount, nonce)
}
