package block

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/transaction"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var (
	ErrInvalidBlockTransaction   = errors.New("invalid block transaction")
	ErrTransactionsRootMismatch  = errors.New("transactions root mismatch")
)

// TransactionsRoot returns a deterministic development commitment over the
// ordered signed transaction encodings. The algorithm is intentionally not
// the final protocol commitment until serialization-spec-v0.1 is frozen.
func TransactionsRoot(txs []any) (types.Hash, error) {
	var b bytes.Buffer
	for index, rawTx := range txs {
		tx, ok := rawTx.(transaction.Transaction)
		if !ok {
			return types.Hash{}, fmt.Errorf("%w at index %d", ErrInvalidBlockTransaction, index)
		}
		encoded, err := transaction.SignedBytes(tx)
		if err != nil {
			return types.Hash{}, fmt.Errorf("transaction %d: %w", index, err)
		}
		var length [4]byte
		if uint64(len(encoded)) > uint64(^uint32(0)) {
			return types.Hash{}, fmt.Errorf("transaction %d too large", index)
		}
		binary.BigEndian.PutUint32(length[:], uint32(len(encoded)))
		_, _ = b.Write(length[:])
		_, _ = b.Write(encoded)
	}
	return sha256.Sum256(b.Bytes()), nil
}

// ValidateTransactionsRoot checks the block header commitment against the
// development transaction commitment. A zero header root remains accepted
// while the protocol is still in development.
func ValidateTransactionsRoot(b Block) error {
	root, err := TransactionsRoot(b.Transactions)
	if err != nil {
		return err
	}
	if b.Header.TransactionsRoot != (types.Hash{}) && b.Header.TransactionsRoot != root {
		return ErrTransactionsRootMismatch
	}
	return nil
}
