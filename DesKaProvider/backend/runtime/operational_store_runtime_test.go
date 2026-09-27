package runtime

import (
 "context"
 "database/sql"
 "os"
 "path/filepath"
 "testing"
 "time"

 _ "github.com/jackc/pgx/v5/stdlib"
)

func TestLoadConfigDefaultsOperationalStoreToJSON(t *testing.T) {
 t.Setenv("DESKAPROVIDER_OPERATIONAL_STORE_DRIVER", "")
 t.Setenv("DESKAPROVIDER_TRANSACTION_STORE_DRIVER", "")
 t.Setenv("DESKAPROVIDER_AUDIT_STORE_DRIVER", "")
 t.Setenv("DESKAPROVIDER_POSTGRES_DSN", "")
 cfg, err := LoadConfig()
 if err != nil { t.Fatal(err) }
 if cfg.OperationalStoreDriver != "json" { t.Fatalf("expected JSON operational store default, got %q", cfg.OperationalStoreDriver) }
}

func TestLoadConfigRejectsPostgresOperationalStoreWithoutDSN(t *testing.T) {
 t.Setenv("DESKAPROVIDER_OPERATIONAL_STORE_DRIVER", "postgres")
 t.Setenv("DESKAPROVIDER_POSTGRES_DSN", "")
 _, err := LoadConfig()
 if err == nil { t.Fatal("expected missing PostgreSQL DSN error") }
 if err.Error() != "DESKAPROVIDER_POSTGRES_DSN is required when DESKAPROVIDER_OPERATIONAL_STORE_DRIVER=postgres" { t.Fatalf("unexpected error: %v", err) }
}

func TestOpenOperationalStoreJSONDoesNotRequirePostgres(t *testing.T) {
 path := filepath.Join(t.TempDir(), "snapshots.json")
 store, db, err := openOperationalStore(context.Background(), Config{OperationalStoreDriver: "json", StorePath: path}, nil)
 if err != nil { t.Fatal(err) }
 if store == nil { t.Fatal("expected operational store") }
 if db != nil { t.Fatal("JSON operational store must not open a database") }
}

func TestOpenOperationalStorePostgresDoesNotFallbackToJSON(t *testing.T) {
 ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
 defer cancel()
 _, db, err := openOperationalStore(ctx, Config{OperationalStoreDriver: "postgres", PostgresDSN: "postgres://invalid:invalid@127.0.0.1:1/invalid?sslmode=disable"}, nil)
 if err == nil { t.Fatal("expected PostgreSQL initialization failure") }
 if db != nil { t.Fatal("failed PostgreSQL initialization must not return a database handle") }
}

func TestOpenOperationalStorePostgresRequiresSchema(t *testing.T) {
 dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
 if dsn == "" { t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured") }
 db, err := sql.Open("pgx", dsn)
 if err != nil { t.Fatal(err) }
 defer db.Close()
 if err := db.Ping(); err != nil { t.Fatal(err) }
 if _, err = db.Exec("DROP TABLE IF EXISTS provider_operational_snapshots"); err != nil { t.Fatal(err) }
 t.Cleanup(func() {
  _, _ = db.Exec("CREATE TABLE IF NOT EXISTS provider_operational_snapshots (provider_name TEXT PRIMARY KEY, balance BIGINT NOT NULL, currency TEXT NOT NULL, health TEXT NOT NULL, last_checked_at TIMESTAMPTZ NOT NULL, last_success_at TIMESTAMPTZ NOT NULL, last_error TEXT NOT NULL DEFAULT '', consecutive_failures INTEGER NOT NULL DEFAULT 0)")
 })
 _, operationalDB, err := openOperationalStore(context.Background(), Config{OperationalStoreDriver: "postgres", PostgresDSN: dsn}, nil)
 if err == nil { t.Fatal("expected schema readiness failure") }
 if operationalDB != nil { t.Fatal("schema readiness failure must not return an owned database handle") }
}


func TestLoadConfigRejectsInvalidPostgresSchemaMode(t *testing.T) {
 t.Setenv("DESKAPROVIDER_OPERATIONAL_STORE_DRIVER", "json")
 t.Setenv("DESKAPROVIDER_TRANSACTION_STORE_DRIVER", "json")
 t.Setenv("DESKAPROVIDER_AUDIT_STORE_DRIVER", "memory")
 t.Setenv("DESKAPROVIDER_POSTGRES_DSN", "")
 t.Setenv("DESKAPROVIDER_POSTGRES_SCHEMA_MODE", "unsafe")
 if _, err := LoadConfig(); err == nil { t.Fatal("expected invalid PostgreSQL schema mode error") }
}

func TestOpenOperationalStorePostgresMigrateModeAppliesSchema(t *testing.T) {
 dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
 if dsn == "" { t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured") }
 db, err := sql.Open("pgx", dsn); if err != nil { t.Fatal(err) }
 defer db.Close()
 ctx := context.Background()
 if err := db.PingContext(ctx); err != nil { t.Fatal(err) }
 schema := "runtime_operational_migrate_" + time.Now().UTC().Format("20060102150405.000000000")
 schema = strings.ReplaceAll(schema, ".", "_")
 if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil { t.Fatal(err) }
 defer func(){ _, _ = db.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE") }()
 if _, err := db.ExecContext(ctx, "SET search_path TO "+schema); err != nil { t.Fatal(err) }
 store, owned, err := openOperationalStore(ctx, Config{OperationalStoreDriver:"postgres",PostgresDSN:dsn,PostgresSchemaMode:"migrate"}, nil)
 if err != nil { t.Fatal(err) }
 if store == nil || owned == nil { t.Fatal("expected PostgreSQL operational store and owned DB") }
 if err := checkOperationalSchema(ctx, owned); err != nil { t.Fatal(err) }
 _ = owned.Close()
}
