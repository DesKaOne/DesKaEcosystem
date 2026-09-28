package routing

import (
	"context"
	"errors"
	"testing"
	payment "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/internal/Payment"
)

func TestPaymentTransactionCorrelationUsesExistingDurableBoundary(t *testing.T) {
	store:=NewMemoryTransactionStore()
	req:=payment.PaymentRequest{ReferenceID:"pay-ref-1",Amount:150000,Currency:"IDR",CustomerID:"cust-1",Description:"test payment"}
	state,err:=NewPaymentTransactionState(req,"midtrans"); if err!=nil { t.Fatal(err) }
	claimed,created,err:=store.CreateIfAbsentContext(context.Background(),state)
	if err!=nil || !created || claimed.Payment==nil || claimed.Payment.Status!=payment.StatusPending { t.Fatalf("expected pending claim: created=%v state=%#v err=%v",created,claimed,err) }
	repeated,created,err:=store.CreateIfAbsentContext(context.Background(),state)
	if err!=nil || created || repeated.Payment.ReferenceID!=req.ReferenceID { t.Fatalf("expected idempotent claim: created=%v state=%#v err=%v",created,repeated,err) }
	mismatch:=state
	mismatch.Payment=&payment.Transaction{ReferenceID:req.ReferenceID,Amount:160000,Currency:"IDR",CustomerID:"cust-1",Description:"test payment",Status:payment.StatusPending}
	if _,_,err:=store.CreateIfAbsentContext(context.Background(),mismatch); !errors.Is(err,ErrReferenceConflict) { t.Fatalf("expected reference conflict, got %v",err) }
}

func TestPaymentTransactionCASAndTerminalImmutability(t *testing.T) {
	store:=NewMemoryTransactionStore()
	state,err:=NewPaymentTransactionState(payment.PaymentRequest{ReferenceID:"pay-ref-2",Amount:20000,Currency:"IDR",CustomerID:"cust-2"},"midtrans")
	if err!=nil { t.Fatal(err) }
	claimed,created,err:=store.CreateIfAbsentContext(context.Background(),state); if err!=nil || !created { t.Fatalf("create pending: created=%v err=%v",created,err) }
	next:=claimed
	next.Payment=&payment.Transaction{ReferenceID:claimed.Payment.ReferenceID,ProviderReference:"mid-123",Amount:claimed.Payment.Amount,Currency:claimed.Payment.Currency,CustomerID:claimed.Payment.CustomerID,Description:claimed.Payment.Description,Status:payment.StatusSuccess,Message:"settlement"}
	next.Version=claimed.Version+1
	if err:=store.PutIfCurrentContext(context.Background(),claimed.Payment.ReferenceID,claimed,next); err!=nil { t.Fatalf("payment CAS transition: %v",err) }
	conflict:=next
	conflict.Payment=&payment.Transaction{ReferenceID:"pay-ref-2",ProviderReference:"mid-456",Amount:20000,Currency:"IDR",CustomerID:"cust-2",Status:payment.StatusSuccess}
	if err:=store.PutIfCurrent("pay-ref-2",next,conflict); !errors.Is(err,ErrReferenceConflict) { t.Fatalf("expected terminal mutation rejection, got %v",err) }
}
