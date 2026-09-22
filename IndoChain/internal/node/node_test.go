package node

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/genesis/devnet"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/transaction"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

func TestNewDevnetInitializesGenesis(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store)
	if err != nil { t.Fatal(err) }
	if n.Head.Header.Height != 0 { t.Fatalf("head height = %d, want 0", n.Head.Header.Height) }
	if n.HeadHash == (types.Hash{}) { t.Fatal("genesis head hash must be non-zero") }
	storedBlock, storedHash, err := store.Head()
	if err != nil { t.Fatal(err) }
	if storedBlock.Header.Height != n.Head.Header.Height || storedHash != n.HeadHash { t.Fatal("store head does not match node head") }
	loadedState, err := store.LoadState()
	if err != nil { t.Fatal(err) }
	if loadedState.Root() != n.State.Root() { t.Fatal("stored state does not match node state") }
}

func TestNewDevnetRejectsNilStore(t *testing.T) {
	if _, err := NewDevnet(nil); err != ErrNilStore { t.Fatalf("error = %v, want %v", err, ErrNilStore) }
}

func TestImportBlockCommitsExecutedState(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store)
	if err != nil { t.Fatal(err) }
	seed := make([]byte, 32)
	seed[0] = 7
	keyPair, err := crypto.NewEd25519KeyPair(seed)
	if err != nil { t.Fatal(err) }
	signer, err := crypto.NewEd25519Signer(keyPair.PrivateKey)
	if err != nil { t.Fatal(err) }

	sender := types.Address([]byte("sender"))
	recipient := types.Address([]byte("recipient"))
	n.State.Set(sender, state.Account{Balance: 100, Nonce: 0})
	tx := transaction.Transaction{
		Version: devnet.ProtocolVersion, ChainID: devnet.ChainID, Nonce: 0,
		Sender: sender, Recipient: recipient, Value: 25, GasLimit: 100,
	}
	sig, err := transaction.Sign(tx, signer)
	if err != nil { t.Fatal(err) }
	tx.Signature = sig

	rules := block.ExecutionRules{
		ChainID: devnet.ChainID, ProtocolVersion: devnet.ProtocolVersion,
		Transaction: state.ExecutionRules{
			Validation: transaction.ValidationRules{
				ProtocolVersion: devnet.ProtocolVersion, ChainID: devnet.ChainID,
				RequireSender: true, RequireRecipient: true, RequireSignature: true, MinGasLimit: 1,
			},
			PublicKey: signer.PublicKey(),
		},
	}
	working := n.State.Snapshot()
	if err := state.ApplyTransaction(working, tx, rules.Transaction); err != nil { t.Fatal(err) }
	next := block.Block{
		Header: block.Header{
			Version: devnet.ProtocolVersion, ChainID: devnet.ChainID, Height: 1,
			Timestamp: n.Head.Header.Timestamp + 1, PreviousHash: n.HeadHash, StateRoot: working.Root(),
		},
		Transactions: []any{tx},
	}
	next.Header.TransactionsRoot, err = block.TransactionsRoot(next.Transactions)
	if err != nil { t.Fatal(err) }
	if err := n.ImportBlock(next, rules); err != nil { t.Fatal(err) }
	if n.Head.Header.Height != 1 { t.Fatalf("head height = %d, want 1", n.Head.Header.Height) }
	if got, _ := n.State.Get(recipient); got.Balance != 25 { t.Fatalf("recipient balance = %d, want 25", got.Balance) }
	if got, _ := n.State.Get(sender); got.Balance != 75 { t.Fatalf("sender balance = %d, want 75", got.Balance) }
	loaded, err := store.LoadState()
	if err != nil || loaded.Root() != n.State.Root() { t.Fatal("stored state does not match imported state") }
}

func TestImportBlockRejectsWrongParentWithoutMutation(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store)
	if err != nil { t.Fatal(err) }
	before := n.State.Root()
	b := block.Block{Header: block.Header{
		Version: devnet.ProtocolVersion, ChainID: devnet.ChainID, Height: 1,
		Timestamp: n.Head.Header.Timestamp + 1, PreviousHash: types.Hash{9},
	}}
	rules := block.ExecutionRules{ChainID: devnet.ChainID, ProtocolVersion: devnet.ProtocolVersion}
	if err := n.ImportBlock(b, rules); err != block.ErrPreviousHash { t.Fatalf("error = %v, want %v", err, block.ErrPreviousHash) }
	if n.Head.Header.Height != 0 || n.State.Root() != before { t.Fatal("node mutated after rejecting invalid parent") }
}
