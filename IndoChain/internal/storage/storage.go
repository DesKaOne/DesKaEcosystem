package storage

import (
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var (
	ErrBlockNotFound = errors.New("block not found")
	ErrEmptyStore    = errors.New("empty chain store")
)

// ChainStore defines the storage boundary used by node initialization.
// Persistence is intentionally deferred; the interface must remain independent
// from any specific database implementation.
type ChainStore interface {
	SaveBlock(block.Block, types.Hash) error
	GetBlock(height types.Height) (block.Block, types.Hash, error)
	SaveState(*state.State) error
	LoadState() (*state.State, error)
	Head() (block.Block, types.Hash, error)
}

// MemoryStore is a deterministic development-only chain store.
type MemoryStore struct {
	blocks map[types.Height]storedBlock
	state  *state.State
	head   types.Height
	hasHead bool
}

type storedBlock struct {
	block block.Block
	hash  types.Hash
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		blocks: make(map[types.Height]storedBlock),
	}
}

func (s *MemoryStore) SaveBlock(b block.Block, hash types.Hash) error {
	if s == nil {
		return ErrEmptyStore
	}
	if s.blocks == nil {
		s.blocks = make(map[types.Height]storedBlock)
	}
	s.blocks[b.Header.Height] = storedBlock{block: b, hash: hash}
	if !s.hasHead || b.Header.Height >= s.head {
		s.head = b.Header.Height
		s.hasHead = true
	}
	return nil
}

func (s *MemoryStore) GetBlock(height types.Height) (block.Block, types.Hash, error) {
	if s == nil {
		return block.Block{}, types.Hash{}, ErrEmptyStore
	}
	stored, ok := s.blocks[height]
	if !ok {
		return block.Block{}, types.Hash{}, ErrBlockNotFound
	}
	return stored.block, stored.hash, nil
}

func (s *MemoryStore) SaveState(st *state.State) error {
	if s == nil {
		return ErrEmptyStore
	}
	if st == nil {
		return errors.New("nil state")
	}
	s.state = st.Snapshot()
	return nil
}

func (s *MemoryStore) LoadState() (*state.State, error) {
	if s == nil || s.state == nil {
		return nil, ErrEmptyStore
	}
	return s.state.Snapshot(), nil
}

func (s *MemoryStore) Head() (block.Block, types.Hash, error) {
	if s == nil || !s.hasHead {
		return block.Block{}, types.Hash{}, ErrEmptyStore
	}
	return s.GetBlock(s.head)
}

var _ ChainStore = (*MemoryStore)(nil)
