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
var _ ContextTransactionStore = (*PostgresTransactionStore)(nil)
var _ ContextReadTransactionStore = (*PostgresTransactionStore)(nil)

const postgresGetSQL = "SELECT reference_id, product_code, customer_no, amount, testing, provider_name, status, provider_code, message, serial_number, price, version, created_at, updated_at FROM provider_transactions WHERE reference_id = $1"
const postgresAllSQL = "SELECT reference_id, product_code, customer_no, amount, testing, provider_name, status, provider_code, message, serial_number, price, version, created_at, updated_at FROM provider_transactions ORDER BY created_at, reference_id"
const postgresInsertSQL = "INSERT INTO provider_transactions (reference_id, product_code, customer_no, amount, testing, provider_name, status, provider_code, message, serial_number, price, version) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) ON CONFLICT (reference_id) DO NOTHING RETURNING reference_id"
const postgresTransitionSQL = "UPDATE provider_transactions SET status=$2, provider_code=$3, message=$4, serial_number=$5, price=$6, version=version+1, updated_at=CURRENT_TIMESTAMP WHERE reference_id=$1 AND version=$7 AND product_code=$8 AND customer_no=$9 AND provider_name=$10 AND status='pending'"

func (s *PostgresTransactionStore) GetContextE(ctx context.Context, referenceID string) (TransactionState, bool, error) {
 row := s.db.QueryRowContext(ctx, postgresGetSQL, referenceID)
 state, err := scanPostgresState(row)
 if errors.Is(err, sql.ErrNoRows) { return TransactionState{}, false, nil }
 if err != nil { return TransactionState{}, false, fmt.Errorf("get transaction: %w", err) }
 return state, true, nil
}

func (s *PostgresTransactionStore) GetContext(ctx context.Context, referenceID string) (TransactionState, bool) {
 state, ok, err := s.GetContextE(ctx, referenceID)
 if err != nil { return TransactionState{}, false }
 return state, ok
}

func (s *PostgresTransactionStore) Get(referenceID string) (TransactionState, bool) {
 return s.GetContext(context.Background(), referenceID)
}

func (s *PostgresTransactionStore) PutContext(ctx context.Context, state TransactionState) error {
 if err := validatePostgresState(state); err != nil { return err }
 current, ok := s.GetContext(ctx, state.Request.ReferenceID)
 if !ok {
  var insertedReference string
  err := s.db.QueryRowContext(ctx, postgresInsertSQL, state.Request.ReferenceID, state.Request.ProductCode, state.Request.CustomerNo, state.Request.Amount, state.Request.Testing, state.Execution.ProviderName, state.Execution.Result.Status, state.Execution.Result.ProviderCode, state.Execution.Result.Message, state.Execution.Result.SerialNumber, state.Execution.Result.Price, 1).Scan(&insertedReference)
  if err == nil {
   return nil
  }
  if errors.Is(err, sql.ErrNoRows) {
   current, ok = s.GetContext(ctx, state.Request.ReferenceID)
   if !ok {
    return ErrTransactionStateConflict
   }
   if validateTransactionTransition(current, state) == nil && current.Request == state.Request && current.Execution.ProviderName == state.Execution.ProviderName && samePurchaseResult(current.Execution.Result, state.Execution.Result) {
    return nil
   }
   return ErrTransactionStateConflict
  }
  return fmt.Errorf("insert transaction: %w", err)
 }
 if err := validateTransactionTransition(current, state); err != nil { return err }
 if samePurchaseResult(current.Execution.Result, state.Execution.Result) {
  return nil
 }
 if current.Execution.Result.Status != provider.StatusPending {
  return ErrReferenceConflict
 }
 if state.Version != 0 && state.Version != current.Version { return ErrTransactionStateConflict }
 next := state
 result, err := s.db.ExecContext(ctx, postgresTransitionSQL,
  state.Request.ReferenceID, next.Execution.Result.Status, next.Execution.Result.ProviderCode,
  next.Execution.Result.Message, next.Execution.Result.SerialNumber, next.Execution.Result.Price,
  current.Version, current.Request.ProductCode, current.Request.CustomerNo, current.Execution.ProviderName)
 if err != nil { return fmt.Errorf("update transaction: %w", err) }
 n, err := result.RowsAffected()
 if err != nil { return fmt.Errorf("read transaction update result: %w", err) }
 if n != 1 { return ErrTransactionStateConflict }
 return nil
}

func (s *PostgresTransactionStore) Put(state TransactionState) error {
 return s.PutContext(context.Background(), state)
}

func (s *PostgresTransactionStore) PutIfCurrentContext(ctx context.Context, referenceID string, previous, next TransactionState) error {
 if referenceID == "" || previous.Request.ReferenceID != referenceID || next.Request.ReferenceID != referenceID { return ErrReferenceConflict }
 if previous.Request != next.Request || previous.Execution.ProviderName != next.Execution.ProviderName { return ErrReferenceConflict }
 if err := validatePostgresState(next); err != nil { return err }
 if previous.Execution.Result.Status != provider.StatusPending {
  if samePurchaseResult(previous.Execution.Result, next.Execution.Result) && previous.Request == next.Request && previous.Execution.ProviderName == next.Execution.ProviderName { return nil }
  return ErrReferenceConflict
 }
 if next.Execution.Result.Status != provider.StatusPending && next.Execution.Result.Status != provider.StatusSuccess && next.Execution.Result.Status != provider.StatusFailed { return ErrReferenceConflict }
 result, err := s.db.ExecContext(ctx, postgresTransitionSQL, referenceID, next.Execution.Result.Status, next.Execution.Result.ProviderCode, next.Execution.Result.Message, next.Execution.Result.SerialNumber, next.Execution.Result.Price, previous.Version, previous.Request.ProductCode, previous.Request.CustomerNo, previous.Execution.ProviderName)
 if err != nil { return fmt.Errorf("atomic transaction transition: %w", err) }
 n, err := result.RowsAffected()
 if err != nil { return fmt.Errorf("read atomic transition result: %w", err) }
 if n != 1 { return ErrTransactionStateConflict }
 return nil
}

func (s *PostgresTransactionStore) PutIfCurrent(referenceID string, previous, next TransactionState) error {
 return s.PutIfCurrentContext(context.Background(), referenceID, previous, next)
}

func (s *PostgresTransactionStore) AllContextE(ctx context.Context) ([]TransactionState, error) {
 rows, err := s.db.QueryContext(ctx, postgresAllSQL)
 if err != nil { return nil, fmt.Errorf("list transactions: %w", err) }
 defer rows.Close()
 var result []TransactionState
 for rows.Next() {
  state, err := scanPostgresState(rows)
  if err != nil { return nil, fmt.Errorf("scan transaction: %w", err) }
  result = append(result, state)
 }
 if err := rows.Err(); err != nil { return nil, fmt.Errorf("iterate transactions: %w", err) }
 return result, nil
}

func (s *PostgresTransactionStore) AllContext(ctx context.Context) []TransactionState {
 result, err := s.AllContextE(ctx)
 if err != nil { return nil }
 return result
}

func (s *PostgresTransactionStore) All() []TransactionState {
 return s.AllContext(context.Background())
}

func validatePostgresState(state TransactionState) error { if state.Request.ReferenceID == "" || state.Execution.ProviderName == "" { return ErrReferenceConflict }; return nil }
type postgresScanner interface { Scan(...any) error }
func scanPostgresState(s postgresScanner) (TransactionState, error) {
 var ref, productCode, customerNo, providerName, status, providerCode, message, serial string
 var amount, price, version int64
 var testing bool
 var createdAt, updatedAt any
 if err := s.Scan(&ref,&productCode,&customerNo,&amount,&testing,&providerName,&status,&providerCode,&message,&serial,&price,&version,&createdAt,&updatedAt); err != nil { return TransactionState{}, err }
 return TransactionState{Request: PurchaseRequest{ReferenceID:ref,ProductCode:productCode,CustomerNo:customerNo,Amount:amount,Testing:testing},Execution:PurchaseExecution{ProviderName:providerName,Result:provider.PurchaseResult{ReferenceID:ref,ProductCode:productCode,CustomerNo:customerNo,Status:provider.TransactionStatus(status),ProviderCode:providerCode,Message:message,SerialNumber:serial,Price:price}},Version:version}, nil
}