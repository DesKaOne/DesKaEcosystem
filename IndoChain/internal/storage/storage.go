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

type ChainStore interface {
	SaveBlock(block.Block, types.Hash) error
	GetBlock(height types.Height) (block.Block, types.Hash, error)
	SaveState(*state.State) error
	LoadState() (*state.State, error)
	Head() (block.Block, types.Hash, error)
	CommitBlockState(block.Block, types.Hash, *state.State) error
}

type MemoryStore struct {
	blocks  map[types.Height]storedBlock
	state   *state.State
	head    types.Height
	hasHead bool
}

type storedBlock struct {
	block block.Block
	hash  types.Hash
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{blocks: make(map[types.Height]storedBlock)}
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

// CommitBlockState is the development storage atomicity boundary.
// A persistent implementation must provide durable atomic commit semantics.
func (s *MemoryStore) CommitBlockState(b block.Block, hash types.Hash, st *state.State) error {
	if s == nil {
		return ErrEmptyStore
	}
	if st == nil {
		return errors.New("nil state")
	}
	if err := s.SaveBlock(b, hash); err != nil {
		return err
	}
	return s.SaveState(st)
}

var _ ChainStore = (*MemoryStore)(nil)
