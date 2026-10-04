package operational

import (
    "database/sql"
    "errors"
    "os"
    "testing"
    "time"

    _ "github.com/jackc/pgx/v5/stdlib"
)

func TestPostgresStoreIntegration(t *testing.T) {
    dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
    if dsn == "" {
        t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured")
    }
    db, err := sql.Open("pgx", dsn)
    if err != nil { t.Fatal(err) }
    defer db.Close()

    if err := db.Ping(); err != nil { t.Fatal(err) }
    _, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS provider_operational_snapshots (
            provider_name TEXT PRIMARY KEY,
            balance BIGINT NOT NULL,
            currency TEXT NOT NULL,
            health TEXT NOT NULL,
            last_checked_at TIMESTAMPTZ NOT NULL,
            last_success_at TIMESTAMPTZ NOT NULL,
            last_error TEXT NOT NULL DEFAULT '',
            consecutive_failures INTEGER NOT NULL DEFAULT 0
        )`)
    if err != nil { t.Fatal(err) }

    store, err := NewPostgresStore(db)
    if err != nil { t.Fatal(err) }

    now := time.Now().UTC().Truncate(time.Microsecond)
    snapshot := Snapshot{
        ProviderName: "postgres-store-test",
        Balance: 1234500,
        Currency: "IDR",
        Health: HealthHealthy,
        LastCheckedAt: now,
        LastSuccessAt: now,
        ConsecutiveFailures: 0,
    }
    t.Cleanup(func() { _, _ = db.Exec("DELETE FROM provider_operational_snapshots WHERE provider_name = $1", snapshot.ProviderName) })

    if err := store.Put(snapshot); err != nil { t.Fatal(err) }
    got, ok := store.Get(snapshot.ProviderName)
    if !ok { t.Fatal("expected persisted snapshot") }
    if got.ProviderName != snapshot.ProviderName || got.Balance != snapshot.Balance || got.Currency != snapshot.Currency || got.Health != snapshot.Health {
        t.Fatalf("unexpected snapshot: %#v", got)
    }

    snapshot.Balance = 9876500
    snapshot.Health = HealthDegraded
    snapshot.LastError = "temporary provider error"
    snapshot.ConsecutiveFailures = 2
    if err := store.Put(snapshot); err != nil { t.Fatal(err) }

    got, ok = store.Get(snapshot.ProviderName)
    if !ok || got.Balance != snapshot.Balance || got.Health != snapshot.Health || got.LastError != snapshot.LastError || got.ConsecutiveFailures != snapshot.ConsecutiveFailures {
        t.Fatalf("unexpected updated snapshot: %#v", got)
    }
}



func TestPostgresStoreWriteFailureIsAmbiguous(t *testing.T) {
	db, err := sql.Open("pgx", "")
	if err != nil { t.Fatal(err) }
	if err := db.Close(); err != nil { t.Fatal(err) }
	store, err := NewPostgresStore(db)
	if err != nil { t.Fatal(err) }
	now := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	snapshot := Snapshot{
		ProviderName: "postgres-write-failure",
		Balance: 100000,
		Currency: "IDR",
		Health: HealthHealthy,
		LastCheckedAt: now,
		LastSuccessAt: now,
	}
	if err := store.Put(snapshot); !errors.Is(err, ErrOperationalPersistenceAmbiguous) {
		t.Fatalf("expected ambiguous persistence error, got %v", err)
	}
}

func TestPostgresStoreRejectsContradictorySnapshotAndRecoversValidState(t *testing.T) {
    dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
    if dsn == "" {
        t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured")
    }
    db, err := sql.Open("pgx", dsn)
    if err != nil { t.Fatal(err) }
    defer db.Close()
    if err := db.Ping(); err != nil { t.Fatal(err) }
    _, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS provider_operational_snapshots (
            provider_name TEXT PRIMARY KEY,
            balance BIGINT NOT NULL,
            currency TEXT NOT NULL,
            health TEXT NOT NULL,
            last_checked_at TIMESTAMPTZ NOT NULL,
            last_success_at TIMESTAMPTZ NOT NULL,
            last_error TEXT NOT NULL DEFAULT '',
            consecutive_failures INTEGER NOT NULL DEFAULT 0
        )`)
    if err != nil { t.Fatal(err) }

    store, err := NewPostgresStore(db)
    if err != nil { t.Fatal(err) }

    now := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
    valid := Snapshot{
        ProviderName: "postgres-consistency-test",
        Balance: 2000000,
        Currency: "IDR",
        Health: HealthHealthy,
        LastCheckedAt: now,
        LastSuccessAt: now,
    }
    t.Cleanup(func() { _, _ = db.Exec("DELETE FROM provider_operational_snapshots WHERE provider_name = $1", valid.ProviderName) })

    if err := store.Put(valid); err != nil { t.Fatal(err) }

    contradictory := valid
    contradictory.Health = HealthDegraded
    contradictory.ConsecutiveFailures = 0
    contradictory.LastError = ""
    if err := store.Put(contradictory); !errors.Is(err, ErrOperationalHealthState) {
        t.Fatalf("expected contradictory snapshot rejection, got %v", err)
    }

    recoveredStore, err := NewPostgresStore(db)
    if err != nil { t.Fatal(err) }
    got, ok, err := recoveredStore.GetWithError(valid.ProviderName)
    if err != nil { t.Fatal(err) }
    if !ok { t.Fatal("expected valid snapshot to remain persisted") }
    if got.Balance != valid.Balance || got.Health != valid.Health || !got.LastCheckedAt.Equal(valid.LastCheckedAt) {
        t.Fatalf("unexpected recovered snapshot: %#v", got)
    }
}
