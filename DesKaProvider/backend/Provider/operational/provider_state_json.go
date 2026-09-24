package operational

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type JSONFileProviderStateStore struct { path string }

type providerStateFile struct { States []ProviderState ` + "`json:"states"`" + ` }

func NewJSONFileProviderStateStore(path string) (*JSONFileProviderStateStore, error) {
	if path == "" { return nil, errors.New("provider state store path is required") }
	return &JSONFileProviderStateStore{path: path}, nil
}

func (s *JSONFileProviderStateStore) Load() ([]ProviderState, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) { return nil, nil }
	if err != nil { return nil, fmt.Errorf("read provider state store: %w", err) }
	if len(data) == 0 { return nil, nil }
	var file providerStateFile
	if err := json.Unmarshal(data, &file); err != nil { return nil, fmt.Errorf("decode provider state store: %w", err) }
	return file.States, nil
}

func (s *JSONFileProviderStateStore) Save(states []ProviderState) error {
	data, err := json.MarshalIndent(providerStateFile{States: states}, "", "  ")
	if err != nil { return fmt.Errorf("encode provider state store: %w", err) }
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o750); err != nil { return fmt.Errorf("create provider state store directory: %w", err) }
	tmp, err := os.CreateTemp(dir, ".provider-state-*.tmp")
	if err != nil { return fmt.Errorf("create provider state store temp file: %w", err) }
	tmpName := tmp.Name()
	cleanup := func() { _ = tmp.Close(); _ = os.Remove(tmpName) }
	defer cleanup()
	if err := tmp.Chmod(0o600); err != nil { return fmt.Errorf("secure provider state store temp file: %w", err) }
	if _, err := tmp.Write(data); err != nil { return fmt.Errorf("write provider state store: %w", err) }
	if err := tmp.Sync(); err != nil { return fmt.Errorf("sync provider state store: %w", err) }
	if err := tmp.Close(); err != nil { return fmt.Errorf("close provider state store: %w", err) }
	if err := os.Rename(tmpName, s.path); err != nil { return fmt.Errorf("replace provider state store: %w", err) }
	return nil
}
