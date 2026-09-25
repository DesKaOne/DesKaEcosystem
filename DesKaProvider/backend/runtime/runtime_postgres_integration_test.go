package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/routing"
)

func TestOpenTransactionStorePostgresIntegration(t *testing.T) {
	dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured")
	}

	ctx := context.Background()
	cfg := Config{TransactionStoreDriver: "postgres", PostgresDSN: dsn}
	store, db, err := openTransactionStore(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	schema := "runtime_test_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil { t.Fatal(err) }
	defer db.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE")
	if _, err := db.ExecContext(ctx, "SET search_path TO "+schema); err != nil { t.Fatal(err) }

	migration, err := os.ReadFile(filepath.Join("..", "migrations", "001_provider_transactions.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}

	pgStore, ok := store.(*routing.PostgresTransactionStore)
	if !ok {
		t.Fatalf("expected PostgreSQL transaction store, got %T", store)
	}
	states, err := pgStore.AllContextE(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 0 {
		t.Fatalf("expected fresh integration table, got %d transactions", len(states))
	}
}

func TestOpenAuditStorePostgresIntegration(t *testing.T) {
	dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
	if dsn == "" { t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured") }
	ctx := context.Background()
	cfg := Config{AuditStoreDriver: "postgres", PostgresDSN: dsn}
	auditStore, db, err := openAuditStore(ctx, cfg, nil)
	if err != nil { t.Fatal(err) }
	defer db.Close()
	db.SetMaxOpenConns(1)
	schema := "runtime_audit_test_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil { t.Fatal(err) }
	defer db.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE")
	if _, err := db.ExecContext(ctx, "SET search_path TO "+schema); err != nil { t.Fatal(err) }
	migration, err := os.ReadFile(filepath.Join("..", "migrations", "001_provider_transactions.sql"))
	if err != nil { t.Fatal(err) }
	if _, err := db.ExecContext(ctx, string(migration)); err != nil { t.Fatal(err) }
	pgStore, ok := auditStore.(*routing.PostgresTransactionAuditStore)
	if !ok { t.Fatalf("expected PostgreSQL audit store, got %T", auditStore) }
	event := routing.TransactionAuditEvent{ReferenceID:"runtime-audit-1",Action:"PURCHASE_RESULT",Previous:"pending",Next:"success",ProviderName:"mock",Message:"success",CreatedAt:time.Now().UTC()}
	if err := pgStore.AppendContext(ctx, event); err != nil { t.Fatal(err) }
	reloaded, err := pgStore.AllContext(ctx, event.ReferenceID)
	if err != nil { t.Fatal(err) }
	if len(reloaded) != 1 || reloaded[0].ReferenceID != event.ReferenceID || reloaded[0].Action != event.Action || reloaded[0].Next != event.Next { t.Fatalf("unexpected durable audit events: %#v", reloaded) }
}


func TestCloseRuntimeDatabasesClosesSharedAndDedicatedHandles(t *testing.T) {
	dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
	if dsn == "" { t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured") }
	ctx := context.Background()

	sharedCfg := Config{TransactionStoreDriver: "postgres", AuditStoreDriver: "postgres", PostgresDSN: dsn}
	_, sharedDB, err := openTransactionStore(ctx, sharedCfg)
	if err != nil { t.Fatal(err) }
	if _, _, err := openAuditStore(ctx, sharedCfg, sharedDB); err != nil { t.Fatal(err) }
	closeRuntimeDatabases(sharedDB, nil)
	if err := sharedDB.PingContext(ctx); err == nil {
		t.Fatal("expected shared PostgreSQL handle to be closed")
	}

	dedicatedCfg := Config{AuditStoreDriver: "postgres", PostgresDSN: dsn}
	_, dedicatedDB, err := openAuditStore(ctx, dedicatedCfg, nil)
	if err != nil { t.Fatal(err) }
	closeRuntimeDatabases(nil, dedicatedDB)
	if err := dedicatedDB.PingContext(ctx); err == nil {
		t.Fatal("expected dedicated PostgreSQL audit handle to be closed")
	}
}
