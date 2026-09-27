package runtime

import (
    "context"
    "database/sql"
    "net/url"
    "os"
    "strconv"
    "strings"
    "testing"
    "time"

    provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
    "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
    "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/migrations"
    "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/routing"
)

func postgresSchemaDSN(t *testing.T, dsn, schema string) string {
    t.Helper()
    parsed, err := url.Parse(dsn)
    if err != nil || parsed.Scheme == "" || parsed.Host == "" {
        t.Skip("DESKAPROVIDER_POSTGRES_DSN must be a PostgreSQL URL for runtime schema-isolated integration")
    }
    query := parsed.Query()
    query.Del("options")
    parsed.RawQuery = query.Encode()
    if parsed.RawQuery != "" { parsed.RawQuery += "&" }
    parsed.RawQuery += "options=-c%20search_path%3D" + url.QueryEscape(schema)
    return parsed.String()
}

func TestPostgresRuntimeCombinedPersistenceMigrationAndReopen(t *testing.T) {
    dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
    if dsn == "" {
        t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured")
    }

    ctx := context.Background()
    baseDB, err := sql.Open("pgx", dsn)
    if err != nil {
        t.Fatal(err)
    }
    defer baseDB.Close()
    if err := baseDB.PingContext(ctx); err != nil {
        t.Fatal(err)
    }

    schema := "runtime_combined_" + strconv.FormatInt(time.Now().UnixNano(), 10)
    if _, err := baseDB.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
        t.Fatal(err)
    }
    defer func() {
        _, _ = baseDB.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
    }()

    isolatedDSN := postgresSchemaDSN(t, dsn, schema)
    cfg := Config{
        TransactionStoreDriver: "postgres",
        AuditStoreDriver: "postgres",
        OperationalStoreDriver: "postgres",
        PostgresDSN: isolatedDSN,
        PostgresSchemaMode: "migrate",
    }

    transactionStore, transactionDB, err := openTransactionStore(ctx, cfg)
    if err != nil {
        t.Fatal(err)
    }
    transactionDB.SetMaxOpenConns(1)

    auditStore, auditDB, err := openAuditStore(ctx, cfg, transactionDB)
    if err != nil {
        _ = transactionDB.Close()
        t.Fatal(err)
    }
    if auditDB != nil {
        _ = transactionDB.Close()
        t.Fatalf("combined PostgreSQL audit path must reuse transaction DB, got %T", auditDB)
    }

    operationalStore, operationalDB, err := openOperationalStore(ctx, cfg, transactionDB)
    if err != nil {
        _ = transactionDB.Close()
        t.Fatal(err)
    }
    if operationalDB != nil {
        _ = transactionDB.Close()
        t.Fatalf("combined PostgreSQL operational path must reuse transaction DB, got %T", operationalDB)
    }

    if err := migrations.ValidateVersionSet(1, 2); err != nil {
        _ = transactionDB.Close()
        t.Fatal(err)
    }

    reference := "runtime-combined-" + strconv.FormatInt(time.Now().UnixNano(), 10)
    pending := routing.TransactionState{
        Request: routing.PurchaseRequest{
            ProductCode: "xld10",
            CustomerNo:  "087800001232",
            ReferenceID: reference,
            Amount:      10000,
        },
        Version: 1,
        Execution: routing.PurchaseExecution{
            ProviderName: "mock",
            Result: provider.PurchaseResult{
                ReferenceID: reference,
                CustomerNo:  "087800001232",
                ProductCode: "xld10",
                Status:      provider.StatusPending,
                ProviderCode: "00",
                Message:      "pending",
            },
        },
    }
    if err := transactionStore.(*routing.PostgresTransactionStore).PutContext(ctx, pending); err != nil {
        _ = transactionDB.Close()
        t.Fatal(err)
    }

    success := pending
    success.Version = 2
    success.Execution.Result.Status = provider.StatusSuccess
    success.Execution.Result.Message = "success"
    success.Execution.Result.Price = 10000
    if err := transactionStore.(*routing.PostgresTransactionStore).PutIfCurrentContext(ctx, reference, pending, success); err != nil {
        _ = transactionDB.Close()
        t.Fatal(err)
    }

    event := routing.TransactionAuditEvent{
        ReferenceID: reference,
        Action:      "COMBINED_REOPEN_CHECK",
        Previous:    "pending",
        Next:        "success",
        ProviderName:"mock",
        Message:     "durable combined persistence",
        CreatedAt:   time.Now().UTC(),
    }
    if err := auditStore.(*routing.PostgresTransactionAuditStore).AppendContext(ctx, event); err != nil {
        _ = transactionDB.Close()
        t.Fatal(err)
    }

    snapshot := operational.Snapshot{
        ProviderName: "mock",
        Balance: 500000,
        Currency: "IDR",
        Health: operational.HealthHealthy,
        LastCheckedAt: time.Now().UTC(),
        LastSuccessAt: time.Now().UTC(),
        ConsecutiveFailures: 0,
    }
    if err := operationalStore.Put(snapshot); err != nil {
        _ = transactionDB.Close()
        t.Fatal(err)
    }

    ownership := newRuntimeDatabaseOwnership(transactionDB, nil)
    ownership.operationalDB = nil
    ownership.transferToService()
    if err := ownership.closeOwned(); err != nil {
        t.Fatal(err)
    }
    if err := transactionDB.PingContext(ctx); err == nil {
        t.Fatal("expected combined runtime database handle to be closed")
    }

    reopenedTx, reopenedTxDB, err := openTransactionStore(ctx, cfg)
    if err != nil {
        t.Fatal(err)
    }
    reopenedTxDB.SetMaxOpenConns(1)
    reopenedAudit, reopenedAuditDB, err := openAuditStore(ctx, cfg, reopenedTxDB)
    if err != nil {
        _ = reopenedTxDB.Close()
        t.Fatal(err)
    }
    reopenedOperational, reopenedOperationalDB, err := openOperationalStore(ctx, cfg, reopenedTxDB)
    if err != nil {
        _ = reopenedTxDB.Close()
        t.Fatal(err)
    }
    if reopenedAuditDB != nil || reopenedOperationalDB != nil {
        _ = reopenedTxDB.Close()
        t.Fatalf("reopened combined stores must share one PostgreSQL handle: audit=%v operational=%v", reopenedAuditDB != nil, reopenedOperationalDB != nil)
    }
    defer reopenedTxDB.Close()

    reloaded, found, err := reopenedTx.(*routing.PostgresTransactionStore).GetContextE(ctx, reference)
    if err != nil {
        t.Fatal(err)
    }
    if !found || reloaded.Execution.Result.Status != provider.StatusSuccess || reloaded.Version != 2 {
        t.Fatalf("unexpected transaction after reopen: found=%v state=%#v", found, reloaded)
    }

    events, err := reopenedAudit.(*routing.PostgresTransactionAuditStore).AllContextE(ctx, reference)
    if err != nil {
        t.Fatal(err)
    }
    if len(events) != 1 || events[0].Action != event.Action || events[0].Next != event.Next {
        t.Fatalf("unexpected audit history after reopen: %#v", events)
    }

    reloadedSnapshot, found, err := reopenedOperational.(*operational.PostgresStore).GetWithError("mock")
    if err != nil {
        t.Fatal(err)
    }
    if !found || reloadedSnapshot.Balance != snapshot.Balance || reloadedSnapshot.Health != snapshot.Health || reloadedSnapshot.Currency != snapshot.Currency {
        t.Fatalf("unexpected operational snapshot after reopen: found=%v snapshot=%#v", found, reloadedSnapshot)
    }

    // The migration ledger is part of the same schema and must prove that
    // explicit migrate mode initialized both runtime persistence versions.
    var migrationCount int
    if err := reopenedTxDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM provider_schema_migrations WHERE version IN (1,2)").Scan(&migrationCount); err != nil {
        t.Fatal(err)
    }
    if migrationCount != 2 {
        t.Fatalf("expected migrations 1 and 2 to be recorded, got %d", migrationCount)
    }

    if strings.TrimSpace(schema) == "" {
        t.Fatal("schema name must remain non-empty")
    }
}
