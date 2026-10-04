package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type StatusPersistence interface {
	Load() ([]SyncStatus, error)
	Save([]SyncStatus) error
}

type JSONFileStatusPersistence struct {
	path string
}

type statusFileData struct {
	Statuses []SyncStatus `json:"statuses"`
}

func NewJSONFileStatusPersistence(path string) (*JSONFileStatusPersistence, error) {
	if path == "" {
		return nil, errors.New("catalog sync status store path is required")
	}
	return &JSONFileStatusPersistence{path: path}, nil
}

func (s *JSONFileStatusPersistence) Load() ([]SyncStatus, error) {
	raw, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read catalog sync status store: %w", err)
	}
	if len(raw) == 0 {
		return nil, nil
	}
	var payload statusFileData
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode catalog sync status store: %w", err)
	}
	seen := make(map[string]struct{}, len(payload.Statuses))
	for _, status := range payload.Statuses {
		if status.ProviderName == "" {
			return nil, errors.New("catalog sync status provider name is required")
		}
		if status.ConsecutiveFailures < 0 {
			return nil, fmt.Errorf("catalog sync status %q has invalid failure count", status.ProviderName)
		}
		if _, ok := seen[status.ProviderName]; ok {
			return nil, fmt.Errorf("duplicate catalog sync status %q", status.ProviderName)
		}
		seen[status.ProviderName] = struct{}{}
	}
	return payload.Statuses, nil
}

func (s *JSONFileStatusPersistence) Save(statuses []SyncStatus) error {
	copied := append([]SyncStatus(nil), statuses...)
	sort.Slice(copied, func(i, j int) bool {
		return copied[i].ProviderName < copied[j].ProviderName
	})
	payload, err := json.MarshalIndent(statusFileData{Statuses: copied}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode catalog sync status store: %w", err)
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create catalog sync status store directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".catalog-sync-status-*.tmp")
	if err != nil {
		return fmt.Errorf("create catalog sync status store temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}
	defer cleanup()

	if err := tmp.Chmod(0o600); err != nil {
		return fmt.Errorf("secure catalog sync status store temp file: %w", err)
	}
	if _, err := tmp.Write(payload); err != nil {
		return fmt.Errorf("write catalog sync status store: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync catalog sync status store: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close catalog sync status store: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("replace catalog sync status store: %w", err)
	}
	if err := syncDirectory(dir); err != nil {
		return fmt.Errorf("sync catalog sync status store directory: %w", err)
	}
	return nil
}
