package migrations

import (
 "context"
 "database/sql"
 "embed"
 "fmt"
)

//go:embed 001_provider_transactions.sql 002_provider_operational_snapshots.sql
var files embed.FS

type Definition struct { Version int; Name string; SQL string }

func Definitions() ([]Definition, error) {
 names := []struct{version int; name string}{
  {1,"001_provider_transactions.sql"},
  {2,"002_provider_operational_snapshots.sql"},
 }
 out := make([]Definition,0,len(names))
 for _, item := range names {
  b, err := files.ReadFile(item.name)
  if err != nil { return nil, fmt.Errorf("read migration %d: %w", item.version, err) }
  out = append(out, Definition{Version:item.version,Name:item.name,SQL:string(b)})
 }
 return out,nil
}

func Apply(ctx context.Context, db *sql.DB, versions ...int) error {
 if db == nil { return fmt.Errorf("migration database is required") }
 if err := ctx.Err(); err != nil { return err }
 if _, err := db.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS provider_schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP)"); err != nil {
  return fmt.Errorf("create schema migration ledger: %w", err)
 }
 wanted := map[int]bool{}
 for _, v := range versions { wanted[v]=true }
 defs, err := Definitions(); if err != nil { return err }
 for _, d := range defs {
  if !wanted[d.Version] { continue }
  var name string
  err := db.QueryRowContext(ctx, "SELECT name FROM provider_schema_migrations WHERE version = $1", d.Version).Scan(&name)
  if err == nil {
   if name != d.Name { return fmt.Errorf("migration version %d name mismatch: stored %q expected %q", d.Version, name, d.Name) }
   continue
  }
  if err != sql.ErrNoRows { return fmt.Errorf("check migration %d: %w", d.Version, err) }
  tx, err := db.BeginTx(ctx,nil); if err != nil { return fmt.Errorf("begin migration %d: %w", d.Version, err) }
  if _, err = tx.ExecContext(ctx,d.SQL); err != nil { _=tx.Rollback(); return fmt.Errorf("apply migration %d: %w", d.Version, err) }
  if _, err = tx.ExecContext(ctx, "INSERT INTO provider_schema_migrations (version,name) VALUES ($1,$2)", d.Version,d.Name); err != nil { _=tx.Rollback(); return fmt.Errorf("record migration %d: %w", d.Version, err) }
  if err = tx.Commit(); err != nil { return fmt.Errorf("commit migration %d: %w", d.Version, err) }
 }
 return nil
}
