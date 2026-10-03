package accounting

import (
 "context"
 "errors"
 "testing"
 "time"

 "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/routing"
 payment "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/internal/Payment"
 provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

func TestPostPersistedTransactionSettlementRequiresTerminalSuccess(t *testing.T) {
 store:=routing.NewMemoryTransactionStore()
 poster,_:=NewSettlementPoster(NewMemoryStore())
 state,err:=routing.NewPaymentTransactionState(payment.PaymentRequest{ReferenceID:"settle-1",Amount:10000,Currency:"IDR",CustomerID:"cust-1"},"mock")
 if err!=nil{t.Fatal(err)}
 if err:=store.Put(state);err!=nil{t.Fatal(err)}
 entries:=[]Entry{{LineID:1,AccountID:"provider-clearing",Direction:Debit,Amount:10000,Currency:"IDR"},{LineID:2,AccountID:"settlement-in",Direction:Credit,Amount:10000,Currency:"IDR"}}
 err=PostPersistedTransactionSettlement(context.Background(),store,poster,"settle-1","ledger-settle-1","PROVIDER_SETTLEMENT","IDR","settlement",time.Date(2026,10,3,10,0,0,0,time.UTC),entries)
 if !errors.Is(err,ErrSettlementNotPostable){t.Fatalf("pending transaction must not settle: %v",err)}
 success:=state
 success.Payment=&payment.Transaction{ReferenceID:"settle-1",Amount:10000,Currency:"IDR",CustomerID:"cust-1",Status:payment.StatusSuccess}
 success.Execution.Result.Status=provider.StatusSuccess
 if err:=store.Put(success);err!=nil{t.Fatal(err)}
 if err:=PostPersistedTransactionSettlement(context.Background(),store,poster,"settle-1","ledger-settle-1","PROVIDER_SETTLEMENT","IDR","settlement",time.Date(2026,10,3,10,0,0,0,time.UTC),entries);err!=nil{t.Fatal(err)}
}

func TestPostPersistedTransactionSettlementDoesNotInferAccountingInputs(t *testing.T) {
 store:=routing.NewMemoryTransactionStore()
 poster,_:=NewSettlementPoster(NewMemoryStore())
 state,err:=routing.NewPaymentTransactionState(payment.PaymentRequest{ReferenceID:"explicit-1",Amount:10000,Currency:"IDR",CustomerID:"cust-1"},"mock")
 if err!=nil{t.Fatal(err)}
 state.Payment.Status=payment.StatusSuccess
 state.Execution.Result.Status=provider.StatusSuccess
 if err:=store.Put(state);err!=nil{t.Fatal(err)}
 err=PostPersistedTransactionSettlement(context.Background(),store,poster,"explicit-1","","PROVIDER_SETTLEMENT","","",time.Date(2026,10,3,10,0,0,0,time.UTC),nil)
 if !errors.Is(err,ErrInvalidSettlement){t.Fatalf("missing explicit accounting inputs must be rejected: %v",err)}
}

func TestPostPersistedTransactionSettlementMissingTransaction(t *testing.T) {
 store:=routing.NewMemoryTransactionStore()
 poster,_:=NewSettlementPoster(NewMemoryStore())
 err:=PostPersistedTransactionSettlement(context.Background(),store,poster,"missing","ledger-missing","PROVIDER_SETTLEMENT","IDR","missing",time.Date(2026,10,3,10,0,0,0,time.UTC),[]Entry{{LineID:1,AccountID:"a",Direction:Debit,Amount:1,Currency:"IDR"},{LineID:2,AccountID:"b",Direction:Credit,Amount:1,Currency:"IDR"}})
 if !errors.Is(err,ErrSettlementTransactionNotFound){t.Fatalf("missing transaction must be explicit: %v",err)}
}
