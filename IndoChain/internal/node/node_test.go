package node

import (
	"errors"
	"reflect"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/genesis/devnet"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
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
	recipients := []types.Address{
		types.Address([]byte("persistent-recipient-1")),
		types.Address([]byte("persistent-recipient-2")),
	}
	n.State.Set(sender, state.Account{Balance: 100, Nonce: 0})

	tx := transaction.Transaction{
		Version: devnet.ProtocolVersion,
		ChainID: devnet.ChainID,
		Nonce: 0,
		Sender: sender,
		Recipient: recipients[0],
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
			Version:       devnet.ProtocolVersion,
			ChainID:       devnet.ChainID,
			Height:        1,
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
	if err := n.ImportBlock(next, signer.PublicKey()); err != nil {
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
	if recovered.HeadHash != n.HeadHash {
		t.Fatal("recovered head hash does not match persisted hash")
	}
	if recovered.State.Root() != n.State.Root() {
		t.Fatal("recovered state root does not match persisted state")
	}
	if got, ok := recovered.State.Get(recipients[0]); !ok || got.Balance != 40 {
		t.Fatalf("recovered recipient balance = %d, want 40", got.Balance)
	}
	if got, ok := recovered.State.Get(sender); !ok || got.Balance != 60 || got.Nonce != 1 {
		t.Fatalf("recovered sender = %+v, want balance 60 nonce 1", got)
	}

	tx2 := transaction.Transaction{
		Version:   devnet.ProtocolVersion,
		ChainID:   devnet.ChainID,
		Nonce:     1,
		Sender:    sender,
		Recipient: recipients[1],
		Value:     25,
		GasLimit:  100,
	}
	sig2, err := transaction.Sign(tx2, signer)
	if err != nil {
		t.Fatal(err)
	}
	tx2.Signature = sig2

	rules2, err := recovered.Config.BlockRules(signer.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	working2 := recovered.State.Snapshot()
	if err := state.ApplyTransaction(working2, tx2, rules2.Transaction); err != nil {
		t.Fatal(err)
	}

	next2 := block.Block{
		Header: block.Header{
			Version:       devnet.ProtocolVersion,
			ChainID:       devnet.ChainID,
			Height:        2,
			Timestamp:     recovered.Head.Header.Timestamp + 1,
			PreviousHash:  recovered.HeadHash,
			StateRoot:     working2.Root(),
		},
		Transactions: []any{tx2},
	}
	next2.Header.TransactionsRoot, err = block.TransactionsRoot(next2.Transactions)
	if err != nil {
		t.Fatal(err)
	}
	if err := recovered.ImportBlock(next2, signer.PublicKey()); err != nil {
		t.Fatal(err)
	}

	if recovered.Head.Header.Height != 2 {
		t.Fatalf("post-recovery head height = %d, want 2", recovered.Head.Header.Height)
	}
	if got, ok := recovered.State.Get(sender); !ok || got.Balance != 35 || got.Nonce != 2 {
		t.Fatalf("post-recovery sender = %+v, want balance 35 nonce 2", got)
	}
	if got, ok := recovered.State.Get(recipients[1]); !ok || got.Balance != 25 {
		t.Fatalf("post-recovery recipient balance = %d, want 25", got.Balance)
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
		Version:       devnet.ProtocolVersion,
		ChainID:       devnet.ChainID,
		Nonce:         0,
		Sender:        sender,
		Recipient:     recipient,
		Value:         30,
		GasLimit:      100,
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
			Height:        1,
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


type senderAuthorityResolver struct { publicKey []byte }
func (r senderAuthorityResolver) PublicKeyForSender(sender []byte) ([]byte, error) { return append([]byte(nil), r.publicKey...), nil }

func TestImportBlockWithAuthorityResolvesSenderKey(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store); if err != nil { t.Fatal(err) }
	seed := make([]byte, 32); seed[0] = 17
	keyPair, err := crypto.NewEd25519KeyPair(seed); if err != nil { t.Fatal(err) }
	signer, err := crypto.NewEd25519Signer(keyPair.PrivateKey); if err != nil { t.Fatal(err) }
	sender := types.Address([]byte("resolver-sender")); recipient := types.Address([]byte("resolver-recipient"))
	n.State.Set(sender, state.Account{Balance: 50, Nonce: 0})
	tx := transaction.Transaction{Version: devnet.ProtocolVersion, ChainID: devnet.ChainID, Nonce: 0, Sender: sender, Recipient: recipient, Value: 10, GasLimit: 100}
	tx.Signature, err = transaction.Sign(tx, signer); if err != nil { t.Fatal(err) }
	rules, err := n.Config.BlockRules(nil); if err != nil { t.Fatal(err) }
	rules.Transaction.PublicKeyResolver = senderAuthorityResolver{publicKey: signer.PublicKey()}
	working := n.State.Snapshot(); if err := state.ApplyTransaction(working, tx, rules.Transaction); err != nil { t.Fatal(err) }
	b := block.Block{Header: block.Header{Version: devnet.ProtocolVersion, ChainID: devnet.ChainID, Height: 1, Timestamp: n.Head.Header.Timestamp + 1, PreviousHash: n.HeadHash, StateRoot: working.Root()}, Transactions: []any{tx}}
	b.Header.TransactionsRoot, err = block.TransactionsRoot(b.Transactions); if err != nil { t.Fatal(err) }
	if err := n.ImportBlockWithAuthority(b, senderAuthorityResolver{publicKey: signer.PublicKey()}); err != nil { t.Fatal(err) }
	if got, ok := n.State.Get(recipient); !ok || got.Balance != 10 { t.Fatalf("recipient = %+v, want balance 10", got) }
}


type validatorAuthorityResolver struct { publicKey []byte }
func (r validatorAuthorityResolver) PublicKeyForValidator(validatorID []byte) ([]byte, error) { return append([]byte(nil), r.publicKey...), nil }

type failingAuthorityResolver struct { err error }
func (r failingAuthorityResolver) PublicKeyForValidator([]byte) ([]byte, error) { return nil, r.err }
func (r failingAuthorityResolver) PublicKeyForSender([]byte) ([]byte, error) { return nil, r.err }

func TestCommitFinalizedBlockRejectsMissingResolversWithoutMutation(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store); if err != nil { t.Fatal(err) }
	beforeHead, beforeHash, beforeRoot := n.Head, n.HeadHash, n.State.Root()
	ctx := consensus.BlockProductionContext{State: consensus.RoundState{ProtocolVersion: devnet.ProtocolVersion, ChainID: devnet.ChainID, Epoch: 1, Height: 0, Round: 0, Phase: consensus.PhaseProposal}, PreviousHash: n.HeadHash, Proposer: []byte("validator")}
	if err := n.CommitFinalizedBlock(ctx, block.Block{}, consensus.FinalityCertificate{}, consensus.ValidatorSet{}, consensus.VotingPowerSet{}, nil, nil); err == nil { t.Fatal("expected missing resolver error") }
	if !reflect.DeepEqual(n.Head, beforeHead) || n.HeadHash != beforeHash || n.State.Root() != beforeRoot { t.Fatal("node mutated after missing resolver rejection") }
}

func TestCommitFinalizedBlockRejectsConsensusContextMismatch(t *testing.T) {
	store := storage.NewMemoryStore(); n, err := NewDevnet(store); if err != nil { t.Fatal(err) }
	beforeHead, beforeHash, beforeRoot := n.Head, n.HeadHash, n.State.Root()
	ctx := consensus.BlockProductionContext{State: consensus.RoundState{ProtocolVersion: devnet.ProtocolVersion, ChainID: devnet.ChainID, Epoch: 1, Height: 1, Round: 0, Phase: consensus.PhaseProposal}, PreviousHash: n.HeadHash, Proposer: []byte("validator")}
	resolver := validatorAuthorityResolver{publicKey: []byte("key")}
	if err := n.CommitFinalizedBlock(ctx, block.Block{}, consensus.FinalityCertificate{}, consensus.ValidatorSet{}, consensus.VotingPowerSet{}, resolver, senderAuthorityResolver{publicKey: []byte("key")}); err != ErrConsensusContextMismatch { t.Fatalf("error = %v, want %v", err, ErrConsensusContextMismatch) }
	if !reflect.DeepEqual(n.Head, beforeHead) || n.HeadHash != beforeHash || n.State.Root() != beforeRoot { t.Fatal("node mutated after context mismatch") }
}

func finalizedBlockFixture(t *testing.T, store storage.ChainStore) (*Node, consensus.BlockProductionContext, block.Block, consensus.FinalityCertificate, validatorAuthorityResolver, senderAuthorityResolver, types.Address) {
	t.Helper()
	n, err := NewDevnet(store); if err != nil { t.Fatal(err) }
	seed := make([]byte, 32); seed[0] = 23
	keyPair, err := crypto.NewEd25519KeyPair(seed); if err != nil { t.Fatal(err) }
	signer, err := crypto.NewEd25519Signer(keyPair.PrivateKey); if err != nil { t.Fatal(err) }
	validatorID := []byte("fixture-validator")
	validators, err := consensus.NewValidatorSet([][]byte{validatorID}); if err != nil { t.Fatal(err) }
	power, err := consensus.NewVotingPowerSet([]consensus.ValidatorVotingPower{{ValidatorID: validatorID, Power: 1}}); if err != nil { t.Fatal(err) }
	ctx := consensus.BlockProductionContext{State: consensus.RoundState{ProtocolVersion: devnet.ProtocolVersion, ChainID: devnet.ChainID, Epoch: 1, Height: 0, Round: 0, Phase: consensus.PhaseProposal}, PreviousHash: n.HeadHash, Proposer: validatorID}
	sender := types.Address([]byte("fixture-sender")); recipient := types.Address([]byte("fixture-recipient"))
	n.State.Set(sender, state.Account{Balance: 60, Nonce: 0})
	tx := transaction.Transaction{Version: devnet.ProtocolVersion, ChainID: devnet.ChainID, Nonce: 0, Sender: sender, Recipient: recipient, Value: 20, GasLimit: 100}
	tx.Signature, err = transaction.Sign(tx, signer); if err != nil { t.Fatal(err) }
	rules, err := n.Config.BlockRules(signer.PublicKey()); if err != nil { t.Fatal(err) }
	working := n.State.Snapshot(); if err := state.ApplyTransaction(working, tx, rules.Transaction); err != nil { t.Fatal(err) }
	candidate := block.Block{Header: block.Header{Version: devnet.ProtocolVersion, ChainID: devnet.ChainID, Height: 1, Timestamp: n.Head.Header.Timestamp + 1, PreviousHash: n.HeadHash, StateRoot: working.Root(), Proposer: validatorID}, Transactions: []any{tx}}
	candidate.Header.TransactionsRoot, err = block.TransactionsRoot(candidate.Transactions); if err != nil { t.Fatal(err) }
	payload, err := consensus.ValidateProducedBlock(ctx, candidate); if err != nil { t.Fatal(err) }
	vote := consensus.Message{ProtocolVersion: devnet.ProtocolVersion, ChainID: devnet.ChainID, Epoch: 1, Height: 0, Round: 0, Sender: validatorID, Type: consensus.MessageTypeVote, Payload: payload[:]}
	certificate, err := consensus.NewFinalityCertificate(ctx.State, validators, power, consensus.QuorumThreshold{Numerator: 1, Denominator: 1}, payload[:], []consensus.Message{vote}); if err != nil { t.Fatal(err) }
	return n, ctx, candidate, certificate, validatorAuthorityResolver{publicKey: signer.PublicKey()}, senderAuthorityResolver{publicKey: signer.PublicKey()}, recipient
}

func TestCommitFinalizedBlockRejectsValidatorAuthorityFailureWithoutMutation(t *testing.T) {
	n, ctx, candidate, certificate, _, senderResolver, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	beforeHead, beforeHash, beforeRoot := n.Head, n.HeadHash, n.State.Root()
	errExpected := errors.New("validator authority failure")
	err := n.CommitFinalizedBlock(ctx, candidate, certificate, mustValidatorSet(t, certificate), mustVotingPowerSet(t, certificate), failingAuthorityResolver{err: errExpected}, senderResolver)
	if !errors.Is(err, errExpected) { t.Fatalf("error = %v, want %v", err, errExpected) }
	if !reflect.DeepEqual(n.Head, beforeHead) || n.HeadHash != beforeHash || n.State.Root() != beforeRoot { t.Fatal("node mutated after validator authority failure") }
}

func TestCommitFinalizedBlockRejectsFinalityMismatchWithoutMutation(t *testing.T) {
	n, ctx, candidate, certificate, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	certificate.Payload = []byte("wrong-finality-payload")
	beforeHead, beforeHash, beforeRoot := n.Head, n.HeadHash, n.State.Root()
	if err := n.CommitFinalizedBlock(ctx, candidate, certificate, mustValidatorSet(t, certificate), mustVotingPowerSet(t, certificate), validatorResolver, senderResolver); err == nil { t.Fatal("expected finality mismatch") }
	if !reflect.DeepEqual(n.Head, beforeHead) || n.HeadHash != beforeHash || n.State.Root() != beforeRoot { t.Fatal("node mutated after finality mismatch") }
}

func TestCommitFinalizedBlockRejectsExecutionAuthorityFailureWithoutMutation(t *testing.T) {
	n, ctx, candidate, certificate, validatorResolver, _, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	beforeHead, beforeHash, beforeRoot := n.Head, n.HeadHash, n.State.Root()
	errExpected := errors.New("sender authority failure")
	err := n.CommitFinalizedBlock(ctx, candidate, certificate, mustValidatorSet(t, certificate), mustVotingPowerSet(t, certificate), validatorResolver, failingAuthorityResolver{err: errExpected})
	if !errors.Is(err, errExpected) { t.Fatalf("error = %v, want %v", err, errExpected) }
	if !reflect.DeepEqual(n.Head, beforeHead) || n.HeadHash != beforeHash || n.State.Root() != beforeRoot { t.Fatal("node mutated after sender authority failure") }
}

func TestCommitFinalizedBlockRejectsStoreFailureWithoutMutation(t *testing.T) {
	store := &failingCommitStore{MemoryStore: storage.NewMemoryStore()}
	n, ctx, candidate, certificate, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, store)
	beforeHead, beforeHash, beforeRoot := n.Head, n.HeadHash, n.State.Root()
	store.failCommit = true
	if err := n.CommitFinalizedBlock(ctx, candidate, certificate, mustValidatorSet(t, certificate), mustVotingPowerSet(t, certificate), validatorResolver, senderResolver); !errors.Is(err, errCommitFailed) { t.Fatalf("error = %v, want %v", err, errCommitFailed) }
	if !reflect.DeepEqual(n.Head, beforeHead) || n.HeadHash != beforeHash || n.State.Root() != beforeRoot { t.Fatal("node mutated after store failure") }
}


func TestCommitFinalizedBlockRejectsTransactionExecutionFailureWithoutMutation(t *testing.T) {
	n, ctx, candidate, _, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	tx := candidate.Transactions[0].(transaction.Transaction)
	tx.Signature = []byte("invalid-signature")
	candidate.Transactions[0] = tx
	var err error
	candidate.Header.TransactionsRoot, err = block.TransactionsRoot(candidate.Transactions); if err != nil { t.Fatal(err) }
	payload, err := consensus.ValidateProducedBlock(ctx, candidate); if err != nil { t.Fatal(err) }
	validatorID := candidate.Header.Proposer
	validators, err := consensus.NewValidatorSet([][]byte{validatorID}); if err != nil { t.Fatal(err) }
	power, err := consensus.NewVotingPowerSet([]consensus.ValidatorVotingPower{{ValidatorID: validatorID, Power: 1}}); if err != nil { t.Fatal(err) }
	vote := consensus.Message{ProtocolVersion: devnet.ProtocolVersion, ChainID: devnet.ChainID, Epoch: 1, Height: 0, Round: 0, Sender: validatorID, Type: consensus.MessageTypeVote, Payload: payload[:]}
	certificate, err := consensus.NewFinalityCertificate(ctx.State, validators, power, consensus.QuorumThreshold{Numerator: 1, Denominator: 1}, payload[:], []consensus.Message{vote}); if err != nil { t.Fatal(err) }
	beforeHead, beforeHash, beforeRoot := n.Head, n.HeadHash, n.State.Root()
	if err := n.CommitFinalizedBlock(ctx, candidate, certificate, validatorResolver, power, validatorResolver, senderResolver); err == nil { t.Fatal("expected transaction execution failure") }
	if !reflect.DeepEqual(n.Head, beforeHead) || n.HeadHash != beforeHash || n.State.Root() != beforeRoot { t.Fatal("node mutated after transaction execution failure") }
}

func mustValidatorSet(t *testing.T, certificate consensus.FinalityCertificate) consensus.ValidatorSet {
	t.Helper(); validators, err := consensus.NewValidatorSet([][]byte{certificate.Votes[0].Sender}); if err != nil { t.Fatal(err) }; return validators
}
func mustVotingPowerSet(t *testing.T, certificate consensus.FinalityCertificate) consensus.VotingPowerSet {
	t.Helper(); power, err := consensus.NewVotingPowerSet([]consensus.ValidatorVotingPower{{ValidatorID: certificate.Votes[0].Sender, Power: 1}}); if err != nil { t.Fatal(err) }; return power
}

func TestCommitFinalizedBlockUsesExplicitAuthorityBoundaries(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store); if err != nil { t.Fatal(err) }
	seed := make([]byte, 32); seed[0] = 19
	keyPair, err := crypto.NewEd25519KeyPair(seed); if err != nil { t.Fatal(err) }
	signer, err := crypto.NewEd25519Signer(keyPair.PrivateKey); if err != nil { t.Fatal(err) }
	validatorID := []byte("validator-a")
	validators, err := consensus.NewValidatorSet([][]byte{validatorID}); if err != nil { t.Fatal(err) }
	power, err := consensus.NewVotingPowerSet([]consensus.ValidatorVotingPower{{ValidatorID: validatorID, Power: 1}}); if err != nil { t.Fatal(err) }
	ctx := consensus.BlockProductionContext{State: consensus.RoundState{ProtocolVersion: devnet.ProtocolVersion, ChainID: devnet.ChainID, Epoch: 1, Height: 0, Round: 0, Phase: consensus.PhaseProposal}, PreviousHash: n.HeadHash, Proposer: validatorID}
	sender := types.Address([]byte("finalized-sender")); recipient := types.Address([]byte("finalized-recipient"))
	n.State.Set(sender, state.Account{Balance: 60, Nonce: 0})
	tx := transaction.Transaction{Version: devnet.ProtocolVersion, ChainID: devnet.ChainID, Nonce: 0, Sender: sender, Recipient: recipient, Value: 20, GasLimit: 100}
	tx.Signature, err = transaction.Sign(tx, signer); if err != nil { t.Fatal(err) }
	rules, err := n.Config.BlockRules(signer.PublicKey()); if err != nil { t.Fatal(err) }
	working := n.State.Snapshot(); if err := state.ApplyTransaction(working, tx, rules.Transaction); err != nil { t.Fatal(err) }
	candidate := block.Block{Header: block.Header{Version: devnet.ProtocolVersion, ChainID: devnet.ChainID, Height: 1, Timestamp: n.Head.Header.Timestamp + 1, PreviousHash: n.HeadHash, StateRoot: working.Root(), Proposer: validatorID}, Transactions: []any{tx}}
	candidate.Header.TransactionsRoot, err = block.TransactionsRoot(candidate.Transactions); if err != nil { t.Fatal(err) }
	payload, err := consensus.ValidateProducedBlock(ctx, candidate); if err != nil { t.Fatal(err) }
	vote := consensus.Message{ProtocolVersion: 1, ChainID: 1001, Epoch: 1, Height: 0, Round: 0, Sender: validatorID, Type: consensus.MessageTypeVote, Payload: payload[:]}
	certificate, err := consensus.NewFinalityCertificate(ctx.State, validators, power, consensus.QuorumThreshold{Numerator: 1, Denominator: 1}, payload[:], []consensus.Message{vote}); if err != nil { t.Fatal(err) }
	if err := n.CommitFinalizedBlock(ctx, candidate, certificate, validators, power, validatorAuthorityResolver{publicKey: signer.PublicKey()}, senderAuthorityResolver{publicKey: signer.PublicKey()}); err != nil { t.Fatal(err) }
	if n.Head.Header.Height != 1 { t.Fatalf("head height = %d, want 1", n.Head.Header.Height) }
	if got, ok := n.State.Get(recipient); !ok || got.Balance != 20 { t.Fatalf("recipient = %+v, want balance 20", got) }
}
