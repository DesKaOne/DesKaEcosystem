package devnet

import (
	"bytes"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestDefaultGenesisDeterministic(t *testing.T) {
	first := Default()
	second := Default()

	if first.Hash() != second.Hash() {
		t.Fatal("default genesis hash is not deterministic")
	}
	if first.State().Root() != second.State().Root() {
		t.Fatal("default genesis state root is not deterministic")
	}
}

func TestGenesisStateUsesAllocations(t *testing.T) {
	g := Default()
	address := []byte{0x01, 0x02, 0x03}
	g.InitialAllocations[string(address)] = 42

	account, ok := g.State().Get(types.Address(address))
	if !ok {
		t.Fatal("genesis allocation was not initialized")
	}
	if account.Balance != 42 {
		t.Fatalf("balance = %d, want 42", account.Balance)
	}
	if account.Nonce != 0 {
		t.Fatalf("nonce = %d, want 0", account.Nonce)
	}
}

func TestGenesisBlockShape(t *testing.T) {
	g := Default()
	b, err := g.Block()
	if err != nil {
		t.Fatal(err)
	}

	if b.Header.Height != 0 {
		t.Fatalf("height = %d, want 0", b.Header.Height)
	}
	if b.Header.ChainID != ChainID {
		t.Fatalf("chain id = %d, want %d", b.Header.ChainID, ChainID)
	}
	if b.Header.Version != ProtocolVersion {
		t.Fatalf("version = %d, want %d", b.Header.Version, ProtocolVersion)
	}
	if b.Header.PreviousHash != (types.Hash{}) {
		t.Fatal("genesis previous hash must be zero")
	}
	if b.Header.StateRoot != g.State().Root() {
		t.Fatal("genesis state root does not match initial state")
	}

	blockHash, err := block.Hash(b)
	if err != nil {
		t.Fatal(err)
	}
	if blockHash == (types.Hash{}) {
		t.Fatal("genesis block hash must be non-zero")
	}
}

func TestGenesisHashIgnoresMapInsertionOrder(t *testing.T) {
	first := Default()
	first.InitialAllocations[string([]byte{0x02})] = 20
	first.InitialAllocations[string([]byte{0x01})] = 10
	first.ProtocolParameters["gas_limit"] = 1000
	first.ProtocolParameters["max_data"] = 256

	second := Default()
	second.InitialAllocations[string([]byte{0x01})] = 10
	second.InitialAllocations[string([]byte{0x02})] = 20
	second.ProtocolParameters["max_data"] = 256
	second.ProtocolParameters["gas_limit"] = 1000

	if first.Hash() != second.Hash() {
		t.Fatal("genesis hash changed with map insertion order")
	}
}

func TestGenesisHashChangesWithConfiguration(t *testing.T) {
	first := Default()
	second := Default()
	second.Timestamp++

	if first.Hash() == second.Hash() {
		t.Fatal("genesis hash did not change after configuration mutation")
	}
}

func TestGenesisHashDoesNotMutateInputs(t *testing.T) {
	validator := []byte{0x03, 0x02, 0x01}
	account := []byte{0x01, 0x02}
	g := Default()
	g.InitialValidators = [][]byte{validator}
	g.InitialAccounts = [][]byte{account}

	beforeValidator := append([]byte(nil), validator...)
	beforeAccount := append([]byte(nil), account...)
	_ = g.Hash()

	if !bytes.Equal(validator, beforeValidator) {
		t.Fatal("validator input was mutated")
	}
	if !bytes.Equal(account, beforeAccount) {
		t.Fatal("account input was mutated")
	}
}
