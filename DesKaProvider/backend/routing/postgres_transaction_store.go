package routing

import (
 "context"
 "database/sql"
 "errors"
 "fmt"

 provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
 payment "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/internal/Payment"
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

const postgresGetSQL = "SELECT transaction_kind, reference_id, product_code, customer_no, amount, testing, provider_name, status, provider_code, message, serial_number, price, payment_provider_reference, payment_currency, payment_customer_id, payment_description, version, created_at, updated_at FROM provider_transactions WHERE reference_id = $1"
const postgresAllSQL = "SELECT transaction_kind, reference_id, product_code, customer_no, amount, testing, provider_name, status, provider_code, message, serial_number, price, payment_provider_reference, payment_currency, payment_customer_id, payment_description, version, created_at, updated_at FROM provider_transactions ORDER BY created_at, reference_id"
const postgresInsertSQL = "INSERT INTO provider_transactions (transaction_kind, reference_id, product_code, customer_no, amount, testing, provider_name, status, provider_code, message, serial_number, price, payment_provider_reference, payment_currency, payment_customer_id, payment_description, version) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17) ON CONFLICT (reference_id) DO NOTHING RETURNING reference_id"
const postgresTransitionSQL = "UPDATE provider_transactions SET status=$2, provider_code=$3, message=$4, serial_number=$5, price=$6, payment_provider_reference=$7, version=version+1, updated_at=CURRENT_TIMESTAMP WHERE reference_id=$1 AND version=$8 AND transaction_kind=$9 AND product_code=$10 AND customer_no=$11 AND provider_name=$12 AND status='pending'"

func (s *PostgresTransactionStore) CreateIfAbsentContext(ctx context.Context, state TransactionState) (TransactionState, bool, error) {
	if err := validatePostgresState(state); err != nil {
		return TransactionState{}, false, err
	}
	var insertedReference string
	err := s.db.QueryRowContext(ctx, postgresInsertSQL,
		string(normalizeTransactionKind(state.Kind)), transactionReferenceID(state), state.Request.ProductCode, state.Request.CustomerNo,
		state.Request.Amount, state.Request.Testing, state.Execution.ProviderName,
		state.Execution.Result.Status, state.Execution.Result.ProviderCode,
		state.Execution.Result.Message, state.Execution.Result.SerialNumber,
		state.Execution.Result.Price, paymentProviderReference(state), paymentCurrency(state),
		paymentCustomerID(state), paymentDescription(state), 1).Scan(&insertedReference)
	if err == nil {
		state.Version = 1
		return state, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return TransactionState{}, false, wrapTransactionPersistenceAmbiguous(fmt.Errorf("create transaction: %w", err))
	}
	current, ok, readErr := s.GetContextE(ctx, transactionReferenceID(state))
	if readErr != nil {
		return TransactionState{}, false, fmt.Errorf("reload existing transaction after create race: %w", readErr)
	}
	if !ok {
		return TransactionState{}, false, ErrTransactionStateConflict
	}
	if !sameTransactionIdentity(current, state) {
		return TransactionState{}, false, ErrReferenceConflict
	}
	return current, false, nil
}

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
 current, ok, readErr := s.GetContextE(ctx, transactionReferenceID(state))
 if readErr != nil {
  return fmt.Errorf("get transaction: %w", readErr)
 }
 if !ok {
  var insertedReference string
  err := s.db.QueryRowContext(ctx, postgresInsertSQL, string(normalizeTransactionKind(state.Kind)), transactionReferenceID(state), state.Request.ProductCode, state.Request.CustomerNo, state.Request.Amount, state.Request.Testing, state.Execution.ProviderName, state.Execution.Result.Status, state.Execution.Result.ProviderCode, state.Execution.Result.Message, state.Execution.Result.SerialNumber, state.Execution.Result.Price, paymentProviderReference(state), paymentCurrency(state), paymentCustomerID(state), paymentDescription(state), 1).Scan(&insertedReference)
  if err == nil {
   return nil
  }
  if errors.Is(err, sql.ErrNoRows) {
   current, ok, readErr = s.GetContextE(ctx, transactionReferenceID(state))
   if readErr != nil {
    return fmt.Errorf("get transaction after insert race: %w", readErr)
   }
   if !ok {
    return ErrTransactionStateConflict
   }
   if validateTransactionTransition(current, state) == nil && sameTransactionIdentity(current, state) && sameTransactionObservedResult(current, state) {
    return nil
   }
   return ErrTransactionStateConflict
  }
  return wrapTransactionPersistenceAmbiguous(fmt.Errorf("insert transaction: %w", err))
 }
 if err := validateTransactionTransition(current, state); err != nil { return err }
 if sameTransactionObservedResult(current, state) {
  return nil
 }
 if current.Execution.Result.Status != provider.StatusPending {
  return ErrReferenceConflict
 }
 if state.Version != 0 && state.Version != current.Version { return ErrTransactionStateConflict }
 next := state
 result, err := s.db.ExecContext(ctx, postgresTransitionSQL,
  transactionReferenceID(state), next.Execution.Result.Status, next.Execution.Result.ProviderCode,
  next.Execution.Result.Message, next.Execution.Result.SerialNumber, next.Execution.Result.Price,
  paymentProviderReference(next), current.Version, string(normalizeTransactionKind(current.Kind)),
  current.Request.ProductCode, current.Request.CustomerNo, current.Execution.ProviderName)
 if err != nil { return wrapTransactionPersistenceAmbiguous(fmt.Errorf("update transaction: %w", err)) }
 n, err := result.RowsAffected()
 if err != nil { return wrapTransactionPersistenceAmbiguous(fmt.Errorf("read transaction update result: %w", err)) }
 if n != 1 { return ErrTransactionStateConflict }
 return nil
}

func (s *PostgresTransactionStore) Put(state TransactionState) error {
 return s.PutContext(context.Background(), state)
}

func (s *PostgresTransactionStore) PutIfCurrentContext(ctx context.Context, referenceID string, previous, next TransactionState) error {
 if referenceID == "" || transactionReferenceID(previous) != referenceID || transactionReferenceID(next) != referenceID { return ErrReferenceConflict }
 if !sameTransactionIdentity(previous, next) { return ErrReferenceConflict }
 if err := validatePostgresState(next); err != nil { return err }
 if previous.Execution.Result.Status != provider.StatusPending {
  if samePurchaseResult(previous.Execution.Result, next.Execution.Result) && previous.Request == next.Request && previous.Execution.ProviderName == next.Execution.ProviderName { return nil }
  return ErrReferenceConflict
 }
 if next.Execution.Result.Status != provider.StatusPending && next.Execution.Result.Status != provider.StatusSuccess && next.Execution.Result.Status != provider.StatusFailed { return ErrReferenceConflict }
 result, err := s.db.ExecContext(ctx, postgresTransitionSQL, referenceID, next.Execution.Result.Status, next.Execution.Result.ProviderCode, next.Execution.Result.Message, next.Execution.Result.SerialNumber, next.Execution.Result.Price, paymentProviderReference(next), previous.Version, string(normalizeTransactionKind(previous.Kind)), previous.Request.ProductCode, previous.Request.CustomerNo, previous.Execution.ProviderName)
 if err != nil { return wrapTransactionPersistenceAmbiguous(fmt.Errorf("atomic transaction transition: %w", err)) }
 n, err := result.RowsAffected()
 if err != nil { return wrapTransactionPersistenceAmbiguous(fmt.Errorf("read atomic transition result: %w", err)) }
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

func validatePostgresState(state TransactionState) error { return validateTransactionState(state) }
type postgresScanner interface { Scan(...any) error }
func scanPostgresState(s postgresScanner) (TransactionState, error) {
 var kind, ref, productCode, customerNo, providerName, status, providerCode, message, serial string
 var amount, price, version int64
 var paymentRef, paymentCurrency, paymentCustomerID, paymentDescription string
 var testing bool
 var createdAt, updatedAt any
 if err := s.Scan(&kind,&ref,&productCode,&customerNo,&amount,&testing,&providerName,&status,&providerCode,&message,&serial,&price,&paymentRef,&paymentCurrency,&paymentCustomerID,&paymentDescription,&version,&createdAt,&updatedAt); err != nil { return TransactionState{}, err }
 stateKind := TransactionKind(kind)
 if stateKind == TransactionKindPPOB { stateKind = "" }
 state := TransactionState{Kind:stateKind,Request:PurchaseRequest{ReferenceID:ref,ProductCode:productCode,CustomerNo:customerNo,Amount:amount,Testing:testing},Execution:PurchaseExecution{ProviderName:providerName,Result:provider.PurchaseResult{ReferenceID:ref,ProductCode:productCode,CustomerNo:customerNo,Status:provider.TransactionStatus(status),ProviderCode:providerCode,Message:message,SerialNumber:serial,Price:price}},Version:version}
 if normalizeTransactionKind(state.Kind) == TransactionKindPayment {
  state.Payment=&payment.Transaction{ReferenceID:ref,ProviderReference:paymentRef,Amount:amount,Currency:paymentCurrency,CustomerID:paymentCustomerID,Description:paymentDescription,Status:payment.Status(status),Message:message}
 }
 return state,nil
}
