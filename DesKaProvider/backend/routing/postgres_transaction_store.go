package routing

import (
 "context"
 "database/sql"
 "errors"
 "fmt"

 provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

type DBTX interface {
 ExecContext(context.Context, string, ...any) (sql.Result, error)
 QueryContext(context.Context, string, ...any) (*sql.Rows, error)
 QueryRowContext(context.Context, string, ...any) *sql.Row
}

type PostgresTransactionStore struct { db DBTX }

func NewPostgresTransactionStore(db DBTX) (*PostgresTransactionStore, error) {
 if db == nil { return nil, errors.New("postgres transaction store database is required") }
 return &PostgresTransactionStore{db: db}, nil
}

var _ AtomicTransactionStore = (*PostgresTransactionStore)(nil)

const postgresGetSQL = "SELECT reference_id, product_code, customer_no, amount, testing, provider_name, status, provider_code, message, serial_number, price, version, created_at, updated_at FROM provider_transactions WHERE reference_id = $1"
const postgresAllSQL = "SELECT reference_id, product_code, customer_no, amount, testing, provider_name, status, provider_code, message, serial_number, price, version, created_at, updated_at FROM provider_transactions ORDER BY created_at, reference_id"
const postgresInsertSQL = "INSERT INTO provider_transactions (reference_id, product_code, customer_no, amount, testing, provider_name, status, provider_code, message, serial_number, price, version) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)"
const postgresTransitionSQL = "UPDATE provider_transactions SET status=$2, provider_code=$3, message=$4, serial_number=$5, price=$6, version=version+1, updated_at=CURRENT_TIMESTAMP WHERE reference_id=$1 AND version=$7 AND product_code=$8 AND customer_no=$9 AND provider_name=$10 AND status='pending'"

func (s *PostgresTransactionStore) Get(ctx context.Context, referenceID string) (TransactionState, bool) {
 row := s.db.QueryRowContext(ctx, postgresGetSQL, referenceID)
 state, err := scanPostgresState(row)
 if errors.Is(err, sql.ErrNoRows) { return TransactionState{}, false }
 if err != nil { return TransactionState{}, false }
 return state, true
}

func (s *PostgresTransactionStore) Put(state TransactionState) error {
 if err := validatePostgresState(state); err != nil { return err }
 current, ok := s.Get(context.Background(), state.Request.ReferenceID)
 if !ok {
  _, err := s.db.ExecContext(context.Background(), postgresInsertSQL, state.Request.ReferenceID, state.Request.ProductCode, state.Request.CustomerNo, state.Request.Amount, state.Request.Testing, state.Execution.ProviderName, state.Execution.Result.Status, state.Execution.Result.ProviderCode, state.Execution.Result.Message, state.Execution.Result.SerialNumber, state.Execution.Result.Price, 1)
  if err != nil { return fmt.Errorf("insert transaction: %w", err) }
  return nil
 }
 if err := validateTransactionTransition(current, state); err != nil { return err }
 if samePurchaseResult(current.Execution.Result, state.Execution.Result) {
  return nil
 }
 if current.Execution.Result.Status != provider.StatusPending {
  return ErrReferenceConflict
 }
 next := state
 result, err := s.db.ExecContext(context.Background(), postgresTransitionSQL,
  state.Request.ReferenceID, next.Execution.Result.Status, next.Execution.Result.ProviderCode,
  next.Execution.Result.Message, next.Execution.Result.SerialNumber, next.Execution.Result.Price,
  1, current.Request.ProductCode, current.Request.CustomerNo, current.Execution.ProviderName)
 if err != nil { return fmt.Errorf("update transaction: %w", err) }
 n, err := result.RowsAffected()
 if err != nil { return fmt.Errorf("read transaction update result: %w", err) }
 if n != 1 { return ErrTransactionStateConflict }
 return nil
}

func (s *PostgresTransactionStore) PutIfCurrent(referenceID string, previous, next TransactionState) error {
 if referenceID == "" || previous.Request.ReferenceID != referenceID || next.Request.ReferenceID != referenceID { return ErrReferenceConflict }
 if err := validatePostgresState(next); err != nil { return err }
 if previous.Execution.Result.Status != provider.StatusPending {
  if samePurchaseResult(previous.Execution.Result, next.Execution.Result) && previous.Request == next.Request && previous.Execution.ProviderName == next.Execution.ProviderName { return nil }
  return ErrReferenceConflict
 }
 if next.Execution.Result.Status != provider.StatusPending && next.Execution.Result.Status != provider.StatusSuccess && next.Execution.Result.Status != provider.StatusFailed { return ErrReferenceConflict }
 result, err := s.db.ExecContext(context.Background(), postgresTransitionSQL, referenceID, next.Execution.Result.Status, next.Execution.Result.ProviderCode, next.Execution.Result.Message, next.Execution.Result.SerialNumber, next.Execution.Result.Price, 1, previous.Request.ProductCode, previous.Request.CustomerNo, previous.Execution.ProviderName)
 if err != nil { return fmt.Errorf("atomic transaction transition: %w", err) }
 n, err := result.RowsAffected()
 if err != nil { return fmt.Errorf("read atomic transition result: %w", err) }
 if n != 1 { return ErrTransactionStateConflict }
 return nil
}

func (s *PostgresTransactionStore) All() []TransactionState {
 rows, err := s.db.QueryContext(context.Background(), postgresAllSQL)
 if err != nil { return nil }
 defer rows.Close()
 var result []TransactionState
 for rows.Next() { state, err := scanPostgresState(rows); if err != nil { return nil }; result = append(result, state) }
 if err := rows.Err(); err != nil { return nil }
 return result
}

func validatePostgresState(state TransactionState) error { if state.Request.ReferenceID == "" || state.Execution.ProviderName == "" { return ErrReferenceConflict }; return nil }
type postgresScanner interface { Scan(...any) error }
func scanPostgresState(s postgresScanner) (TransactionState, error) {
 var ref, productCode, customerNo, providerName, status, providerCode, message, serial string
 var amount, price, version int64
 var testing bool
 var createdAt, updatedAt any
 if err := s.Scan(&ref,&productCode,&customerNo,&amount,&testing,&providerName,&status,&providerCode,&message,&serial,&price,&version,&createdAt,&updatedAt); err != nil { return TransactionState{}, err }
 return TransactionState{Request: PurchaseRequest{ReferenceID:ref,ProductCode:productCode,CustomerNo:customerNo,Amount:amount,Testing:testing},Execution:PurchaseExecution{ProviderName:providerName,Result:provider.PurchaseResult{ReferenceID:ref,ProductCode:productCode,CustomerNo:customerNo,Status:status,ProviderCode:providerCode,Message:message,SerialNumber:serial,Price:price}}}, nil
}