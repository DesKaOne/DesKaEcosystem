package accounting

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func accountingPostgresDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil { t.Fatal(err) }
	t.Cleanup(func(){ _ = db.Close() })
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil { t.Fatal(err) }
	return db
}

func applyLedgerMigration(t *testing.T, db *sql.DB) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok { t.Fatal("resolve test path") }
	path := filepath.Join(filepath.Dir(file), "..", "migrations", "004_double_entry_ledger.sql")
	b, err := os.ReadFile(path)
	if err != nil { t.Fatal(err) }
	for _, statement := range strings.Split(strings.Join(func() []string { var lines []string; for _, line := range strings.Split(string(b), "\n") { if strings.HasPrefix(strings.TrimSpace(line), "--") { continue }; lines = append(lines, line) }; return lines }(), "\n"), ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" || strings.HasPrefix(statement, "--") { continue }
		if _, err := db.Exec(statement); err != nil { t.Fatalf("apply ledger migration: %v", err) }
	}
}

func TestPostgresLedgerAppendIsImmutableAndIdempotent(t *testing.T) {
	db := accountingPostgresDB(t)
	ctx := context.Background()
	schema := "ledger_it_" + time.Now().Format("20060102150405.000000000")
	schema = strings.ReplaceAll(schema, ".", "_")
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil { t.Fatal(err) }
	t.Cleanup(func(){ _, _ = db.ExecContext(context.Background(),"DROP SCHEMA "+schema+" CASCADE") })
	if _, err := db.ExecContext(ctx, "SET search_path TO "+schema); err != nil { t.Fatal(err) }
	applyLedgerMigration(t, db)

	store, err := NewPostgresStore(db)
	if err != nil { t.Fatal(err) }
	account := Account{ID:"customer-main",Type:AccountTypeMain,Currency:"IDR",Name:"Customer Main",Active:true}
	if _, created, err := store.CreateAccount(ctx, account); err != nil || !created { t.Fatalf("create account: %v %v",err,created) }
	settlement := Account{ID:"provider-clearing",Type:AccountTypeClearing,Currency:"IDR",Name:"Provider Clearing",Active:true}
	if _, created, err := store.CreateAccount(ctx, settlement); err != nil || !created { t.Fatalf("create settlement account: %v %v",err,created) }

	tx := validLedgerTransaction()
	tx.CreatedAt = time.Now().UTC()
	if err := store.Append(ctx, tx); err != nil { t.Fatal(err) }
	if err := store.Append(ctx, tx); err != nil { t.Fatalf("identical ledger append must be idempotent: %v",err) }
	conflict := tx
	conflict.Description = "tampered"
	if err := store.Append(ctx, conflict); err != ErrLedgerConflict { t.Fatalf("expected immutable conflict, got %v",err) }
	got, ok, err := store.Get(ctx, tx.ID)
	if err != nil || !ok { t.Fatalf("get ledger transaction: %v %v",err,ok) }
	if len(got.Entries) != 2 || got.Entries[0].Amount != 10000 || got.Entries[1].Amount != 10000 { t.Fatalf("unexpected ledger entries: %#v",got.Entries) }
	all, err := store.All(ctx)
	if err != nil { t.Fatalf("list ledger transactions: %v", err) }
	if len(all) != 1 || all[0].ID != tx.ID { t.Fatalf("unexpected ledger transactions: %#v", all) }
}
