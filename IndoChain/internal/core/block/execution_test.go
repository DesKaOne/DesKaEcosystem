package block

import (
	"bytes"
	"crypto/ed25519"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/transaction"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
)

func blockRules(pub []byte) ExecutionRules {
	return ExecutionRules{
		ChainID: 1001, ProtocolVersion: 1,
		Transaction: state.ExecutionRules{
			Validation: transaction.ValidationRules{
				ProtocolVersion: 1, ChainID: 1001,
				RequireSender: true, RequireRecipient: true, RequireSignature: true,
				MinGasLimit: 21000, MaxDataSize: 1024,
			},
			PublicKey: pub,
		},
	}
}

func blockTx(t *testing.T, signer *crypto.Ed25519Signer, nonce types.Nonce, value uint64) transaction.Transaction {
	t.Helper()
	tx := transaction.Transaction{
		Version: 1, ChainID: 1001, Nonce: nonce,
		Sender: types.Address{1}, Recipient: types.Address{2},
		Value: value, GasLimit: 21000,
	}
	sig, err := transaction.Sign(tx, signer)
	if err != nil { t.Fatal(err) }
	tx.Signature = sig
	return tx
}

func newBlockSigner(t *testing.T) *crypto.Ed25519Signer {
	t.Helper()
	seed := bytes.Repeat([]byte{0x42}, ed25519.SeedSize)
	signer, err := crypto.NewEd25519Signer(ed25519.NewKeyFromSeed(seed))
	if err != nil { t.Fatal(err) }
	return signer
}

func TestTransactionsRootDeterministic(t *testing.T) {
	signer := newBlockSigner(t)
	txs := []any{blockTx(t, signer, 0, 30), blockTx(t, signer, 1, 20)}
	first, err := TransactionsRoot(txs)
	if err != nil { t.Fatal(err) }
	second, err := TransactionsRoot(txs)
	if err != nil { t.Fatal(err) }
	if first != second { t.Fatalf("root changed between identical calculations: %s vs %s", first, second) }
}

func TestTransactionsRootDependsOnOrder(t *testing.T) {
	signer := newBlockSigner(t)
	firstTx := blockTx(t, signer, 0, 30)
	secondTx := blockTx(t, signer, 1, 20)
	first, err := TransactionsRoot([]any{firstTx, secondTx})
	if err != nil { t.Fatal(err) }
	second, err := TransactionsRoot([]any{secondTx, firstTx})
	if err != nil { t.Fatal(err) }
	if first == second { t.Fatal("transaction order did not affect root") }
}

func TestValidateTransactionsRootRejectsMismatch(t *testing.T) {
	signer := newBlockSigner(t)
	b := Block{
		Header: Header{TransactionsRoot: types.Hash{1}},
		Transactions: []any{blockTx(t, signer, 0, 30)},
	}
	if err := ValidateTransactionsRoot(b); err != ErrTransactionsRootMismatch {
		t.Fatalf("expected transactions root mismatch, got %v", err)
	}
}

func TestExecuteBlockCommitsAllTransactions(t *testing.T) {
	signer := newBlockSigner(t)
	s := state.New()
	s.Set(types.Address{1}, state.Account{Balance: 100, Nonce: 0})

	b := Block{
		Header: Header{Version: 1, ChainID: 1001, Height: 1},
		Transactions: []any{
			blockTx(t, signer, 0, 30),
			blockTx(t, signer, 1, 20),
		},
	}
	expectedRoot := func() types.Hash {
		working := s.Snapshot()
		_ = state.ApplyTransaction(working, b.Transactions[0].(transaction.Transaction), blockRules(signer.PublicKey()).Transaction)
		_ = state.ApplyTransaction(working, b.Transactions[1].(transaction.Transaction), blockRules(signer.PublicKey()).Transaction)
		return working.Root()
	}()
	b.Header.StateRoot = expectedRoot

	if err := ExecuteBlock(s, b, 1, types.Hash{}, blockRules(signer.PublicKey())); err != nil {
		t.Fatal(err)
	}
	if s.Root() != expectedRoot {
		t.Fatalf("unexpected committed root: %s", s.Root())
	}
	sender, _ := s.Get(types.Address{1})
	recipient, _ := s.Get(types.Address{2})
	if sender.Balance != 50 || sender.Nonce != 2 { t.Fatalf("unexpected sender: %+v", sender) }
	if recipient.Balance != 50 { t.Fatalf("unexpected recipient: %+v", recipient) }
}

func TestExecuteBlockRejectsStateRootMismatch(t *testing.T) {
	signer := newBlockSigner(t)
	s := state.New()
	s.Set(types.Address{1}, state.Account{Balance: 100, Nonce: 0})

	b := Block{
		Header: Header{Version: 1, ChainID: 1001, Height: 1, StateRoot: types.Hash{1}},
		Transactions: []any{blockTx(t, signer, 0, 30)},
	}
	if err := ExecuteBlock(s, b, 1, types.Hash{}, blockRules(signer.PublicKey())); err != ErrStateRootMismatch {
		t.Fatalf("expected state root mismatch, got %v", err)
	}
	sender, _ := s.Get(types.Address{1})
	if sender.Balance != 100 || sender.Nonce != 0 {
		t.Fatalf("state committed despite root mismatch: %+v", sender)
	}
}

func TestExecuteBlockRollsBackOnTransactionFailure(t *testing.T) {
	signer := newBlockSigner(t)
	s := state.New()
	s.Set(types.Address{1}, state.Account{Balance: 40, Nonce: 0})
	b := Block{
		Header: Header{Version: 1, ChainID: 1001, Height: 1},
		Transactions: []any{blockTx(t, signer, 0, 30), blockTx(t, signer, 1, 30)},
	}
	if err := ExecuteBlock(s, b, 1, types.Hash{}, blockRules(signer.PublicKey())); err == nil {
		t.Fatal("expected block execution failure")
	}
	sender, _ := s.Get(types.Address{1})
	if sender.Balance != 40 || sender.Nonce != 0 { t.Fatalf("block partially committed: %+v", sender) }
	if _, ok := s.Get(types.Address{2}); ok { t.Fatal("recipient created after rollback") }
}

func TestExecuteBlockRejectsBadParent(t *testing.T) {
	s := state.New()
	b := Block{Header: Header{Version: 1, ChainID: 1001, Height: 2}}
	var parent types.Hash
	parent[0] = 1
	if err := ExecuteBlock(s, b, 2, parent, blockRules(nil)); err != ErrPreviousHash {
		t.Fatalf("expected previous hash error, got %v", err)
	}
}
