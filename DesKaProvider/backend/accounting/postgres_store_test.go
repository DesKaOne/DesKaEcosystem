package accounting

import (
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/routing"
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

func applyAccountingMigrations(t *testing.T, db *sql.DB) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok { t.Fatal("resolve test path") }
	for _, name := range []string{"004_double_entry_ledger.sql", "005_settlement_audit.sql", "006_ledger_constraint_hardening.sql"} {
		path := filepath.Join(filepath.Dir(file), "..", "migrations", name)
		b, err := os.ReadFile(path)
		if err != nil { t.Fatal(err) }
		for _, statement := range strings.Split(strings.Join(func() []string { var lines []string; for _, line := range strings.Split(string(b), "\n") { if strings.HasPrefix(strings.TrimSpace(line), "--") { continue }; lines = append(lines, line) }; return lines }(), "\n"), ";") {
		statement = strings.TrimSpace(statement)
			if statement == "" || strings.HasPrefix(statement, "--") { continue }
			if _, err := db.Exec(statement); err != nil { t.Fatalf("apply accounting migration %s: %v", name, err) }
		}
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
	applyAccountingMigrations(t, db)

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


func TestPostgresSettlementAppendIsAtomicAndIdempotent(t *testing.T) {
	db := accountingPostgresDB(t)
	ctx := context.Background()
	schema := "settlement_it_" + strings.ReplaceAll(time.Now().Format("20060102150405.000000000"), ".", "_")
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil { t.Fatal(err) }
	t.Cleanup(func(){ _, _ = db.ExecContext(context.Background(),"DROP SCHEMA "+schema+" CASCADE") })
	if _, err := db.ExecContext(ctx, "SET search_path TO "+schema); err != nil { t.Fatal(err) }
	applyAccountingMigrations(t, db)

	store, err := NewPostgresStore(db)
	if err != nil { t.Fatal(err) }
	for _, account := range []Account{
		{ID:"customer-main",Type:AccountTypeMain,Currency:"IDR",Name:"Customer Main",Active:true},
		{ID:"provider-clearing",Type:AccountTypeClearing,Currency:"IDR",Name:"Provider Clearing",Active:true},
		{ID:"settlement-in",Type:AccountTypeSettlementIn,Currency:"IDR",Name:"Settlement In",Active:true},
	} {
		if _, created, err := store.CreateAccount(ctx, account); err != nil || !created { t.Fatalf("create settlement account: %v %v", err, created) }
	}
	tx := validLedgerTransaction()
	tx.ID = "settlement-ledger-pg-1"
	tx.CreatedAt = time.Now().UTC()
	audit := SettlementAudit{
		EventID: "settlement-audit-pg-1",
		TransactionID: tx.ID,
		ReferenceID: tx.ReferenceID,
		SourceType: tx.SourceType,
		SourceID: tx.SourceID,
		Status: ProviderStatusSuccess,
		CreatedAt: tx.CreatedAt,
	}
	if err := store.AppendSettlement(ctx, tx, audit); err != nil { t.Fatal(err) }
	if err := store.AppendSettlement(ctx, tx, audit); err != nil { t.Fatalf("identical settlement append must be idempotent: %v", err) }
	got, ok, err := store.GetSettlementAudit(ctx, tx.ID)
	if err != nil || !ok { t.Fatalf("get settlement audit: %v %v", err, ok) }
	if got.EventID != audit.EventID || got.ReferenceID != audit.ReferenceID || got.SourceID != audit.SourceID {
		t.Fatalf("unexpected settlement audit: %#v", got)
	}
	conflict := audit
	conflict.EventID = "tampered-event"
	if err := store.AppendSettlement(ctx, tx, conflict); err != ErrLedgerConflict { t.Fatalf("expected audit identity conflict, got %v", err) }
}


func TestPostgresSettlementReconciliationReportsOrphansReadOnly(t *testing.T) {
	db := accountingPostgresDB(t)
	ctx := context.Background()
	schema := "recon_it_" + strings.ReplaceAll(time.Now().Format("20060102150405.000000000"), ".", "_")
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil { t.Fatal(err) }
	t.Cleanup(func(){ _, _ = db.ExecContext(context.Background(),"DROP SCHEMA "+schema+" CASCADE") })
	if _, err := db.ExecContext(ctx, "SET search_path TO "+schema); err != nil { t.Fatal(err) }
	applyAccountingMigrations(t, db)

	store, err := NewPostgresStore(db)
	if err != nil { t.Fatal(err) }
	for _, account := range []Account{
		{ID:"customer-main",Type:AccountTypeMain,Currency:"IDR",Name:"Customer Main",Active:true},
		{ID:"provider-clearing",Type:AccountTypeClearing,Currency:"IDR",Name:"Provider Clearing",Active:true},
		{ID:"settlement-in",Type:AccountTypeSettlementIn,Currency:"IDR",Name:"Settlement In",Active:true},
	} {
		if _, created, err := store.CreateAccount(ctx, account); err != nil || !created { t.Fatalf("create account: %v %v", err, created) }
	}

	orphan := validLedgerTransaction()
	orphan.ID = "recon-orphan-ledger"
	orphan.ReferenceID = "recon-orphan-reference"
	orphan.SourceID = "recon-orphan-source"
	orphan.CreatedAt = time.Date(2026,10,3,12,0,0,0,time.UTC)
	if err := store.Append(ctx, orphan); err != nil { t.Fatal(err) }

	if _, err := db.ExecContext(ctx,
		"INSERT INTO settlement_audit (event_id,transaction_id,reference_id,source_type,source_id,status,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)",
		"recon-orphan-audit",orphan.ID,orphan.ReferenceID,orphan.SourceType,orphan.SourceID,ProviderStatusSuccess,
		time.Date(2026,10,3,12,1,0,0,time.UTC),
	); err != nil { t.Fatal(err) }

	txStore := routing.NewMemoryTransactionStore()
	reconciler, err := NewSettlementReconciler(txStore, store, store)
	if err != nil { t.Fatal(err) }
	before, err := store.All(ctx)
	if err != nil { t.Fatal(err) }

	report, err := reconciler.Reconcile(ctx)
	if err != nil { t.Fatal(err) }

	var orphanLedger bool
	for _, item := range report.Items {
		switch item.Status {
		case ReconciliationOrphanedLedger:
			orphanLedger = item.LedgerTransactionID == orphan.ID
		}
	}
	if !orphanLedger { t.Fatalf("expected orphaned ledger diagnostic: %#v", report.Items) }

	after, err := store.All(ctx)
	if err != nil { t.Fatal(err) }
	if len(after) != len(before) { t.Fatal("reconciliation must not mutate durable ledger state") }
}

func TestPostgresLedgerSchemaRejectsInvalidEntryDirectionAndCurrency(t *testing.T) {
	db := accountingPostgresDB(t)
	ctx := context.Background()
	schema := "ledger_constraints_" + strings.ReplaceAll(time.Now().Format("20060102150405.000000000"), ".", "_")
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil { t.Fatal(err) }
	t.Cleanup(func(){ _, _ = db.ExecContext(context.Background(),"DROP SCHEMA "+schema+" CASCADE") })
	if _, err := db.ExecContext(ctx, "SET search_path TO "+schema); err != nil { t.Fatal(err) }
	applyAccountingMigrations(t, db)

	if _, err := db.ExecContext(ctx,
		"INSERT INTO ledger_transactions (transaction_id,reference_id,source_type,source_id,currency,description,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)",
		"constraint-tx","constraint-ref","TEST","constraint-source","IDR","constraints",time.Now().UTC(),
	); err != nil { t.Fatal(err) }

	if _, err := db.ExecContext(ctx,
		"INSERT INTO ledger_accounts (account_id,account_type,owner_id,currency,name,active) VALUES ($1,$2,$3,$4,$5,$6)",
		"constraint-account","CLEARING","","IDR","Constraint Account",true,
	); err != nil { t.Fatal(err) }

	if _, err := db.ExecContext(ctx,
		"INSERT INTO ledger_entries (transaction_id,line_id,account_id,direction,amount,currency,memo) VALUES ($1,$2,$3,$4,$5,$6,$7)",
		"constraint-tx",1,"constraint-account","DEBIT",100,"IDR",""); err != nil { t.Fatal(err) }

	_, err := db.ExecContext(ctx,
		"INSERT INTO ledger_entries (transaction_id,line_id,account_id,direction,amount,currency,memo) VALUES ($1,$2,$3,$4,$5,$6,$7)",
		"constraint-tx",2,"constraint-account","INVALID",100,"IDR","")
	if err == nil { t.Fatal("schema must reject invalid ledger direction") }

	_, err = db.ExecContext(ctx,
		"INSERT INTO ledger_entries (transaction_id,line_id,account_id,direction,amount,currency,memo) VALUES ($1,$2,$3,$4,$5,$6,$7)",
		"constraint-tx",3,"constraint-account","CREDIT",100,"USD","")
	if err == nil { t.Fatal("schema must reject ledger-entry currency mismatch") }
}
