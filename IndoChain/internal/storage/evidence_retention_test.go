package storage

import (
	"path/filepath"
	"testing"
)

func TestMemoryConsensusEvidenceStoreDeleteIsIdempotent(t *testing.T) {
	store := NewMemoryConsensusEvidenceStore()
	if err := store.PutConsensusEvidence("k", []byte("evidence")); err != nil { t.Fatal(err) }
	if err := store.DeleteConsensusEvidence("k"); err != nil { t.Fatal(err) }
	if err := store.DeleteConsensusEvidence("k"); err != nil { t.Fatal(err) }
	records, err := store.LoadConsensusEvidence()
	if err != nil { t.Fatal(err) }
	if len(records) != 0 { t.Fatalf("expected empty evidence store, got %d records", len(records)) }
}

func TestFileConsensusEvidenceStoreDeletePersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "evidence.gob")
	store, err := NewFileConsensusEvidenceStore(path)
	if err != nil { t.Fatal(err) }
	if err := store.PutConsensusEvidence("k", []byte("evidence")); err != nil { t.Fatal(err) }
	if err := store.DeleteConsensusEvidence("k"); err != nil { t.Fatal(err) }
	reopened, err := NewFileConsensusEvidenceStore(path)
	if err != nil { t.Fatal(err) }
	records, err := reopened.LoadConsensusEvidence()
	if err != nil { t.Fatal(err) }
	if len(records) != 0 { t.Fatalf("expected deleted evidence after reopen, got %d records", len(records)) }
}
