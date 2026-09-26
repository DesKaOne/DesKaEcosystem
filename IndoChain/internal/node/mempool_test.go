package node

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/genesis/devnet"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/transaction"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/mempool"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

func TestNodeMempoolSubmitTransaction(t *testing.T) {
	n, err := NewDevnet(storage.NewMemoryStore())
	if err != nil { t.Fatal(err) }

	seed := make([]byte, 32)
	seed[0] = 41
	keys, err := crypto.NewEd25519KeyPair(seed)
	if err != nil { t.Fatal(err) }
	signer, err := crypto.NewEd25519Signer(keys.PrivateKey)
	if err != nil { t.Fatal(err) }

	sender := []byte("mempool-sender")
	n.State.Set(sender, state.Account{Balance: 50, Nonce: 0})
	tx := transaction.Transaction{
		Version: devnet.ProtocolVersion,
		ChainID: devnet.ChainID,
		Nonce: 0,
		Sender: sender,
		Recipient: []byte("mempool-recipient"),
		Value: 10,
		GasLimit: 100,
	}
	sig, err := transaction.Sign(tx, signer)
	if err != nil { t.Fatal(err) }
	tx.Signature = sig

	pool := mempool.New(mempool.Config{MaxTransactions: 10})
	submitter := NewNodeMempool(n, pool)
	if err := submitter.SubmitTransaction(tx, signer.PublicKey()); err != nil {
		t.Fatal(err)
	}
	if pool.Len() != 1 { t.Fatalf("pool length = %d, want 1", pool.Len()) }
	if got, ok := pool.Get(transaction.Hash(tx).String()); !ok || transaction.Hash(got) != transaction.Hash(tx) {
		t.Fatal("submitted transaction not found in mempool")
	}
	account, ok := n.State.Get(sender)
	if !ok || account.Balance != 50 || account.Nonce != 0 {
		t.Fatalf("submission mutated canonical state: %+v", account)
	}
	if err := submitter.SubmitTransaction(tx, signer.PublicKey()); err != mempool.ErrDuplicate {
		t.Fatalf("duplicate error = %v, want %v", err, mempool.ErrDuplicate)
	}
}

func TestNodeMempoolRejectsInvalidTransactionWithoutMutation(t *testing.T) {
	n, err := NewDevnet(storage.NewMemoryStore())
	if err != nil { t.Fatal(err) }
	pool := mempool.New(mempool.Config{MaxTransactions: 10})
	submitter := NewNodeMempool(n, pool)

	tx := transaction.Transaction{
		Version: devnet.ProtocolVersion,
		ChainID: devnet.ChainID,
		Nonce: 0,
		Sender: []byte("missing"),
		Recipient: []byte("recipient"),
		Value: 10,
		GasLimit: 100,
	}
	if err := submitter.SubmitTransaction(tx, []byte("bad-key")); err == nil {
		t.Fatal("expected invalid transaction error")
	}
	if pool.Len() != 0 { t.Fatal("invalid transaction entered mempool") }
}
