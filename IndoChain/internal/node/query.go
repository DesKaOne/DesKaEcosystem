package node

import (
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/transaction"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

var (
	ErrNilNode             = errors.New("nil node")
	ErrTransactionNotFound = errors.New("transaction not found")
)

type TransactionRecord struct {
	Transaction transaction.Transaction
	BlockHeight types.Height
	BlockHash   types.Hash
	Index       uint32
}

// ChainReader is the read-only chain access boundary used by future RPC,
// explorer, indexer, and service adapters.
type ChainReader interface {
	HeadBlock() (block.Block, types.Hash, error)
	BlockByHeight(height types.Height) (block.Block, types.Hash, error)
	TransactionByHash(hash types.Hash) (TransactionRecord, error)
	StateSnapshot() (*state.State, error)
}

// HeadBlock returns a copy of the current canonical head and its stored hash.
func (n *Node) HeadBlock() (block.Block, types.Hash, error) {
	if n == nil || n.Store == nil {
		return block.Block{}, types.Hash{}, ErrNilNode
	}
	return n.Store.Head()
}

// BlockByHeight returns a block and its stored hash from canonical chain storage.
// It does not execute or mutate the block.
func (n *Node) BlockByHeight(height types.Height) (block.Block, types.Hash, error) {
	if n == nil || n.Store == nil {
		return block.Block{}, types.Hash{}, ErrNilNode
	}
	if height > n.Head.Header.Height {
		return block.Block{}, types.Hash{}, storage.ErrBlockNotFound
	}
	return n.Store.GetBlock(height)
}

// TransactionByHash searches canonical blocks for a transaction hash.
// The v0.1 implementation is intentionally linear; a future index can preserve this contract.
func (n *Node) TransactionByHash(hash types.Hash) (TransactionRecord, error) {
	if n == nil || n.Store == nil {
		return TransactionRecord{}, ErrNilNode
	}
	if hash == (types.Hash{}) {
		return TransactionRecord{}, ErrTransactionNotFound
	}
	for height := types.Height(0); height <= n.Head.Header.Height; height++ {
		b, blockHash, err := n.Store.GetBlock(height)
		if err != nil {
			return TransactionRecord{}, err
		}
		for index, rawTx := range b.Transactions {
			tx, ok := rawTx.(transaction.Transaction)
			if ok && transaction.Hash(tx) == hash {
				return TransactionRecord{Transaction: tx, BlockHeight: height, BlockHash: blockHash, Index: uint32(index)}, nil
			}
		}
	}
	return TransactionRecord{}, ErrTransactionNotFound
}

// StateSnapshot returns an isolated state snapshot. Mutating the returned state
// does not mutate the node's canonical in-memory state.
func (n *Node) StateSnapshot() (*state.State, error) {
	if n == nil || n.State == nil {
		return nil, ErrNilNode
	}
	return n.State.Snapshot(), nil
}

var _ ChainReader = (*Node)(nil)
