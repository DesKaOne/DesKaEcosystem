package state

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/transaction"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
)

func transitionRules(publicKey []byte) ExecutionRules {
	return ExecutionRules{
		Validation: transaction.ValidationRules{
			ProtocolVersion:  1,
			ChainID:          1001,
			RequireSender:    true,
			RequireRecipient: true,
			RequireSignature: true,
			MinGasLimit:      21000,
			MaxDataSize:      1024,
		},
		PublicKey: publicKey,
	}
}

func signedTransfer(t *testing.T, signer *crypto.Ed25519Signer) transaction.Transaction {
	t.Helper()

	tx := transaction.Transaction{
		Version:   1,
		ChainID:   1001,
		Nonce:     0,
		Sender:    types.Address{1},
		Recipient: types.Address{2},
		Value:     30,
		GasLimit:  21000,
		Data:      []byte("transfer"),
	}
	signature, err := transaction.Sign(tx, signer)
	if err != nil {
		t.Fatal(err)
	}
	tx.Signature = signature
	return tx
}


type mutatingSenderAuthorityResolver struct {
	publicKey []byte
}

type failingSenderAuthorityResolver struct {
	err error
}

func (r failingSenderAuthorityResolver) PublicKeyForSender([]byte) ([]byte, error) {
	return nil, r.err
}


func (r mutatingSenderAuthorityResolver) PublicKeyForSender(sender []byte) ([]byte, error) {
	if len(sender) > 0 {
		sender[0] = 0xff
	}
	return r.publicKey, nil
}

func TestApplyTransactionClonesSenderForAuthorityResolver(t *testing.T) {
	seed := bytes.Repeat([]byte{0x42}, ed25519.SeedSize)
	signer, err := crypto.NewEd25519Signer(ed25519.NewKeyFromSeed(seed))
	if err != nil {
		t.Fatal(err)
	}

	s := New()
	s.Set(types.Address{1}, Account{Balance: 100, Nonce: 0})

	tx := signedTransfer(t, signer)
	rules := transitionRules(nil)
	rules.PublicKeyResolver = mutatingSenderAuthorityResolver{publicKey: signer.PublicKey()}

	if err := ApplyTransaction(s, tx, rules); err != nil {
		t.Fatalf("ApplyTransaction() error = %v", err)
	}
	if !bytes.Equal(tx.Sender, types.Address{1}) {
		t.Fatalf("resolver mutated transaction sender: %v", tx.Sender)
	}
	if sender, ok := s.Get(types.Address{1}); !ok || sender.Balance != 70 || sender.Nonce != 1 {
		t.Fatalf("sender state = %+v, want balance 70 nonce 1", sender)
	}
	if recipient, ok := s.Get(types.Address{2}); !ok || recipient.Balance != 30 {
		t.Fatalf("recipient state = %+v, want balance 30", recipient)
	}
}

func TestApplyTransactionPropagatesSenderAuthorityResolverError(t *testing.T) {
	seed := bytes.Repeat([]byte{0x42}, ed25519.SeedSize)
	signer, err := crypto.NewEd25519Signer(ed25519.NewKeyFromSeed(seed))
	if err != nil {
		t.Fatal(err)
	}

	s := New()
	s.Set(types.Address{1}, Account{Balance: 100, Nonce: 0})

	tx := signedTransfer(t, signer)
	rules := transitionRules(nil)
	resolverErr := errors.New("sender authority lookup failed")
	rules.PublicKeyResolver = failingSenderAuthorityResolver{err: resolverErr}

	if err := ApplyTransaction(s, tx, rules); !errors.Is(err, resolverErr) {
		t.Fatalf("ApplyTransaction() error = %v, want resolver error", err)
	}
	if sender, ok := s.Get(types.Address{1}); !ok || sender.Balance != 100 || sender.Nonce != 0 {
		t.Fatalf("sender state mutated after resolver error: %+v", sender)
	}
	if _, ok := s.Get(types.Address{2}); ok {
		t.Fatal("recipient created after resolver error")
	}
}

func TestApplyTransaction(t *testing.T) {
	seed := bytes.Repeat([]byte{0x42}, ed25519.SeedSize)
	signer, err := crypto.NewEd25519Signer(ed25519.NewKeyFromSeed(seed))
	if err != nil {
		t.Fatal(err)
	}

	s := New()
	s.Set(types.Address{1}, Account{Balance: 100, Nonce: 0})

	tx := signedTransfer(t, signer)
	if err := ApplyTransaction(s, tx, transitionRules(signer.PublicKey())); err != nil {
		t.Fatal(err)
	}

	sender, _ := s.Get(types.Address{1})
	recipient, _ := s.Get(types.Address{2})
	if sender.Balance != 70 || sender.Nonce != 1 {
		t.Fatalf("unexpected sender state: %+v", sender)
	}
	if recipient.Balance != 30 || recipient.Nonce != 0 {
		t.Fatalf("unexpected recipient state: %+v", recipient)
	}
}

func TestApplyTransactionFailureDoesNotMutateState(t *testing.T) {
	seed := bytes.Repeat([]byte{0x42}, ed25519.SeedSize)
	signer, err := crypto.NewEd25519Signer(ed25519.NewKeyFromSeed(seed))
	if err != nil {
		t.Fatal(err)
	}

	s := New()
	s.Set(types.Address{1}, Account{Balance: 10, Nonce: 0})

	tx := signedTransfer(t, signer)
	if err := ApplyTransaction(s, tx, transitionRules(signer.PublicKey())); err != ErrInsufficientBalance {
		t.Fatalf("expected insufficient balance, got %v", err)
	}

	sender, ok := s.Get(types.Address{1})
	if !ok {
		t.Fatal("sender account missing")
	}
	if sender.Balance != 10 || sender.Nonce != 0 {
		t.Fatalf("state mutated after failed execution: %+v", sender)
	}
	if _, ok := s.Get(types.Address{2}); ok {
		t.Fatal("recipient created after failed execution")
	}
}

func TestTransferRejectsBalanceOverflow(t *testing.T) {
	s := New()
	s.Set(types.Address{1}, Account{Balance: 1, Nonce: 0})
	s.Set(types.Address{2}, Account{Balance: ^uint64(0), Nonce: 0})

	if err := s.Transfer(types.Address{1}, types.Address{2}, 1, 0); err != ErrBalanceOverflow {
		t.Fatalf("expected balance overflow, got %v", err)
	}
}

func TestSelfTransferAdvancesNonceWithoutChangingBalance(t *testing.T) {
	s := New()
	address := types.Address{1}
	s.Set(address, Account{Balance: 100, Nonce: 3})

	if err := s.Transfer(address, address, 10, 3); err != nil {
		t.Fatal(err)
	}

	account, ok := s.Get(address)
	if !ok {
		t.Fatal("account missing")
	}
	if account.Balance != 100 || account.Nonce != 4 {
		t.Fatalf("unexpected self-transfer state: %+v", account)
	}
}
