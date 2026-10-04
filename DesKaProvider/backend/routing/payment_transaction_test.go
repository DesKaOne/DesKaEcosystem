package routing
import("context";"errors";"testing";payment "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/internal/Payment")
func TestPaymentTransactionCorrelationUsesExistingDurableBoundary(t *testing.T){
 s:=NewMemoryTransactionStore();req:=payment.PaymentRequest{ReferenceID:"pay-ref-1",Amount:150000,Currency:"IDR",CustomerID:"cust-1",Description:"test payment"}
 state,err:=NewPaymentTransactionState(req,"midtrans");if err!=nil{t.Fatal(err)}
 claimed,created,err:=s.CreateIfAbsentContext(context.Background(),state);if err!=nil||!created||claimed.Payment==nil||claimed.Payment.Status!=payment.StatusPending{t.Fatalf("expected pending claim: %#v %v %v",claimed,created,err)}
 repeated,created,err:=s.CreateIfAbsentContext(context.Background(),state);if err!=nil||created||repeated.Payment.ReferenceID!=req.ReferenceID{t.Fatalf("expected idempotent claim: %#v %v %v",repeated,created,err)}
 mismatch:=state;mismatch.Payment=&payment.Transaction{ReferenceID:req.ReferenceID,Amount:160000,Currency:"IDR",CustomerID:"cust-1",Description:"test payment",Status:payment.StatusPending}
 if _,_,err:=s.CreateIfAbsentContext(context.Background(),mismatch);!errors.Is(err,ErrReferenceConflict){t.Fatalf("expected conflict, got %v",err)}
}
func TestPaymentTransactionCASAndTerminalImmutability(t *testing.T){
 s:=NewMemoryTransactionStore();state,err:=NewPaymentTransactionState(payment.PaymentRequest{ReferenceID:"pay-ref-2",Amount:20000,Currency:"IDR",CustomerID:"cust-2"},"midtrans");if err!=nil{t.Fatal(err)}
 claimed,created,err:=s.CreateIfAbsentContext(context.Background(),state);if err!=nil||!created{t.Fatal(err)}
 next:=claimed;next.Payment=&payment.Transaction{ReferenceID:"pay-ref-2",ProviderReference:"mid-123",Amount:20000,Currency:"IDR",CustomerID:"cust-2",Status:payment.StatusSuccess,Message:"settlement"};next.Version=2
 if err:=s.PutIfCurrentContext(context.Background(),"pay-ref-2",claimed,next);err!=nil{t.Fatal(err)}
 conflict:=next;conflict.Payment=&payment.Transaction{ReferenceID:"pay-ref-2",ProviderReference:"mid-456",Amount:20000,Currency:"IDR",CustomerID:"cust-2",Status:payment.StatusSuccess,Message:"settlement"}
 if err:=s.PutIfCurrent("pay-ref-2",next,conflict);!errors.Is(err,ErrReferenceConflict){t.Fatalf("expected terminal mutation rejection, got %v",err)}
}
