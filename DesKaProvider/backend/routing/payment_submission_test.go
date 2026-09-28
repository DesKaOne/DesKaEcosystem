package routing

import (
 "context"
 "errors"
 "testing"
 provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
 payment "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/internal/Payment"
)

type paymentSubmissionProvider struct {
 calls int
 statusCalls int
 lastStatusRequest payment.StatusRequest
 result payment.PaymentResult
 statusResult payment.StatusResult
 err error
}
func (p *paymentSubmissionProvider) CreatePayment(_ context.Context, req payment.PaymentRequest) (payment.PaymentResult,error) {
 p.calls++
 if p.err != nil { return payment.PaymentResult{}, p.err }
 return p.result,nil
}
func (p *paymentSubmissionProvider) GetPaymentStatus(_ context.Context, req payment.StatusRequest)(payment.StatusResult,error) {
 p.statusCalls++
 p.lastStatusRequest=req
 if p.statusResult.ReferenceID=="" { return payment.StatusResult{}, payment.ErrUnsupported }
 return p.statusResult,nil
}

func newPaymentSubmissionService(t *testing.T, p *paymentSubmissionProvider, enabled bool) *Service {
 t.Helper()
 reg:=provider.NewRegistry()
 if err:=reg.RegisterCapabilityProvider("midtrans",provider.CapabilityPayment,p,provider.CapabilityStatus{AdapterImplemented:true,Tested:true,Enabled:enabled});err!=nil{t.Fatal(err)}
 return &Service{Router:&Router{Registry:reg},Store:NewMemoryTransactionStore(),AuditStore:NewMemoryTransactionAuditStore()}
}

func TestSubmitPaymentClaimsReferenceBeforeExternalSubmission(t *testing.T){
 p:=&paymentSubmissionProvider{result:payment.PaymentResult{ReferenceID:"pay-submit-1",ProviderReference:"mid-1",Status:payment.StatusPending,Amount:10000,Currency:"IDR",Message:"token"}}
 s:=newPaymentSubmissionService(t,p,true)
 req:=payment.PaymentRequest{ReferenceID:"pay-submit-1",Amount:10000,Currency:"IDR",CustomerID:"cust-1",Description:"checkout"}
 got,err:=s.SubmitPayment(context.Background(),"midtrans",req)
 if err!=nil{t.Fatal(err)}
 if p.calls!=1||got.ProviderReference!="mid-1"{t.Fatalf("unexpected first submission: calls=%d result=%#v",p.calls,got)}
 _,err=s.SubmitPayment(context.Background(),"midtrans",req)
 if !errors.Is(err,ErrPaymentSubmissionClaimed){t.Fatalf("expected durable claim rejection, got %v",err)}
 if p.calls!=1{t.Fatalf("expected exactly one external submission, calls=%d",p.calls)}
 state,ok:=s.Store.Get(req.ReferenceID)
 if !ok||state.Payment==nil||state.Payment.ProviderReference!="mid-1"{t.Fatalf("expected durable provider reference: %#v",state)}
}

func TestSubmitPaymentExistingReferenceWithDifferentIdentityDoesNotSubmit(t *testing.T){
 p:=&paymentSubmissionProvider{result:payment.PaymentResult{ReferenceID:"pay-submit-2",ProviderReference:"mid-2",Status:payment.StatusPending,Amount:20000,Currency:"IDR"}}
 s:=newPaymentSubmissionService(t,p,true)
 first:=payment.PaymentRequest{ReferenceID:"pay-submit-2",Amount:20000,Currency:"IDR",CustomerID:"cust-2"}
 if _,err:=s.SubmitPayment(context.Background(),"midtrans",first);err!=nil{t.Fatal(err)}
 mismatch:=first;mismatch.Amount=21000
 if _,err:=s.SubmitPayment(context.Background(),"midtrans",mismatch);!errors.Is(err,ErrPaymentSubmissionConflict){t.Fatalf("expected identity conflict, got %v",err)}
 if p.calls!=1{t.Fatalf("expected no resubmission, calls=%d",p.calls)}
}

func TestSubmitPaymentProviderErrorLeavesClaimAndForbidsRetry(t *testing.T){
 p:=&paymentSubmissionProvider{err:errors.New("ambiguous provider timeout")}
 s:=newPaymentSubmissionService(t,p,true)
 req:=payment.PaymentRequest{ReferenceID:"pay-submit-3",Amount:30000,Currency:"IDR",CustomerID:"cust-3"}
 if _,err:=s.SubmitPayment(context.Background(),"midtrans",req);err==nil{t.Fatal("expected provider error")}
 if p.calls!=1{t.Fatalf("expected one provider call, calls=%d",p.calls)}
 if _,err:=s.SubmitPayment(context.Background(),"midtrans",req);!errors.Is(err,ErrPaymentSubmissionClaimed){t.Fatalf("expected retry rejection after provider error, got %v",err)}
 if p.calls!=1{t.Fatalf("expected no automatic resubmission, calls=%d",p.calls)}
}

func TestSubmitPaymentRequiresEnabledCapability(t *testing.T){
 p:=&paymentSubmissionProvider{result:payment.PaymentResult{ReferenceID:"pay-submit-4",ProviderReference:"mid-4",Status:payment.StatusPending,Amount:40000,Currency:"IDR"}}
 s:=newPaymentSubmissionService(t,p,false)
 req:=payment.PaymentRequest{ReferenceID:"pay-submit-4",Amount:40000,Currency:"IDR",CustomerID:"cust-4"}
 if _,err:=s.SubmitPayment(context.Background(),"midtrans",req);!errors.Is(err,ErrPaymentCapabilityDisabled){t.Fatalf("expected disabled capability, got %v",err)}
 if p.calls!=0{t.Fatalf("disabled capability must not call provider, calls=%d",p.calls)}
}

func TestNewServiceLoadsPersistedPaymentStateWithoutTreatingItAsPPOB(t *testing.T) {
	p := &paymentSubmissionProvider{result: payment.PaymentResult{
		ReferenceID:      "pay-submit-restart",
		ProviderReference: "mid-restart",
		Status:           payment.StatusPending,
		Amount:           50000,
		Currency:         "IDR",
	}}
	s := newPaymentSubmissionService(t, p, true)
	req := payment.PaymentRequest{
		ReferenceID: "pay-submit-restart",
		Amount:      50000,
		Currency:    "IDR",
		CustomerID:  "cust-restart",
	}
	if _, err := s.SubmitPayment(context.Background(), "midtrans", req); err != nil {
		t.Fatal(err)
	}
	restarted, err := NewServiceWithStoreAndAudit(s.Router, s.Store, NewMemoryTransactionAuditStore())
	if err != nil {
		t.Fatalf("payment state must survive service reconstruction: %v", err)
	}
	if _, err := restarted.SubmitPayment(context.Background(), "midtrans", req); !errors.Is(err, ErrPaymentSubmissionClaimed) {
		t.Fatalf("expected persisted claim to remain authoritative after restart, got %v", err)
	}
	if p.calls != 1 {
		t.Fatalf("restart must not resubmit payment, calls=%d", p.calls)
	}
}


func TestReconcilePaymentUsesDurableReferenceWithoutResubmission(t *testing.T) {
	p := &paymentSubmissionProvider{
		result: payment.PaymentResult{
			ReferenceID: "pay-reconcile-1",
			ProviderReference: "snap-token",
			Status: payment.StatusPending,
			Amount: 75000,
			Currency: "IDR",
		},
	}
	s := newPaymentSubmissionService(t, p, true)
	req := payment.PaymentRequest{ReferenceID:"pay-reconcile-1", Amount:75000, Currency:"IDR", CustomerID:"cust-reconcile"}
	if _, err := s.SubmitPayment(context.Background(), "midtrans", req); err != nil { t.Fatal(err) }

	p.statusResult = payment.StatusResult{
		ReferenceID:"pay-reconcile-1", ProviderReference:"midtrans-tx-1",
		Status:payment.StatusSuccess, Amount:75000, Currency:"IDR", Message:"settlement",
	}
	got, err := s.ReconcilePayment(context.Background(), req.ReferenceID)
	if err != nil { t.Fatal(err) }
	if got.ProviderReference != "midtrans-tx-1" || got.Status != payment.StatusSuccess { t.Fatalf("unexpected reconciliation: %#v", got) }
	if p.statusCalls != 1 || p.lastStatusRequest != (payment.StatusRequest{ReferenceID:req.ReferenceID}) {
		t.Fatalf("reconciliation must use durable reference only: calls=%d request=%#v", p.statusCalls, p.lastStatusRequest)
	}
	if p.calls != 1 { t.Fatalf("reconciliation must never resubmit payment, create calls=%d", p.calls) }
	state, ok := s.Store.Get(req.ReferenceID)
	if !ok || state.Payment == nil || state.Payment.Status != payment.StatusSuccess || state.Payment.ProviderReference != "midtrans-tx-1" {
		t.Fatalf("expected terminal reconciled state: %#v", state)
	}
}

func TestReconcilePaymentRejectsProviderIdentityMismatch(t *testing.T) {
	p := &paymentSubmissionProvider{
		result: payment.PaymentResult{ReferenceID:"pay-reconcile-2", Amount:50000, Currency:"IDR", Status:payment.StatusPending},
		statusResult: payment.StatusResult{ReferenceID:"other-reference", ProviderReference:"tx-2", Status:payment.StatusSuccess, Amount:50000, Currency:"IDR"},
	}
	s := newPaymentSubmissionService(t,p,true)
	req:=payment.PaymentRequest{ReferenceID:"pay-reconcile-2",Amount:50000,Currency:"IDR",CustomerID:"cust-2"}
	if _,err:=s.SubmitPayment(context.Background(),"midtrans",req);err!=nil{t.Fatal(err)}
	if _,err:=s.ReconcilePayment(context.Background(),req.ReferenceID);!errors.Is(err,ErrPaymentReconciliationConflict){t.Fatalf("expected reconciliation conflict, got %v",err)}
	state,_:=s.Store.Get(req.ReferenceID)
	if state.Payment == nil || state.Payment.Status != payment.StatusPending {t.Fatalf("reconciliation conflict must preserve pending state: %#v",state)}
}

func TestReconcilePaymentDoesNotRewriteTerminalState(t *testing.T) {
	p:=&paymentSubmissionProvider{result:payment.PaymentResult{ReferenceID:"pay-reconcile-3",Amount:25000,Currency:"IDR",Status:payment.StatusPending},statusResult:payment.StatusResult{ReferenceID:"pay-reconcile-3",ProviderReference:"tx-3",Status:payment.StatusSuccess,Amount:25000,Currency:"IDR"}}
	s:=newPaymentSubmissionService(t,p,true)
	req:=payment.PaymentRequest{ReferenceID:"pay-reconcile-3",Amount:25000,Currency:"IDR",CustomerID:"cust-3"}
	if _,err:=s.SubmitPayment(context.Background(),"midtrans",req);err!=nil{t.Fatal(err)}
	if _,err:=s.ReconcilePayment(context.Background(),req.ReferenceID);err!=nil{t.Fatal(err)}
	p.statusResult.Status=payment.StatusFailed
	if _,err:=s.ReconcilePayment(context.Background(),req.ReferenceID);!errors.Is(err,ErrPaymentReconciliationConflict){t.Fatalf("expected terminal conflict, got %v",err)}
}
