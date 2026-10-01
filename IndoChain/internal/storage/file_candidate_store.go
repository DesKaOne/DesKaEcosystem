package storage

import (
	"encoding/gob"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/transaction"
)

type candidateSnapshot struct {
	Candidates map[CandidateKey]StoredBlock
}

type FileCandidateStore struct {
	mu   sync.RWMutex
	path string
	data candidateSnapshot
}

func NewFileCandidateStore(path string) (*FileCandidateStore, error) {
	if path == "" { return nil, errors.New("empty candidate storage path") }
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { return nil, err }
	s := &FileCandidateStore{path:path, data:candidateSnapshot{Candidates:make(map[CandidateKey]StoredBlock)}}
	if err := s.load(); err != nil { return nil, err }
	return s, nil
}

func (s *FileCandidateStore) load() error {
	f, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) { return nil }
	if err != nil { return err }
	defer f.Close()
	var data candidateSnapshot
	if err := gob.NewDecoder(f).Decode(&data); err != nil { return err }
	if data.Candidates == nil { data.Candidates = make(map[CandidateKey]StoredBlock) }
	s.data = data
	return nil
}

func (s *FileCandidateStore) SaveCandidate(key CandidateKey, candidate block.Block) error {
	if s == nil { return ErrNilCandidateStore }
	if err := validateCandidateKey(key, candidate); err != nil { return err }
	s.mu.Lock()
	defer s.mu.Unlock()
	data := candidateSnapshot{Candidates:make(map[CandidateKey]StoredBlock,len(s.data.Candidates)+1)}
	for k,v := range s.data.Candidates { data.Candidates[k] = v }
	data.Candidates[key] = StoredBlock{Block:cloneBlock(candidate), Hash:key.Hash}
	return s.persistLocked(data)
}

func (s *FileCandidateStore) GetCandidate(key CandidateKey) (block.Block, error) {
	if s == nil { return block.Block{}, ErrNilCandidateStore }
	s.mu.RLock()
	defer s.mu.RUnlock()
	stored, ok := s.data.Candidates[key]
	if !ok { return block.Block{}, ErrCandidateNotFound }
	if err := validateCandidateKey(key, stored.Block); err != nil { return block.Block{}, err }
	return cloneBlock(stored.Block), nil
}

func (s *FileCandidateStore) persistLocked(data candidateSnapshot) error {
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".indochain-candidate-*")
	if err != nil { return err }
	name := tmp.Name()
	defer os.Remove(name)
	if err := gob.NewEncoder(tmp).Encode(data); err != nil { _ = tmp.Close(); return err }
	if err := tmp.Sync(); err != nil { _ = tmp.Close(); return err }
	if err := tmp.Close(); err != nil { return err }
	if err := os.Rename(name, s.path); err != nil { return err }
	s.data = data
	return nil
}

func init() { gob.Register(transaction.Transaction{}) }

var _ CandidateStore = (*FileCandidateStore)(nil)
