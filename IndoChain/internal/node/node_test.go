package node

import (
	"errors"
	"reflect"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/genesis/devnet"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/transaction"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

var errCommitFailed = errors.New("commit failed")

type failingCommitStore struct {
	*storage.MemoryStore
	failCommit bool
}

func (s *failingCommitStore) CommitBlockState(b block.Block, hash types.Hash, st *state.State) error {
	if s.failCommit {
		return errCommitFailed
	}
	return s.MemoryStore.CommitBlockState(b, hash, st)
}

func TestNewDevnetInitializesGenesis(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store)
	if err != nil {
		t.Fatal(err)
	}
	if n.Config.NetworkProfile != devnet.NetworkProfile || n.Config.ChainID != devnet.ChainID || n.Config.ProtocolVersion != devnet.ProtocolVersion {
		t.Fatal("node config does not match Devnet genesis")
	}
	if n.Head.Header.Height != 0 {
		t.Fatalf("head height = %d, want 0", n.Head.Header.Height)
	}
	if n.HeadHash == (types.Hash{}) {
		t.Fatal("genesis head hash must be non-zero")
	}
	storedBlock, storedHash, err := store.Head()
	if err != nil {
		t.Fatal(err)
	}
	if storedBlock.Header.Height != n.Head.Header.Height || storedHash != n.HeadHash {
		t.Fatal("store head does not match node head")
	}
	loadedState, err := store.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if loadedState.Root() != n.State.Root() {
		t.Fatal("stored state does not match node state")
	}
}

func TestNewDevnetRejectsNilStore(t *testing.T) {
	if _, err := NewDevnet(nil); err != ErrNilStore {
		t.Fatalf("error = %v, want %v", err, ErrNilStore)
	}
}

func TestOpenDevnetInitializesEmptyStore(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := OpenDevnet(store)
	if err != nil {
		t.Fatal(err)
	}
	if n.Head.Header.Height != 0 || n.HeadHash == (types.Hash{}) {
		t.Fatal("empty store was not initialized from genesis")
	}
}

func TestOpenDevnetRecoversExistingState(t *testing.T) {
	store := storage.NewMemoryStore()
	original, err := NewDevnet(store)
	if err != nil {
		t.Fatal(err)
	}

	recovered, err := OpenDevnet(store)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Head.Header.Height != original.Head.Header.Height || recovered.HeadHash != original.HeadHash {
		t.Fatal("recovered head does not match stored head")
	}
	if recovered.State.Root() != original.State.Root() {
		t.Fatal("recovered state does not match stored state")
	}
	if recovered.Config != original.Config {
		t.Fatal("recovered config does not match original config")
	}
}

func TestOpenDevnetRejectsCorruptHeadHash(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store)
	if err != nil {
		t.Fatal(err)
	}

	blockAtHead, _, err := store.Head()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveBlock(blockAtHead, types.Hash{9}); err != nil {
		t.Fatal(err)
	}

	if _, err := OpenDevnet(store); err != ErrBlockHashMismatch {
		t.Fatalf("error = %v, want %v", err, ErrBlockHashMismatch)
	}
	if n.HeadHash == (types.Hash{}) {
		t.Fatal("test setup produced invalid node hash")
	}
}

func TestOpenDevnetRejectsCorruptStateRoot(t *testing.T) {
	store := storage.NewMemoryStore()
	_, err := NewDevnet(store)
	if err != nil {
		t.Fatal(err)
	}

	b, _, err := store.Head()
	if err != nil {
		t.Fatal(err)
	}
	b.Header.StateRoot = types.Hash{8}
	corruptHash, err := block.Hash(b)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveBlock(b, corruptHash); err != nil {
		t.Fatal(err)
	}

	if _, err := OpenDevnet(store); err != ErrStateRootMismatch {
		t.Fatalf("error = %v, want %v", err, ErrStateRootMismatch)
	}
}

func TestOpenDevnetRecoversFileStore(t *testing.T) {
	path := t.TempDir() + "/chain.gob"
	store, err := storage.NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}

	original, err := NewDevnet(store)
	if err != nil {
		t.Fatal(err)
	}

	reopenedStore, err := storage.NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := OpenDevnet(reopenedStore)
	if err != nil {
		t.Fatal(err)
	}

	if recovered.Head.Header.Height != original.Head.Header.Height || recovered.HeadHash != original.HeadHash {
		t.Fatal("recovered file-store head does not match original")
	}
	if recovered.State.Root() != original.State.Root() {
		t.Fatal("recovered file-store state does not match original")
	}
}

func TestImportBlockCommitsExecutedState(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store)
	if err != nil {
		t.Fatal(err)
	}
	seed := make([]byte, 32)
	seed[0] = 7
	keyPair, err := crypto.NewEd25519KeyPair(seed)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := crypto.NewEd25519Signer(keyPair.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}

	sender := types.Address([]byte("sender"))
	recipient := types.Address([]byte("recipient"))
	n.State.Set(sender, state.Account{Balance: 100, Nonce: 0})
	tx := transaction.Transaction{
		Version: devnet.ProtocolVersion, ChainID: devnet.ChainID, Nonce: 0,
		Sender: sender, Recipient: recipient, Value: 25, GasLimit: 100,
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
			Version: devnet.ProtocolVersion, ChainID: devnet.ChainID, Height: 1,
			Timestamp: n.Head.Header.Timestamp + 1, PreviousHash: n.HeadHash, StateRoot: working.Root(),
		},
		Transactions: []any{tx},
	}
	next.Header.TransactionsRoot, err = block.TransactionsRoot(next.Transactions)
	if err != nil {
		t.Fatal(err)
	}
	if err := n.ImportBlock(next, signer.PublicKey()); err != nil {
		t.Fatal(err)
	}
	if n.Head.Header.Height != 1 {
		t.Fatalf("head height = %d, want 1", n.Head.Header.Height)
	}
	if got, _ := n.State.Get(recipient); got.Balance != 25 {
		t.Fatalf("recipient balance = %d, want 25", got.Balance)
	}
	if got, _ := n.State.Get(sender); got.Balance != 75 {
		t.Fatalf("sender balance = %d, want 75", got.Balance)
	}
	loaded, err := store.LoadState()
	if err != nil || loaded.Root() != n.State.Root() {
		t.Fatal("stored state does not match imported state")
	}
}

func TestOpenDevnetRecoversFileStoreAfterImportedBlock(t *testing.T) {
	path := t.TempDir() + "/chain.gob"
	store, err := storage.NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	n, err := NewDevnet(store)
	if err != nil {
		t.Fatal(err)
	}

	seed := make([]byte, 32)
	seed[0] = 9
	keyPair, err := crypto.NewEd25519KeyPair(seed)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := crypto.NewEd25519Signer(keyPair.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}

	sender := types.Address([]byte("persistent-sender"))
	recipient := types.Address([]byte("persistent-recipient"))
	n.State.Set(sender, state.Account{Balance: 100, Nonce: 0})

	tx := transaction.Transaction{
		Version: devnet.ProtocolVersion,
		ChainID: devnet.ChainID,
		Nonce: 0,
		Sender: sender,
		Recipient: recipient,
		Value: 40,
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
			Version: devnet.ProtocolVersion,
			ChainID: devnet.ChainID,
			Height: 1,
			Timestamp: n.Head.Header.Timestamp + 1,
			PreviousHash: n.HeadHash,
			StateRoot: working.Root(),
		},
		Transactions: []any{tx},
	}
	next.Header.TransactionsRoot, err = block.TransactionsRoot(next.Transactions)
	if err != nil {
		t.Fatal(err)
	}
	if err := n.ImportBlock(next, signer.PublicKey()); err != nil {
		t.Fatal(err)
	}

	storedHead, storedHash, err := store.Head()
	if err != nil {
		t.Fatal(err)
	}
	storedState, err := store.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	reopenedStore, err := storage.NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := OpenDevnet(reopenedStore)
	if err != nil {
		t.Fatal(err)
	}

	if recovered.Head.Header.Height != 1 {
		t.Fatalf("recovered head height = %d, want 1", recovered.Head.Header.Height)
	}
	if recovered.HeadHash != storedHash {
		t.Fatal("recovered head hash does not match persisted hash")
	}
	if recovered.Head.Header.Height != storedHead.Header.Height {
		t.Fatal("recovered head does not match persisted head")
	}
	if recovered.State.Root() != storedState.Root() {
		t.Fatal("recovered state root does not match persisted state")
	}
	if got, ok := recovered.State.Get(recipient); !ok || got.Balance != 40 {
		t.Fatalf("recovered recipient balance = %d, want 40", got.Balance)
	}
	if got, ok := recovered.State.Get(sender); !ok || got.Balance != 60 || got.Nonce != 1 {
		t.Fatalf("recovered sender = %+v, want balance 60 nonce 1", got)
	}
}

func TestImportBlockRejectsWrongParentWithoutMutation(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store)
	if err != nil {
		t.Fatal(err)
	}
	before := n.State.Root()
	b := block.Block{Header: block.Header{
		Version: devnet.ProtocolVersion, ChainID: devnet.ChainID, Height: 1,
		Timestamp: n.Head.Header.Timestamp + 1, PreviousHash: types.Hash{9},
	}}
	if err := n.ImportBlock(b, nil); err != block.ErrPreviousHash {
		t.Fatalf("error = %v, want %v", err, block.ErrPreviousHash)
	}
	if n.Head.Header.Height != 0 || n.State.Root() != before {
		t.Fatal("node mutated after rejecting invalid parent")
	}
}

func TestImportBlockCommitFailureWithoutMutation(t *testing.T) {
	store := &failingCommitStore{MemoryStore: storage.NewMemoryStore()}
	n, err := NewDevnet(store)
	if err != nil {
		t.Fatal(err)
	}

	seed := make([]byte, 32)
	seed[0] = 11
	keyPair, err := crypto.NewEd25519KeyPair(seed)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := crypto.NewEd25519Signer(keyPair.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}

	sender := types.Address([]byte("commit-failure-sender"))
	recipient := types.Address([]byte("commit-failure-recipient"))
	n.State.Set(sender, state.Account{Balance: 100, Nonce: 0})
	beforeHead := n.Head
	beforeHash := n.HeadHash
	beforeStateRoot := n.State.Root()
	storedBeforeHead, storedBeforeHash, err := store.Head()
	if err != nil {
		t.Fatal(err)
	}
	storedBeforeState, err := store.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	tx := transaction.Transaction{
		Version: devnet.ProtocolVersion,
		ChainID: devnet.ChainID,
		Nonce: 0,
		Sender: sender,
		Recipient: recipient,
		Value: 30,
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
			Version: devnet.ProtocolVersion,
			ChainID: devnet.ChainID,
			Height: 1,
			Timestamp: n.Head.Header.Timestamp + 1,
			PreviousHash: n.HeadHash,
			StateRoot: working.Root(),
		},
		Transactions: []any{tx},
	}
	next.Header.TransactionsRoot, err = block.TransactionsRoot(next.Transactions)
	if err != nil {
		t.Fatal(err)
	}

	store.failCommit = true
	if err := n.ImportBlock(next, signer.PublicKey()); !errors.Is(err, errCommitFailed) {
		t.Fatalf("error = %v, want %v", err, errCommitFailed)
	}

	if !reflect.DeepEqual(n.Head, beforeHead) || n.HeadHash != beforeHash || n.State.Root() != beforeStateRoot {
		t.Fatal("node mutated after failed block commit")
	}
	storedAfterHead, storedAfterHash, err := store.Head()
	if err != nil {
		t.Fatal(err)
	}
	storedAfterState, err := store.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(storedAfterHead, storedBeforeHead) || storedAfterHash != storedBeforeHash {
		t.Fatal("store head mutated after failed block commit")
	}
	if storedAfterState.Root() != storedBeforeState.Root() {
		t.Fatal("store state mutated after failed block commit")
	}
}
