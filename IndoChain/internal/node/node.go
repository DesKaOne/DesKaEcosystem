package node

import (
	"errors"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/genesis/devnet"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/config"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
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
	ErrStoreCorrupt      = errors.New("chain store consistency check failed")
	ErrHistoryMismatch   = errors.New("chain history consistency check failed")
	ErrConsensusContextMismatch = errors.New("consensus execution context mismatch")
)

type ValidatorAuthorityResolver interface {
	PublicKeyForValidator(validatorID []byte) ([]byte, error)
}

type TransactionAuthorityResolver interface {
	PublicKeyForSender(sender []byte) ([]byte, error)
}

type Node struct {
	Config   config.ChainConfig
	Genesis  devnet.Genesis
	Store    storage.ChainStore
	State    *state.State
	Head     block.Block
	HeadHash types.Hash
}

func devnetConfigAndGenesis() (config.ChainConfig, devnet.Genesis, error) {
	chainConfig := config.Devnet()
	if err := chainConfig.Validate(); err != nil {
		return config.ChainConfig{}, devnet.Genesis{}, err
	}
	genesis := devnet.Default()
	if chainConfig.NetworkProfile != genesis.NetworkProfile || chainConfig.ChainID != genesis.ChainID || chainConfig.ProtocolVersion != genesis.ProtocolVersion {
		return config.ChainConfig{}, devnet.Genesis{}, ErrGenesisMismatch
	}
	return chainConfig, genesis, nil
}

func NewDevnet(store storage.ChainStore) (*Node, error) {
	if store == nil {
		return nil, ErrNilStore
	}
	chainConfig, genesis, err := devnetConfigAndGenesis()
	if err != nil {
		return nil, err
	}
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
		Config: chainConfig, Genesis: genesis, Store: store, State: initialState.Snapshot(),
		Head: genesisBlock, HeadHash: genesisHash,
	}, nil
}

// OpenDevnet opens an existing Devnet store, or initializes an empty store with
// the canonical Devnet genesis. Existing state is validated before the node is
// returned so recovery never silently starts from a fresh genesis.
func OpenDevnet(store storage.ChainStore) (*Node, error) {
	if store == nil {
		return nil, ErrNilStore
	}
	chainConfig, genesis, err := devnetConfigAndGenesis()
	if err != nil {
		return nil, err
	}

	head, storedHash, err := store.Head()
	if errors.Is(err, storage.ErrEmptyStore) {
		return NewDevnet(store)
	}
	if err != nil {
		return nil, err
	}

	stateSnapshot, err := store.LoadState()
	if err != nil {
		return nil, fmt.Errorf("%w: load state: %v", ErrStoreCorrupt, err)
	}
	if stateSnapshot == nil {
		return nil, fmt.Errorf("%w: nil state", ErrStoreCorrupt)
	}
	if head.Header.ChainID != chainConfig.ChainID || head.Header.Version != chainConfig.ProtocolVersion {
		return nil, ErrGenesisMismatch
	}
	computedHeadHash, err := block.Hash(head)
	if err != nil {
		return nil, fmt.Errorf("%w: compute head hash: %v", ErrStoreCorrupt, err)
	}
	if storedHash == (types.Hash{}) || storedHash != computedHeadHash {
		return nil, ErrBlockHashMismatch
	}
	if head.Header.StateRoot != (types.Hash{}) && head.Header.StateRoot != stateSnapshot.Root() {
		return nil, ErrStateRootMismatch
	}
	if err := validateStoredHistory(store, head, storedHash, genesis, chainConfig); err != nil {
		return nil, err
	}

	return &Node{
		Config: chainConfig, Genesis: genesis, Store: store,
		State: stateSnapshot.Snapshot(), Head: head, HeadHash: storedHash,
	}, nil
}

func validateStoredHistory(store storage.ChainStore, head block.Block, storedHeadHash types.Hash, genesis devnet.Genesis, chainConfig config.ChainConfig) error {
	genesisBlock, err := genesis.Block()
	if err != nil {
		return err
	}
	genesisHash, err := block.Hash(genesisBlock)
	if err != nil {
		return err
	}

	var previousHash types.Hash
	for height := types.Height(0); height <= head.Header.Height; height++ {
		storedBlock, storedHash, err := store.GetBlock(height)
		if err != nil {
			return fmt.Errorf("%w: missing block %d: %v", ErrHistoryMismatch, height, err)
		}
		if storedBlock.Header.Height != height {
			return fmt.Errorf("%w: block %d has header height %d", ErrHistoryMismatch, height, storedBlock.Header.Height)
		}
		if storedBlock.Header.ChainID != chainConfig.ChainID || storedBlock.Header.Version != chainConfig.ProtocolVersion {
			return fmt.Errorf("%w: block %d has incompatible chain metadata", ErrHistoryMismatch, height)
		}
		computedHash, err := block.Hash(storedBlock)
		if err != nil {
			return fmt.Errorf("%w: hash block %d: %v", ErrHistoryMismatch, height, err)
		}
		if storedHash == (types.Hash{}) || storedHash != computedHash {
			return fmt.Errorf("%w: block %d hash mismatch", ErrHistoryMismatch, height)
		}
		if height == 0 {
			if storedHash != genesisHash {
				return ErrGenesisMismatch
			}
			previousHash = storedHash
			continue
		}
		if storedBlock.Header.PreviousHash != previousHash {
			return fmt.Errorf("%w: block %d previous hash mismatch", ErrHistoryMismatch, height)
		}
		previousHash = storedHash
	}

	if storedHeadHash != previousHash {
		return fmt.Errorf("%w: stored head hash mismatch", ErrHistoryMismatch)
	}
	return nil
}

// ImportBlock validates and executes the next block against canonical state,
// then commits the resulting block and state before advancing the node head.
func (n *Node) ImportBlock(b block.Block, publicKey []byte) error {
	if n == nil || n.Store == nil || n.State == nil {
		return ErrNilStore
	}
	rules, err := n.Config.BlockRules(publicKey)
	if err != nil {
		return err
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


// ImportBlockWithAuthority validates and executes a block using a sender-key
// resolver owned by the execution/node layer. The canonical state and block
// commit remain atomic at the node boundary.
func (n *Node) ImportBlockWithAuthority(b block.Block, resolver TransactionAuthorityResolver) error {
	if n == nil || n.Store == nil || n.State == nil {
		return ErrNilStore
	}
	if resolver == nil {
		return errors.New("nil transaction authority resolver")
	}
	rules, err := n.Config.BlockRules(nil)
	if err != nil { return err }
	rules.Transaction.PublicKeyResolver = resolver
	expectedHeight := n.Head.Header.Height + 1
	if err := block.ValidateHeader(b, expectedHeight, n.HeadHash, rules); err != nil { return err }
	working := n.State.Snapshot()
	if err := block.ExecuteBlock(working, b, expectedHeight, n.HeadHash, rules); err != nil {
		return fmt.Errorf("execute block: %w", err)
	}
	hash, err := block.Hash(b)
	if err != nil { return fmt.Errorf("hash block: %w", err) }
	if hash == (types.Hash{}) { return ErrBlockHashMismatch }
	if err := n.Store.CommitBlockState(b, hash, working); err != nil { return fmt.Errorf("commit block: %w", err) }
	n.State = working.Snapshot(); n.Head = b; n.HeadHash = hash
	return nil
}

// CommitFinalizedBlock validates the consensus finality binding and explicit
// proposer authority before executing and committing the block. Transaction
// sender authority remains separately resolved by senderResolver; validator
// identity is never treated as a transaction address.
func (n *Node) CommitFinalizedBlock(
	ctx consensus.BlockProductionContext,
	candidate block.Block,
	certificate consensus.FinalityCertificate,
	validators consensus.ValidatorSet,
	votingPower consensus.VotingPowerSet,
	validatorResolver ValidatorAuthorityResolver,
	senderResolver TransactionAuthorityResolver,
) error {
	if n == nil || n.Store == nil || n.State == nil {
		return ErrNilStore
	}
	if validatorResolver == nil || senderResolver == nil {
		return errors.New("missing finalized-block authority resolver")
	}
	if err := ctx.State.Validate(); err != nil {
		return err
	}
	if ctx.State.ProtocolVersion != n.Config.ProtocolVersion || ctx.State.ChainID != n.Config.ChainID ||
		ctx.State.Height != n.Head.Header.Height || ctx.PreviousHash != n.HeadHash {
		return ErrConsensusContextMismatch
	}
	if _, err := consensus.ValidateFinalizedBlock(ctx, candidate, certificate, validators, votingPower); err != nil {
		return err
	}
	authorization := consensus.FinalizedBlockAuthorization{
		BlockHash: func() types.Hash { h, _ := block.Hash(candidate); return h }(),
		Proposer: candidate.Header.Proposer,
		Certificate: certificate,
	}
	if _, err := consensus.ResolveProposerAuthority(authorization, validatorResolver); err != nil {
		return err
	}
	return n.ImportBlockWithAuthority(candidate, senderResolver)
}
