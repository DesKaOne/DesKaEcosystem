package operational

import (
    "database/sql"
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
