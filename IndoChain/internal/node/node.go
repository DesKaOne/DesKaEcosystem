package node

import (
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/genesis/devnet"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

var (
	ErrNilStore         = errors.New("nil chain store")
	ErrGenesisMismatch  = errors.New("genesis identity mismatch")
	ErrStateRootMismatch = errors.New("genesis state root mismatch")
)

// Node is the development node composition boundary.
type Node struct {
	Genesis devnet.Genesis
	Store   storage.ChainStore
	State   *state.State
	Head    block.Block
	HeadHash types.Hash
}

// NewDevnet initializes a node from the deterministic Devnet genesis.
func NewDevnet(store storage.ChainStore) (*Node, error) {
	if store == nil {
		return nil, ErrNilStore
	}

	genesis := devnet.Default()
	genesisBlock, err := genesis.Block()
	if err != nil {
		return nil, err
	}

	genesisHash, err := block.Hash(genesisBlock)
	if err != nil {
		return nil, err
	}

	initialState := genesis.State()
	if genesisBlock.Header.StateRoot != initialState.Root() {
		return nil, ErrStateRootMismatch
	}

	if err := store.SaveBlock(genesisBlock, genesisHash); err != nil {
		return nil, err
	}
	if err := store.SaveState(initialState); err != nil {
		return nil, err
	}

	return &Node{
		Genesis:  genesis,
		Store:    store,
		State:    initialState.Snapshot(),
		Head:     genesisBlock,
		HeadHash: genesisHash,
	}, nil
}
