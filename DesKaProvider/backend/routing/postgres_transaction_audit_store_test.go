package routing

import (
	"database/sql"
	"database/sql/driver"
	"context"
	"errors"
	"testing"
	"time"
	"io"
	"strconv"
)

func TestPostgresTransactionAuditStoreValidation(t *testing.T) {
	store, err := NewPostgresTransactionAuditStore(&postgresStoreDBStub{})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Append(TransactionAuditEvent{Action: "TEST", CreatedAt: time.Now().UTC()}); err == nil {
		t.Fatal("expected missing reference ID to be rejected")
	}
	if err := store.Append(TransactionAuditEvent{ReferenceID: "ref", Action: "TEST"}); err == nil {
		t.Fatal("expected missing created_at to be rejected")
	}
	if err := store.AppendContext(context.Background(), TransactionAuditEvent{ReferenceID: "ref", Action: "TEST"}); err == nil {
		t.Fatal("expected invalid event to be rejected before database access")
	}
}

func TestPostgresTransactionAuditStoreAppendContextPropagatesDatabaseError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	stub := &postgresStoreDBStub{err: wantErr}
	store, err := NewPostgresTransactionAuditStore(stub)
	if err != nil {
		t.Fatal(err)
	}

	err = store.AppendContext(context.Background(), TransactionAuditEvent{
		ReferenceID: "ref",
		Action: "PURCHASE_RESULT",
		CreatedAt: time.Now().UTC(),
	})
	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("expected database append error to propagate, got %v", err)
	}
	if errors.Is(err, context.Canceled) {
		t.Fatalf("database append error must not be reported as cancellation: %v", err)
	}
	if stub.query != postgresAuditAppendSQL {
		t.Fatalf("expected audit append SQL, got %q", stub.query)
	}
}

func TestPostgresTransactionAuditStoreAppendContextPropagatesCancellation(t *testing.T) {
	store, err := NewPostgresTransactionAuditStore(&postgresStoreDBStub{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = store.AppendContext(ctx, TransactionAuditEvent{
		ReferenceID: "ref",
		Action: "TEST",
		CreatedAt: time.Now().UTC(),
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

type postgresAuditRowsScenario struct {
	queryErr    error
	rows        [][]driver.Value
	scanErr     error
	rowsErr     error
	cancelAfter int
	cancel      context.CancelFunc
}

type postgresAuditRowsDriver struct {
	scenario *postgresAuditRowsScenario
}

func (d *postgresAuditRowsDriver) Open(string) (driver.Conn, error) {
	return &postgresAuditRowsConn{scenario: d.scenario}, nil
}

type postgresAuditRowsConn struct {
	scenario *postgresAuditRowsScenario
}

func (c *postgresAuditRowsConn) Prepare(string) (driver.Stmt, error) {
	return &postgresAuditRowsStmt{scenario: c.scenario}, nil
}

func (c *postgresAuditRowsConn) Close() error { return nil }
func (c *postgresAuditRowsConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions are not supported")
}

type postgresAuditRowsStmt struct {
	scenario *postgresAuditRowsScenario
}

func (s *postgresAuditRowsStmt) Close() error { return nil }
func (s *postgresAuditRowsStmt) NumInput() int { return -1 }

func (s *postgresAuditRowsStmt) Exec([]driver.Value) (driver.Result, error) {
	return postgresAuditRowsResult{}, nil
}

func (s *postgresAuditRowsStmt) Query([]driver.Value) (driver.Rows, error) {
	if s.scenario.queryErr != nil {
		return nil, s.scenario.queryErr
	}
	return &postgresAuditRowsRows{scenario: s.scenario, index: -1}, nil
}

type postgresAuditRowsRows struct {
	scenario *postgresAuditRowsScenario
	index    int
}

func (r *postgresAuditRowsRows) Columns() []string {
	return []string{"reference_id", "action", "previous_status", "next_status", "provider_name", "message", "created_at"}
}

func (r *postgresAuditRowsRows) Close() error { return nil }

func (r *postgresAuditRowsRows) Next(dest []driver.Value) error {
	r.index++
	if r.index >= len(r.scenario.rows) {
		if r.scenario.rowsErr != nil {
			return r.scenario.rowsErr
		}
		return io.EOF
	}
	if r.scenario.scanErr != nil {
		return r.scenario.scanErr
	}
	copy(dest, r.scenario.rows[r.index])
	if r.scenario.cancelAfter == r.index && r.scenario.cancel != nil {
		r.scenario.cancel()
		r.scenario.rowsErr = context.Canceled
	}
	return nil
}

type postgresAuditRowsResult struct{}

func (postgresAuditRowsResult) LastInsertId() (int64, error) { return 0, nil }
func (postgresAuditRowsResult) RowsAffected() (int64, error) { return 0, nil }

func openPostgresAuditRowsDB(t *testing.T, scenario *postgresAuditRowsScenario) *sql.DB {
	t.Helper()
	driverName := "deskaprovider_audit_rows_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	sql.Register(driverName, &postgresAuditRowsDriver{scenario: scenario})
	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestPostgresTransactionAuditStoreAllContextPropagatesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	scenario := &postgresAuditRowsScenario{
		rows: [][]driver.Value{{
			"ref", "TEST", nil, nil, "mock", "message", time.Now().UTC(),
		}},
	}
	db := openPostgresAuditRowsDB(t, scenario)
	store, err := NewPostgresTransactionAuditStore(db)
	if err != nil {
		t.Fatal(err)
	}

	result, err := store.AllContextE(ctx, "ref")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if result != nil {
		t.Fatalf("canceled audit read must not expose any history, got %#v", result)
	}
}

func TestPostgresTransactionAuditStoreAllContextRejectsMidStreamCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	scenario := &postgresAuditRowsScenario{
		rows: [][]driver.Value{
			{"ref", "TEST_1", "", "", "mock", "message-1", time.Now().UTC()},
			{"ref", "TEST_2", "", "", "mock", "message-2", time.Now().UTC().Add(time.Second)},
		},
		cancelAfter: 0,
		cancel:      cancel,
	}
	db := openPostgresAuditRowsDB(t, scenario)
	store, err := NewPostgresTransactionAuditStore(db)
	if err != nil {
		t.Fatal(err)
	}

	result, err := store.AllContextE(ctx, "ref")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected mid-stream cancellation, got %v", err)
	}
	if result != nil {
		t.Fatalf("mid-stream cancellation must not expose accumulated audit history, got %#v", result)
	}
}

func TestPostgresTransactionAuditStoreAllContextPropagatesDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	t.Cleanup(cancel)
	time.Sleep(time.Millisecond)

	scenario := &postgresAuditRowsScenario{
		rows: [][]driver.Value{{
			"ref", "TEST", "", "", "mock", "message", time.Now().UTC(),
		}},
	}
	db := openPostgresAuditRowsDB(t, scenario)
	store, err := NewPostgresTransactionAuditStore(db)
	if err != nil {
		t.Fatal(err)
	}

	result, err := store.AllContextE(ctx, "ref")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	if result != nil {
		t.Fatalf("deadline-exceeded audit read must not expose any history, got %#v", result)
	}
}

func TestPostgresTransactionAuditStoreAllContextPropagatesQueryError(t *testing.T) {
	wantErr := errors.New("audit query unavailable")
	db := openPostgresAuditRowsDB(t, &postgresAuditRowsScenario{queryErr: wantErr})
	store, err := NewPostgresTransactionAuditStore(db)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.AllContextE(context.Background(), "ref")
	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("expected query error to propagate, got %v", err)
	}
}

func TestPostgresTransactionAuditStoreAllContextPropagatesScanError(t *testing.T) {
	wantErr := errors.New("audit scan unavailable")
	scenario := &postgresAuditRowsScenario{
		rows: [][]driver.Value{{
			"ref", "TEST", nil, nil, "mock", "message", time.Now().UTC(),
		}},
		scanErr: wantErr,
	}
	db := openPostgresAuditRowsDB(t, scenario)
	store, err := NewPostgresTransactionAuditStore(db)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.AllContextE(context.Background(), "ref")
	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("expected scan error to propagate, got %v", err)
	}
}

func TestPostgresTransactionAuditStoreAllContextPropagatesRowsError(t *testing.T) {
	wantErr := errors.New("audit rows iteration failed")
	scenario := &postgresAuditRowsScenario{rowsErr: wantErr}
	db := openPostgresAuditRowsDB(t, scenario)
	store, err := NewPostgresTransactionAuditStore(db)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.AllContextE(context.Background(), "ref")
	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("expected rows iteration error to propagate, got %v", err)
	}
}
