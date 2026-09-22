package node

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/genesis/devnet"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/transaction"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

func TestChainReaderHeadAndBlockByHeight(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store)
	if err != nil {
		t.Fatal(err)
	}

	head, headHash, err := n.HeadBlock()
	if err != nil {
		t.Fatal(err)
	}
	if head.Header.Height != 0 || headHash != n.HeadHash {
		t.Fatal("head query does not match node head")
	}

	genesis, genesisHash, err := n.BlockByHeight(0)
	if err != nil {
		t.Fatal(err)
	}
	if genesis.Header.Height != 0 || genesisHash != n.HeadHash {
		t.Fatal("height query does not return canonical genesis")
	}

	if _, _, err := n.BlockByHeight(1); err != storage.ErrBlockNotFound {
		t.Fatalf("error = %v, want %v", err, storage.ErrBlockNotFound)
	}
}

func TestChainReaderTransactionByHash(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store)
	if err != nil {
		t.Fatal(err)
	}
	seed := make([]byte, 32)
	seed[0] = 31
	keys, err := crypto.NewEd25519KeyPair(seed)
	if err != nil { t.Fatal(err) }
	signer, err := crypto.NewEd25519Signer(keys.PrivateKey)
	if err != nil { t.Fatal(err) }
	tx := transaction.Transaction{Version: devnet.ProtocolVersion, ChainID: devnet.ChainID, Nonce: 0, Sender: []byte("q-sender"), Recipient: []byte("q-recipient"), Value: 7, GasLimit: 100}
	sig, err := transaction.Sign(tx, signer)
	if err != nil { t.Fatal(err) }
	tx.Signature = sig
	b := block.Block{Header: block.Header{Version: devnet.ProtocolVersion, ChainID: devnet.ChainID, Height: 1, Timestamp: n.Head.Header.Timestamp + 1, PreviousHash: n.HeadHash}, Transactions: []any{tx}}
	b.Header.TransactionsRoot, err = block.TransactionsRoot(b.Transactions)
	if err != nil { t.Fatal(err) }
	if err := n.ImportBlock(b, signer.PublicKey()); err == nil { t.Fatal("expected state execution failure for unfunded sender") }
	// Query the transaction from a directly stored block to isolate the lookup boundary.
	hash := transaction.Hash(tx)
	blockHash, err := block.Hash(b)
	if err != nil { t.Fatal(err) }
	if err := store.SaveBlock(b, blockHash); err != nil { t.Fatal(err) }
	record, err := n.TransactionByHash(hash)
	if err != nil { t.Fatal(err) }
	if record.BlockHeight != 1 || record.BlockHash != blockHash || record.Index != 0 || transaction.Hash(record.Transaction) != hash {
		t.Fatal("transaction query returned incorrect record")
	}
	if _, err := n.TransactionByHash(types.Hash{99}); err != ErrTransactionNotFound { t.Fatalf("error = %v, want %v", err, ErrTransactionNotFound) }
}

func TestChainReaderStateSnapshotIsolated(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store)
	if err != nil {
		t.Fatal(err)
	}

	address := types.Address([]byte("query-isolation"))
	n.State.Set(address, state.Account{Balance: 10, Nonce: 0})

	snapshot, err := n.StateSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Set(address, state.Account{Balance: 999, Nonce: 99})

	got, ok := n.State.Get(address)
	if !ok {
		t.Fatal("canonical state account disappeared")
	}
	if got.Balance != 10 || got.Nonce != 0 {
		t.Fatalf("canonical state mutated through query snapshot: %+v", got)
	}
}

func TestChainReaderRejectsNilNode(t *testing.T) {
	var n *Node

	if _, _, err := n.HeadBlock(); err != ErrNilNode {
		t.Fatalf("HeadBlock error = %v, want %v", err, ErrNilNode)
	}
	if _, _, err := n.BlockByHeight(0); err != ErrNilNode {
		t.Fatalf("BlockByHeight error = %v, want %v", err, ErrNilNode)
	}
	if _, err := n.StateSnapshot(); err != ErrNilNode {
		t.Fatalf("StateSnapshot error = %v, want %v", err, ErrNilNode)
	}
}
