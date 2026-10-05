package storage

import (
	"encoding/gob"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

var (
	ErrConsensusEvidenceConflict = errors.New("consensus evidence conflict")
	ErrConsensusEvidenceCorrupt  = errors.New("consensus evidence store corrupt")
)

type consensusEvidenceSnapshot struct {
	Records map[string][]byte
}

// MemoryConsensusEvidenceStore is a separate in-memory persistence boundary
// for consensus evidence; it is never part of canonical ChainStore state.
type MemoryConsensusEvidenceStore struct {
	mu      sync.RWMutex
	records map[string][]byte
}

func NewMemoryConsensusEvidenceStore() *MemoryConsensusEvidenceStore {
	return &MemoryConsensusEvidenceStore{records: make(map[string][]byte)}
}

func (s *MemoryConsensusEvidenceStore) PutConsensusEvidence(key string, encoded []byte) error {
	if s == nil {
		return ErrConsensusEvidenceCorrupt
	}
	if key == "" || len(encoded) == 0 {
		return ErrConsensusEvidenceCorrupt
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.records == nil {
		s.records = make(map[string][]byte)
	}
	if prior, ok := s.records[key]; ok {
		if string(prior) == string(encoded) {
			return nil
		}
		return ErrConsensusEvidenceConflict
	}
	s.records[key] = append([]byte(nil), encoded...)
	return nil
}

func (s *MemoryConsensusEvidenceStore) DeleteConsensusEvidence(key string) error {
	if s == nil {
		return ErrConsensusEvidenceCorrupt
	}
	if key == "" {
		return ErrConsensusEvidenceCorrupt
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.records, key)
	return nil
}

func (s *MemoryConsensusEvidenceStore) LoadConsensusEvidence() (map[string][]byte, error) {
	if s == nil {
		return nil, ErrConsensusEvidenceCorrupt
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string][]byte, len(s.records))
	for key, value := range s.records {
		out[key] = append([]byte(nil), value...)
	}
	return out, nil
}

// FileConsensusEvidenceStore is an atomic, separate file-backed store for
// authenticated consensus evidence. It must not share the ChainStore path.
type FileConsensusEvidenceStore struct {
	mu      sync.RWMutex
	path    string
	records map[string][]byte
}

func NewFileConsensusEvidenceStore(path string) (*FileConsensusEvidenceStore, error) {
	if path == "" {
		return nil, errors.New("empty consensus evidence storage path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	s := &FileConsensusEvidenceStore{path: path, records: make(map[string][]byte)}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *FileConsensusEvidenceStore) load() error {
	f, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	var snapshot consensusEvidenceSnapshot
	if err := gob.NewDecoder(f).Decode(&snapshot); err != nil {
		return err
	}
	if snapshot.Records == nil {
		return ErrConsensusEvidenceCorrupt
	}
	for key, value := range snapshot.Records {
		if key == "" || len(value) == 0 {
			return ErrConsensusEvidenceCorrupt
		}
	}
	s.records = snapshot.Records
	return nil
}

func (s *FileConsensusEvidenceStore) PutConsensusEvidence(key string, encoded []byte) error {
	if s == nil {
		return ErrConsensusEvidenceCorrupt
	}
	if key == "" || len(encoded) == 0 {
		return ErrConsensusEvidenceCorrupt
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if prior, ok := s.records[key]; ok {
		if string(prior) == string(encoded) {
			return nil
		}
		return ErrConsensusEvidenceConflict
	}
	next := cloneEvidenceRecords(s.records)
	next[key] = append([]byte(nil), encoded...)
	return s.persistLocked(next)
}

func (s *FileConsensusEvidenceStore) DeleteConsensusEvidence(key string) error {
	if s == nil {
		return ErrConsensusEvidenceCorrupt
	}
	if key == "" {
		return ErrConsensusEvidenceCorrupt
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.records[key]; !ok {
		return nil
	}
	next := cloneEvidenceRecords(s.records)
	delete(next, key)
	return s.persistLocked(next)
}

func (s *FileConsensusEvidenceStore) LoadConsensusEvidence() (map[string][]byte, error) {
	if s == nil {
		return nil, ErrConsensusEvidenceCorrupt
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneEvidenceRecords(s.records), nil
}

func (s *FileConsensusEvidenceStore) persistLocked(records map[string][]byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".indochain-consensus-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := gob.NewEncoder(tmp).Encode(consensusEvidenceSnapshot{Records: records}); err != nil {
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
	// The rename makes the new snapshot visible atomically. Sync the parent
	// directory as well so the rename itself survives a power-loss restart.
	dir, err := os.Open(filepath.Dir(s.path))
	if err != nil {
		return err
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil {
		return err
	}
	s.records = records
	return nil
}

func cloneEvidenceRecords(in map[string][]byte) map[string][]byte {
	out := make(map[string][]byte, len(in))
	for key, value := range in {
		out[key] = append([]byte(nil), value...)
	}
	return out
}

var _ interface {
	PutConsensusEvidence(string, []byte) error
	LoadConsensusEvidence() (map[string][]byte, error)
	DeleteConsensusEvidence(string) error
} = (*MemoryConsensusEvidenceStore)(nil)

var _ interface {
	PutConsensusEvidence(string, []byte) error
	LoadConsensusEvidence() (map[string][]byte, error)
	DeleteConsensusEvidence(string) error
} = (*FileConsensusEvidenceStore)(nil)
