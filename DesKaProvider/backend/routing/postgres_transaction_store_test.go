package routing

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

type postgresStoreExecResult struct{ rows int64 }
func (r postgresStoreExecResult) LastInsertId() (int64,error) { return 0,nil }
func (r postgresStoreExecResult) RowsAffected() (int64,error) { return r.rows,nil }

type postgresStoreDBStub struct {
	result sql.Result
	err error
	query string
	args []any
}
func (s *postgresStoreDBStub) ExecContext(_ context.Context, query string, args ...any) (sql.Result,error) {
	s.query=query; s.args=args
	if s.err != nil { return nil,s.err }
	return s.result,nil
}
func (s *postgresStoreDBStub) QueryContext(context.Context,string,...any) (*sql.Rows,error) { return nil, errors.New("not used") }
func (s *postgresStoreDBStub) QueryRowContext(context.Context,string,...any) *sql.Row { panic("not used") }

func postgresPendingState() TransactionState {
	return TransactionState{
		Request: PurchaseRequest{ReferenceID:"ref-77",ProductCode:"pln20",CustomerNo:"0812",Amount:20000},
		Execution: PurchaseExecution{ProviderName:"mock",Result:provider.PurchaseResult{
			ReferenceID:"ref-77",ProductCode:"pln20",CustomerNo:"0812",Status:provider.StatusPending,
		}},
	}
}

func TestPostgresTransactionStorePutIfCurrentSuccess(t *testing.T) {
	stub:=&postgresStoreDBStub{result:postgresStoreExecResult{rows:1}}
	store,err:=NewPostgresTransactionStore(stub)
	if err!=nil { t.Fatal(err) }
	prev:=postgresPendingState()
	next:=prev
	next.Execution.Result.Status=provider.StatusSuccess
	next.Execution.Result.ProviderCode="00"
	if err:=store.PutIfCurrent("ref-77",prev,next); err!=nil { t.Fatal(err) }
	if stub.query != postgresTransitionSQL { t.Fatalf("unexpected SQL: %s",stub.query) }
}

func TestPostgresTransactionStorePutIfCurrentConflictOnZeroRows(t *testing.T) {
	stub:=&postgresStoreDBStub{result:postgresStoreExecResult{rows:0}}
	store,err:=NewPostgresTransactionStore(stub)
	if err!=nil { t.Fatal(err) }
	prev:=postgresPendingState()
	next:=prev
	next.Execution.Result.Status=provider.StatusFailed
	err=store.PutIfCurrent("ref-77",prev,next)
	if !errors.Is(err,ErrTransactionStateConflict) { t.Fatalf("expected state conflict, got %v",err) }
}

func TestPostgresTransactionStorePutIfCurrentRejectsIdentityMismatch(t *testing.T) {
	stub:=&postgresStoreDBStub{result:postgresStoreExecResult{rows:1}}
	store,err:=NewPostgresTransactionStore(stub)
	if err!=nil { t.Fatal(err) }
	prev:=postgresPendingState()
	next:=prev
	next.Request.CustomerNo="0999"
	next.Execution.Result.Status=provider.StatusSuccess
	err=store.PutIfCurrent("ref-77",prev,next)
	if !errors.Is(err,ErrReferenceConflict) { t.Fatalf("expected reference conflict, got %v",err) }
}

func TestPostgresTransactionStorePutContextPropagatesDatabaseError(t *testing.T) {
	stub := &contextReadDBStub{err: errors.New("database unavailable")}
	store, err := NewPostgresTransactionStore(stub)
	if err != nil { t.Fatal(err) }
	state := postgresPendingState()

	if err := store.PutContext(context.Background(), state); err == nil || !errors.Is(err, stub.err) {
		t.Fatalf("expected underlying database error to propagate, got %v", err)
	}
}
