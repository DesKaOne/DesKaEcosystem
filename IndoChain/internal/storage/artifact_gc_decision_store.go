package storage

import (
	"encoding/gob"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

var (
	ErrArtifactGCDecisionConflict = errors.New("artifact GC decision conflict")
	ErrArtifactGCDecisionCorrupt  = errors.New("artifact GC decision store corrupt")
)

type artifactGCDecisionSnapshot struct {
	Records map[string][]byte
}

// MemoryArtifactGCDecisionStore is an isolated durable-decision boundary for
// tests and process-local recovery.
type MemoryArtifactGCDecisionStore struct {
	mu      sync.RWMutex
	records map[string][]byte
}

func NewMemoryArtifactGCDecisionStore() *MemoryArtifactGCDecisionStore {
	return &MemoryArtifactGCDecisionStore{records: make(map[string][]byte)}
}

func (s *MemoryArtifactGCDecisionStore) PutArtifactGCDecision(key string, encoded []byte) error {
	if s == nil || key == "" || len(encoded) == 0 {
		return ErrArtifactGCDecisionCorrupt
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
		return ErrArtifactGCDecisionConflict
	}
	s.records[key] = append([]byte(nil), encoded...)
	return nil
}

func (s *MemoryArtifactGCDecisionStore) LoadArtifactGCDecisions() (map[string][]byte, error) {
	if s == nil {
		return nil, ErrArtifactGCDecisionCorrupt
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneArtifactGCDecisions(s.records), nil
}

func (s *MemoryArtifactGCDecisionStore) DeleteArtifactGCDecision(key string) error {
	if s == nil || key == "" {
		return ErrArtifactGCDecisionCorrupt
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.records, key)
	return nil
}

// FileArtifactGCDecisionStore uses atomic snapshot replacement and must remain
// separate from canonical ChainStore and consensus evidence storage.
type FileArtifactGCDecisionStore struct {
	mu      sync.RWMutex
	path    string
	records map[string][]byte
}

func NewFileArtifactGCDecisionStore(path string) (*FileArtifactGCDecisionStore, error) {
	if path == "" {
		return nil, errors.New("empty artifact GC decision storage path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	s := &FileArtifactGCDecisionStore{path: path, records: make(map[string][]byte)}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *FileArtifactGCDecisionStore) load() error {
	f, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	var snapshot artifactGCDecisionSnapshot
	if err := gob.NewDecoder(f).Decode(&snapshot); err != nil {
		return err
	}
	if snapshot.Records == nil {
		return ErrArtifactGCDecisionCorrupt
	}
	for key, value := range snapshot.Records {
		if key == "" || len(value) == 0 {
			return ErrArtifactGCDecisionCorrupt
		}
	}
	s.records = cloneArtifactGCDecisions(snapshot.Records)
	return nil
}

func (s *FileArtifactGCDecisionStore) PutArtifactGCDecision(key string, encoded []byte) error {
	if s == nil || key == "" || len(encoded) == 0 {
		return ErrArtifactGCDecisionCorrupt
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if prior, ok := s.records[key]; ok {
		if string(prior) == string(encoded) {
			return nil
		}
		return ErrArtifactGCDecisionConflict
	}
	next := cloneArtifactGCDecisions(s.records)
	next[key] = append([]byte(nil), encoded...)
	return s.persistLocked(next)
}

func (s *FileArtifactGCDecisionStore) LoadArtifactGCDecisions() (map[string][]byte, error) {
	if s == nil {
		return nil, ErrArtifactGCDecisionCorrupt
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneArtifactGCDecisions(s.records), nil
}

func (s *FileArtifactGCDecisionStore) DeleteArtifactGCDecision(key string) error {
	if s == nil || key == "" {
		return ErrArtifactGCDecisionCorrupt
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.records[key]; !ok {
		return nil
	}
	next := cloneArtifactGCDecisions(s.records)
	delete(next, key)
	return s.persistLocked(next)
}

func (s *FileArtifactGCDecisionStore) persistLocked(records map[string][]byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".indochain-artifact-gc-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := gob.NewEncoder(tmp).Encode(artifactGCDecisionSnapshot{Records: records}); err != nil {
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
	s.records = records
	return nil
}

func cloneArtifactGCDecisions(in map[string][]byte) map[string][]byte {
	out := make(map[string][]byte, len(in))
	for key, value := range in {
		out[key] = append([]byte(nil), value...)
	}
	return out
}

var _ interface {
	PutArtifactGCDecision(string, []byte) error
	LoadArtifactGCDecisions() (map[string][]byte, error)
	DeleteArtifactGCDecision(string) error
} = (*MemoryArtifactGCDecisionStore)(nil)

var _ interface {
	PutArtifactGCDecision(string, []byte) error
	LoadArtifactGCDecisions() (map[string][]byte, error)
	DeleteArtifactGCDecision(string) error
} = (*FileArtifactGCDecisionStore)(nil)
