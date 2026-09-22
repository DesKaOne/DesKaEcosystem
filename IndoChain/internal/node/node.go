package node

import (
	"errors"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/genesis/devnet"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

var (
	ErrNilStore          = errors.New("nil chain store")
	ErrGenesisMismatch   = errors.New("genesis identity mismatch")
	ErrStateRootMismatch = errors.New("genesis state root mismatch")
	ErrBlockHashMismatch = errors.New("block hash mismatch")
)

type Node struct {
	Genesis  devnet.Genesis
	Store    storage.ChainStore
	State    *state.State
	Head     block.Block
	HeadHash types.Hash
}

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
	if err := store.CommitBlockState(genesisBlock, genesisHash, initialState); err != nil {
		return nil, err
	}
	return &Node{
		Genesis: genesis, Store: store, State: initialState.Snapshot(),
		Head: genesisBlock, HeadHash: genesisHash,
	}, nil
}

// ImportBlock validates and executes the next block against canonical state,
// then commits the resulting block and state before advancing the node head.
func (n *Node) ImportBlock(b block.Block, rules block.ExecutionRules) error {
	if n == nil || n.Store == nil || n.State == nil {
		return ErrNilStore
	}
	expectedHeight := n.Head.Header.Height + 1
	if err := block.ValidateHeader(b, expectedHeight, n.HeadHash, rules); err != nil {
		return err
	}
	working := n.State.Snapshot()
	if err := block.ExecuteBlock(working, b, expectedHeight, n.HeadHash, rules); err != nil {
		return fmt.Errorf("execute block: %w", err)
	}
	hash, err := block.Hash(b)
	if err != nil {
		return fmt.Errorf("hash block: %w", err)
	}
	if hash == (types.Hash{}) {
		return ErrBlockHashMismatch
	}
	if err := n.Store.CommitBlockState(b, hash, working); err != nil {
		return fmt.Errorf("commit block: %w", err)
	}
	n.State = working.Snapshot()
	n.Head = b
	n.HeadHash = hash
	return nil
}
