package transaction

import (
    "errors"

    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var (
    ErrInvalidVersion = errors.New("invalid transaction version")
    ErrInvalidChainID = errors.New("invalid transaction chain id")
    ErrInvalidSender = errors.New("invalid transaction sender")
    ErrInvalidRecipient = errors.New("invalid transaction recipient")
    ErrInvalidSignature = errors.New("invalid transaction signature")
    ErrInvalidPublicKey = errors.New("invalid transaction public key")
)

// ValidationRules contains development transaction validation limits.
// Exact protocol limits remain subject to protocol freeze.
type ValidationRules struct {
    ProtocolVersion types.ProtocolVersion
    ChainID         types.ChainID
    RequireSender   bool
    RequireRecipient bool
    RequireSignature bool
    MinGasLimit     uint64
    MaxDataSize     uint32
}

// Validate checks structural and replay-protection fields.
func Validate(tx Transaction, rules ValidationRules) error {
    if tx.Version != rules.ProtocolVersion {
        return ErrInvalidVersion
    }
    if tx.ChainID != rules.ChainID {
        return ErrInvalidChainID
    }
    if rules.RequireSender && len(tx.Sender) == 0 {
        return ErrInvalidSender
    }
    if rules.RequireRecipient && len(tx.Recipient) == 0 {
        return ErrInvalidRecipient
    }
    if tx.GasLimit < rules.MinGasLimit {
        return errors.New("gas limit below minimum")
    }
    if uint64(len(tx.Data)) > uint64(rules.MaxDataSize) {
        return errors.New("transaction data too large")
    }
    if rules.RequireSignature && len(tx.Signature) == 0 {
        return ErrInvalidSignature
    }
    return nil
}

// ValidateSignature verifies a transaction signature after structural validation.
func ValidateSignature(tx Transaction, publicKey []byte) error {
    if len(publicKey) == 0 {
        return ErrInvalidPublicKey
    }
    ok, err := VerifySignature(tx, tx.Signature, publicKey)
    if err != nil {
        return err
    }
    if !ok {
        return ErrInvalidSignature
    }
    return nil
}

// ValidateAndVerify combines structural and signature validation.
func ValidateAndVerify(tx Transaction, rules ValidationRules, publicKey []byte) error {
    if err := Validate(tx, rules); err != nil {
        return err
    }
    return ValidateSignature(tx, publicKey)
}

var _ crypto.Verifier = verifierAdapter{}

type verifierAdapter struct{}

func (verifierAdapter) Verify(message, signature, publicKey []byte) bool {
    return crypto.VerifyEd25519(message, signature, publicKey)
}
