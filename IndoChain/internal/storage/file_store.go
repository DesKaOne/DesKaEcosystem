package storage

import (
	"encoding/gob"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/transaction"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

type fileSnapshot struct {
	Blocks  map[types.Height]storedBlock
	State   map[string]state.Account
	Head    types.Height
	HasHead bool
}

// FileStore is a development persistent ChainStore backed by one atomically replaced file.
// Its gob representation is an implementation format and is not canonical protocol encoding.
type FileStore struct {
	mu    sync.RWMutex
	path  string
	data  fileSnapshot
}

func NewFileStore(path string) (*FileStore, error) {
	if path == "" {
		return nil, errors.New("empty storage path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	fs := &FileStore{path: path, data: fileSnapshot{Blocks: make(map[types.Height]storedBlock)}}
	if err := fs.load(); err != nil {
		return nil, err
	}
	return fs, nil
}

func (s *FileStore) load() error {
	f, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()

	dec := gob.NewDecoder(f)
	var data fileSnapshot
	if err := dec.Decode(&data); err != nil {
		return err
	}
	if data.Blocks == nil {
		data.Blocks = make(map[types.Height]storedBlock)
	}
	s.data = data
	return nil
}

func (s *FileStore) SaveBlock(b block.Block, hash types.Hash) error {
	if s == nil {
		return ErrEmptyStore
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Blocks[b.Header.Height] = storedBlock{block: b, hash: hash}
	if !s.data.HasHead || b.Header.Height >= s.data.Head {
		s.data.Head = b.Header.Height
		s.data.HasHead = true
	}
	return s.persistLocked()
}

func (s *FileStore) GetBlock(height types.Height) (block.Block, types.Hash, error) {
	if s == nil {
		return block.Block{}, types.Hash{}, ErrEmptyStore
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	stored, ok := s.data.Blocks[height]
	if !ok {
		return block.Block{}, types.Hash{}, ErrBlockNotFound
	}
	return stored.block, stored.hash, nil
}

func (s *FileStore) SaveState(st *state.State) error {
	if s == nil {
		return ErrEmptyStore
	}
	if st == nil {
		return errors.New("nil state")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.State = st.Accounts()
	return s.persistLocked()
}

func (s *FileStore) LoadState() (*state.State, error) {
	if s == nil {
		return nil, ErrEmptyStore
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.data.HasHead || s.data.State == nil {
		return nil, ErrEmptyStore
	}
	return state.FromAccounts(s.data.State), nil
}

func (s *FileStore) Head() (block.Block, types.Hash, error) {
	if s == nil {
		return block.Block{}, types.Hash{}, ErrEmptyStore
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.data.HasHead {
		return block.Block{}, types.Hash{}, ErrEmptyStore
	}
	stored, ok := s.data.Blocks[s.data.Head]
	if !ok {
		return block.Block{}, types.Hash{}, ErrBlockNotFound
	}
	return stored.block, stored.hash, nil
}

// CommitBlockState atomically replaces the on-disk snapshot after block/state validation
// has already completed at the node execution boundary.
func (s *FileStore) CommitBlockState(b block.Block, hash types.Hash, st *state.State) error {
	if s == nil {
		return ErrEmptyStore
	}
	if st == nil {
		return errors.New("nil state")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	candidate := fileSnapshot{
		Blocks: make(map[types.Height]storedBlock, len(s.data.Blocks)+1),
		State: st.Accounts(),
		Head: b.Header.Height,
		HasHead: true,
	}
	for height, stored := range s.data.Blocks {
		candidate.Blocks[height] = stored
	}
	candidate.Blocks[b.Header.Height] = storedBlock{block: b, hash: hash}
	return s.persistSnapshotLocked(candidate)
}

func (s *FileStore) persistLocked() error {
	return s.persistSnapshotLocked(s.data)
}

func (s *FileStore) persistSnapshotLocked(data fileSnapshot) error {
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".indochain-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := gob.NewEncoder(tmp).Encode(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return err
	}
	s.data = data
	return nil
}

func init() {
	gob.Register(transaction.Transaction{})
}

var _ ChainStore = (*FileStore)(nil)
