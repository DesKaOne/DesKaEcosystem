package runtime

import (
    "context"
    "database/sql"
    "fmt"
    "os"
    "strconv"
    "strings"
    "testing"
    "time"
)

func TestOpenTransactionStorePostgresReadinessFailureIsAttributedAndClosesHandle(t *testing.T) {
    dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
    if dsn == "" {
        t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured")
    }

    ctx := context.Background()
    baseDB, err := sql.Open("pgx", dsn)
    if err != nil { t.Fatal(err) }
    defer baseDB.Close()
    if err := baseDB.PingContext(ctx); err != nil { t.Fatal(err) }

    schema := "runtime_readiness_failure_" + strconv.FormatInt(time.Now().UnixNano(), 10)
    if _, err := baseDB.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil { t.Fatal(err) }
    defer func() { _, _ = baseDB.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE") }()

    isolatedDSN := postgresSchemaDSN(t, dsn, schema)
    cfg := Config{
        TransactionStoreDriver: "postgres",
        PostgresDSN: isolatedDSN,
        PostgresSchemaMode: "check",
    }

    store, db, err := openTransactionStore(ctx, cfg)
    if err == nil {
        if db != nil { _ = db.Close() }
        t.Fatalf("expected schema readiness failure, got store=%T db=%v", store, db != nil)
    }
    if store != nil || db != nil {
        t.Fatalf("failed transaction initialization must not return usable store/database: store=%T db=%v", store, db != nil)
    }
    if !strings.Contains(err.Error(), "provider_transactions") {
        t.Fatalf("expected readiness error to identify the missing transaction schema object, got %v", err)
    }
}

func TestOpenTransactionStorePostgresMigrationConflictIsAttributedAndClosesHandle(t *testing.T) {
    dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
    if dsn == "" {
        t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured")
    }

    ctx := context.Background()
    baseDB, err := sql.Open("pgx", dsn)
    if err != nil { t.Fatal(err) }
    defer baseDB.Close()
    if err := baseDB.PingContext(ctx); err != nil { t.Fatal(err) }

    schema := "runtime_migration_conflict_" + strconv.FormatInt(time.Now().UnixNano(), 10)
    if _, err := baseDB.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil { t.Fatal(err) }
    defer func() { _, _ = baseDB.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE") }()

    isolatedDSN := postgresSchemaDSN(t, dsn, schema)
    conflictDB, err := sql.Open("pgx", isolatedDSN)
    if err != nil { t.Fatal(err) }
    if err := conflictDB.PingContext(ctx); err != nil {
        _ = conflictDB.Close()
        t.Fatal(err)
    }
    if _, err := conflictDB.ExecContext(ctx, "CREATE TABLE provider_schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP)"); err != nil {
        _ = conflictDB.Close()
        t.Fatal(err)
    }
    if _, err := conflictDB.ExecContext(ctx, "INSERT INTO provider_schema_migrations (version, name) VALUES (1, 'wrong_migration_name')"); err != nil {
        _ = conflictDB.Close()
        t.Fatal(err)
    }
    if err := conflictDB.Close(); err != nil { t.Fatal(err) }

    cfg := Config{
        TransactionStoreDriver: "postgres",
        PostgresDSN: isolatedDSN,
        PostgresSchemaMode: "migrate",
    }
    store, db, err := openTransactionStore(ctx, cfg)
    if err == nil {
        if db != nil { _ = db.Close() }
        t.Fatalf("expected migration conflict, got store=%T db=%v", store, db != nil)
    }
    if store != nil || db != nil {
        t.Fatalf("failed migration initialization must not return usable store/database: store=%T db=%v", store, db != nil)
    }
    if !strings.Contains(err.Error(), "migration version 1") {
        t.Fatalf("expected migration conflict to identify version 1, got %v", err)
    }
    if !strings.Contains(err.Error(), fmt.Sprintf("%q", "wrong_migration_name")) {
        t.Fatalf("expected migration conflict to preserve recorded migration name, got %v", err)
    }
}
