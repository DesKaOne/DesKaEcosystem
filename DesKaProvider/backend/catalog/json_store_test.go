package catalog

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

func TestJSONFileStorePersistsAndReloadsCatalog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog", "state.json")
	now := time.Date(2026,9,25,12,0,0,0,time.UTC)
	store, err := NewJSONFileStore(path)
	if err != nil { t.Fatal(err) }
	if err := store.Put(Snapshot{ProviderName:"mock", Products:[]provider.Product{{Code:"xld10",Name:"XL 10K"}}, SyncedAt:now}); err != nil { t.Fatal(err) }
	reloaded, err := NewJSONFileStore(path)
	if err != nil { t.Fatal(err) }
	got, ok := reloaded.Get("mock")
	if !ok || got.SyncedAt != now || len(got.Products) != 1 || got.Products[0].Code != "xld10" { t.Fatalf("unexpected reloaded snapshot: %#v", got) }
	info, err := os.Stat(path)
	if err != nil { t.Fatal(err) }
	if info.Mode().Perm() != 0600 { t.Fatalf("expected 0600 store permissions, got %o", info.Mode().Perm()) }
}

func TestJSONFileStoreRejectsCorruptJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("{bad"), 0600); err != nil { t.Fatal(err) }
	if _, err := NewJSONFileStore(path); err == nil { t.Fatal("expected corrupt JSON error") }
}
