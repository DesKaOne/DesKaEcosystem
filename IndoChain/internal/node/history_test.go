package node

import (
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/genesis/devnet"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/transaction"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

func makeTestBlock(t *testing.T, n *Node, signer *crypto.Ed25519Signer, value uint64, recipient types.Address) block.Block {
	t.Helper()

	sender := types.Address([]byte("history-sender"))
	n.State.Set(sender, state.Account{Balance: 100, Nonce: 0})
	tx := transaction.Transaction{
		Version: devnet.ProtocolVersion,
		ChainID: devnet.ChainID,
		Nonce: 0,
		Sender: sender,
		Recipient: recipient,
		Value: value,
		GasLimit: 100,
	}
	sig, err := transaction.Sign(tx, signer)
	if err != nil {
		t.Fatal(err)
	}
	tx.Signature = sig

	rules, err := n.Config.BlockRules(signer.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	working := n.State.Snapshot()
	if err := state.ApplyTransaction(working, tx, rules.Transaction); err != nil {
		t.Fatal(err)
	}

	next := block.Block{
		Header: block.Header{
			Version:       devnet.ProtocolVersion,
			ChainID:       devnet.ChainID,
			Height:        n.Head.Header.Height + 1,
			Timestamp:     n.Head.Header.Timestamp + 1,
			PreviousHash:  n.HeadHash,
			StateRoot:     working.Root(),
		},
		Transactions: []any{tx},
	}
	next.Header.TransactionsRoot, err = block.TransactionsRoot(next.Transactions)
	if err != nil {
		t.Fatal(err)
	}
	return next
}

func newHistorySigner(t *testing.T) *crypto.Ed25519Signer {
	t.Helper()
	seed := make([]byte, 32)
	seed[0] = 21
	keyPair, err := crypto.NewEd25519KeyPair(seed)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := crypto.NewEd25519Signer(keyPair.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

func TestOpenDevnetRejectsMissingHistoricalBlock(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store)
	if err != nil {
		t.Fatal(err)
	}
	signer := newHistorySigner(t)
	b1 := makeTestBlock(t, n, signer, 10, types.Address([]byte("history-recipient")))
	if err := n.ImportBlock(b1, signer.PublicKey()); err != nil {
		t.Fatal(err)
	}

	if err := store.SaveBlock(b1, n.HeadHash); err != nil {
		t.Fatal(err)
	}
	delete(store.BlocksForTest(), 0)

	_, err = OpenDevnet(store)
	if !errors.Is(err, ErrHistoryMismatch) {
		t.Fatalf("error = %v, want %v", err, ErrHistoryMismatch)
	}
}

func TestOpenDevnetRejectsBrokenHistoricalParentHash(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store)
	if err != nil {
		t.Fatal(err)
	}
	signer := newHistorySigner(t)
	b1 := makeTestBlock(t, n, signer, 10, types.Address([]byte("history-recipient")))
	if err := n.ImportBlock(b1, signer.PublicKey()); err != nil {
		t.Fatal(err)
	}

	bad := b1
	bad.Header.PreviousHash = types.Hash{7}
	hash, err := block.Hash(bad)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveBlock(bad, hash); err != nil {
		t.Fatal(err)
	}

	_, err = OpenDevnet(store)
	if !errors.Is(err, ErrHistoryMismatch) {
		t.Fatalf("error = %v, want %v", err, ErrHistoryMismatch)
	}
}
