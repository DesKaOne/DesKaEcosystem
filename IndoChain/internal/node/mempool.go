package node

import (
	"errors"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/transaction"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/mempool"
)

var (
	ErrNilMempool = errors.New("nil mempool")
	ErrInvalidTransaction = errors.New("invalid transaction for mempool")
)

type TransactionSubmitter interface {
	SubmitTransaction(tx transaction.Transaction, publicKey []byte) error
}

type NodeMempool struct {
	Node *Node
	Pool *mempool.Pool
}

func NewNodeMempool(n *Node, pool *mempool.Pool) *NodeMempool {
	return &NodeMempool{Node: n, Pool: pool}
}

// SubmitTransaction validates a transaction against the node's current
// execution rules and canonical state, then admits it to the mempool.
// Admission does not change canonical state or mark the transaction confirmed.
func (m *NodeMempool) SubmitTransaction(tx transaction.Transaction, publicKey []byte) error {
	if m == nil || m.Node == nil {
		return ErrNilNode
	}
	if m.Pool == nil {
		return ErrNilMempool
	}
	rules, err := m.Node.Config.BlockRules(publicKey)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidTransaction, err)
	}
	working := m.Node.State.Snapshot()
	if err := transaction.ValidateAndVerify(tx, rules.Transaction.Validation, publicKey); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidTransaction, err)
	}
	if err := transaction.ValidateSignature(tx, publicKey); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidTransaction, err)
	}
	if err := working.Transfer(tx.Sender, tx.Recipient, tx.Value, tx.Nonce); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidTransaction, err)
	}
	return m.Pool.Add(tx)
}
