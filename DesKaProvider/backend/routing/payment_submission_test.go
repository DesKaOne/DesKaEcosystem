package routing

import (
 "context"
 "errors"
 "testing"
 provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
 "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
 payment "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/internal/Payment"
)

type paymentSubmissionProvider struct {
 calls int
 statusCalls int
 webhookCalls int
 lastStatusRequest payment.StatusRequest
 lastWebhookPayload []byte
 result payment.PaymentResult
 statusResult payment.StatusResult
 webhookResult payment.StatusResult
 err error
 webhookErr error
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
func (p *paymentSubmissionProvider) HandlePaymentWebhook(_ context.Context, payload []byte) (payment.StatusResult, error) {
 p.webhookCalls++
 p.lastWebhookPayload=append([]byte(nil), payload...)
 if p.webhookErr != nil { return payment.StatusResult{}, p.webhookErr }
 return p.webhookResult, nil
}

func newPaymentSubmissionService(t *testing.T, p *paymentSubmissionProvider, enabled bool) *Service {
 t.Helper()
 reg:=provider.NewRegistry()
 if err:=reg.RegisterCapabilityProvider("midtrans",provider.CapabilityPayment,p,provider.CapabilityStatus{AdapterImplemented:true,Tested:true,Enabled:enabled});err!=nil{t.Fatal(err)}
 return &Service{Router:&Router{Registry:reg},Store:NewMemoryTransactionStore(),AuditStore:NewMemoryTransactionAuditStore()}
}

type ambiguousPaymentPersistenceStore struct {
	*MemoryTransactionStore
	failTerminalPut bool
}

func (s *ambiguousPaymentPersistenceStore) Put(state TransactionState) error {
	if s.failTerminalPut && state.Payment != nil && state.Payment.Status != payment.StatusPending {
		return ErrTransactionPersistenceAmbiguous
	}
	return s.MemoryTransactionStore.Put(state)
}

func (s *ambiguousPaymentPersistenceStore) PutIfCurrentContext(ctx context.Context, referenceID string, previous, next TransactionState) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.failTerminalPut && next.Payment != nil && next.Payment.Status != payment.StatusPending {
		return ErrTransactionPersistenceAmbiguous
	}
	return s.MemoryTransactionStore.PutIfCurrentContext(ctx, referenceID, previous, next)
}

func TestSubmitPaymentAmbiguousPersistencePreservesClaimAndForbidsRetry(t *testing.T) {
	p := &paymentSubmissionProvider{
		result: payment.PaymentResult{
			ReferenceID: "pay-ambiguous-persist",
			ProviderReference: "mid-ambiguous",
			Status: payment.StatusSuccess,
			Amount: 10000,
			Currency: "IDR",
		},
	}
	store := &ambiguousPaymentPersistenceStore{MemoryTransactionStore: NewMemoryTransactionStore()}
	reg := provider.NewRegistry()
	if err := reg.RegisterCapabilityProvider("midtrans", provider.CapabilityPayment, p, provider.CapabilityStatus{AdapterImplemented: true, Tested: true, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	s := &Service{
		Router: &Router{Registry: reg},
		Store: store,
		AuditStore: NewMemoryTransactionAuditStore(),
	}
	req := payment.PaymentRequest{ReferenceID: "pay-ambiguous-persist", Amount: 10000, Currency: "IDR", CustomerID: "cust-ambiguous"}
	store.failTerminalPut = true

	result, err := s.SubmitPayment(context.Background(), "midtrans", req)
	if !errors.Is(err, ErrTransactionPersistenceAmbiguous) {
		t.Fatalf("expected ambiguous persistence to survive service wrapping, got result=%#v err=%v", result, err)
	}
	if p.calls != 1 {
		t.Fatalf("ambiguous persistence must not trigger an immediate resubmission, calls=%d", p.calls)
	}
	state, ok := store.Get(req.ReferenceID)
	if !ok || state.Payment == nil || state.Payment.Status != payment.StatusPending {
		t.Fatalf("durable claim must remain pending after ambiguous terminal persistence: %#v", state)
	}

	if _, err := s.SubmitPayment(context.Background(), "midtrans", req); !errors.Is(err, ErrPaymentSubmissionClaimed) {
		t.Fatalf("same-process retry must remain blocked by durable claim, got %v", err)
	}
	if p.calls != 1 {
		t.Fatalf("same-process retry must not resubmit payment, calls=%d", p.calls)
	}

	restarted, err := NewServiceWithStoreAndAudit(s.Router, store, NewMemoryTransactionAuditStore())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.SubmitPayment(context.Background(), "midtrans", req); !errors.Is(err, ErrPaymentSubmissionClaimed) {
		t.Fatalf("restart must preserve pending claim and block resubmission, got %v", err)
	}
	if p.calls != 1 {
		t.Fatalf("restart must not resubmit ambiguous payment, calls=%d", p.calls)
	}
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

type disableAfterPaymentClaimStore struct {
	*MemoryTransactionStore
	stateStore *operational.ProviderStateStore
	providerName string
}

func (s *disableAfterPaymentClaimStore) CreateIfAbsentContext(ctx context.Context, state TransactionState) (TransactionState, bool, error) {
	existing, created, err := s.MemoryTransactionStore.CreateIfAbsentContext(ctx, state)
	if err == nil && created {
		providerState, found := s.stateStore.Get(s.providerName)
		if found {
			providerState.Lifecycle = operational.LifecycleDisabled
			if err := s.stateStore.Put(providerState); err != nil {
				return TransactionState{}, false, err
			}
		}
	}
	return existing, created, err
}

func TestSubmitPaymentRechecksLifecycleAfterDurableClaim(t *testing.T) {
	p := &paymentSubmissionProvider{result: payment.PaymentResult{
		ReferenceID: "pay-lifecycle-race",
		ProviderReference: "mid-lifecycle-race",
		Status: payment.StatusPending,
		Amount: 41000,
		Currency: "IDR",
	}}
	registry := provider.NewRegistry()
	if err := registry.RegisterCapabilityProvider("midtrans", provider.CapabilityPayment, p, provider.CapabilityStatus{
		AdapterImplemented: true, Tested: true, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	stateStore := operational.NewProviderStateStore()
	state, err := operational.NewProviderState("midtrans")
	if err != nil { t.Fatal(err) }
	state.Lifecycle = operational.LifecycleEnabled
	state.Capabilities = []operational.Capability{operational.CapabilityPayment}
	state.EnabledCapabilities = []operational.Capability{operational.CapabilityPayment}
	if err := stateStore.Put(state); err != nil { t.Fatal(err) }

	store := &disableAfterPaymentClaimStore{
		MemoryTransactionStore: NewMemoryTransactionStore(),
		stateStore: stateStore,
		providerName: "midtrans",
	}
	router := &Router{Registry: registry, ProviderState: stateStore}
	s := &Service{Router: router, Store: store, AuditStore: NewMemoryTransactionAuditStore()}
	req := payment.PaymentRequest{ReferenceID: "pay-lifecycle-race", Amount: 41000, Currency: "IDR", CustomerID: "cust-lifecycle-race"}

	if _, err := s.SubmitPayment(context.Background(), "midtrans", req); !errors.Is(err, ErrPaymentCapabilityDisabled) {
		t.Fatalf("expected lifecycle disable after claim to block provider call, got %v", err)
	}
	if p.calls != 0 {
		t.Fatalf("lifecycle disable after claim must prevent external payment creation, calls=%d", p.calls)
	}
	stateAfter, ok := store.Get(req.ReferenceID)
	if !ok || stateAfter.Payment == nil || stateAfter.Payment.Status != payment.StatusPending {
		t.Fatalf("durable claim must remain pending after lifecycle gate closes: %#v", stateAfter)
	}
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


func TestReconcilePaymentAmbiguousPersistencePreservesPendingAndForbidsRetry(t *testing.T) {
	p := &paymentSubmissionProvider{
		result: payment.PaymentResult{
			ReferenceID:      "pay-reconcile-ambiguous",
			ProviderReference: "mid-reconcile-ambiguous",
			Status:           payment.StatusPending,
			Amount:           75000,
			Currency:         "IDR",
		},
		statusResult: payment.StatusResult{
			ReferenceID:      "pay-reconcile-ambiguous",
			ProviderReference: "mid-reconcile-ambiguous",
			Status:            payment.StatusSuccess,
			Amount:            75000,
			Currency:          "IDR",
			Message:           "provider reports success",
		},
	}
	store := &ambiguousPaymentPersistenceStore{MemoryTransactionStore: NewMemoryTransactionStore()}
	reg := provider.NewRegistry()
	if err := reg.RegisterCapabilityProvider("midtrans", provider.CapabilityPayment, p, provider.CapabilityStatus{AdapterImplemented: true, Tested: true, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	s := &Service{
		Router: &Router{Registry: reg},
		Store: store,
		AuditStore: NewMemoryTransactionAuditStore(),
	}
	req := payment.PaymentRequest{ReferenceID: "pay-reconcile-ambiguous", Amount: 75000, Currency: "IDR", CustomerID: "cust-reconcile-ambiguous"}
	if _, err := s.SubmitPayment(context.Background(), "midtrans", req); err != nil {
		t.Fatal(err)
	}
	store.failTerminalPut = true

	result, err := s.ReconcilePayment(context.Background(), req.ReferenceID)
	if !errors.Is(err, ErrTransactionPersistenceAmbiguous) {
		t.Fatalf("expected ambiguous persistence to survive reconciliation wrapping, got result=%#v err=%v", result, err)
	}
	if p.calls != 1 {
		t.Fatalf("reconciliation must never resubmit payment, create calls=%d", p.calls)
	}
	state, ok := store.Get(req.ReferenceID)
	if !ok {
		t.Fatal("expected durable payment claim to remain")
	}
	if state.Payment == nil || state.Payment.Status != payment.StatusPending {
		t.Fatalf("ambiguous reconciliation must preserve durable pending payment state: %#v", state.Payment)
	}
	if _, err := s.ReconcilePayment(context.Background(), req.ReferenceID); !errors.Is(err, ErrTransactionPersistenceAmbiguous) {
		t.Fatalf("repeated reconciliation must remain blocked by ambiguity, got %v", err)
	}
	if p.calls != 1 {
		t.Fatalf("repeated reconciliation must never resubmit payment, create calls=%d", p.calls)
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

func TestHandlePaymentWebhookTransitionsDurablePaymentWithoutResubmission(t *testing.T) {
	p:=&paymentSubmissionProvider{
		result:payment.PaymentResult{ReferenceID:"pay-webhook-1",ProviderReference:"snap-token",Status:payment.StatusPending,Amount:60000,Currency:"IDR"},
		webhookResult:payment.StatusResult{ReferenceID:"pay-webhook-1",ProviderReference:"midtrans-tx-webhook-1",Status:payment.StatusSuccess,Amount:60000,Currency:"IDR",Message:"settlement"},
	}
	s:=newPaymentSubmissionService(t,p,true)
	req:=payment.PaymentRequest{ReferenceID:"pay-webhook-1",Amount:60000,Currency:"IDR",CustomerID:"cust-webhook"}
	if _,err:=s.SubmitPayment(context.Background(),"midtrans",req);err!=nil{t.Fatal(err)}
	got,err:=s.HandlePaymentWebhook(context.Background(),"midtrans",[]byte("{}"))
	if err!=nil{t.Fatal(err)}
	if got.Status!=payment.StatusSuccess||got.ProviderReference!="midtrans-tx-webhook-1"{t.Fatalf("unexpected webhook result: %#v",got)}
	if p.webhookCalls!=1||p.calls!=1{t.Fatalf("webhook must normalize once and never resubmit: webhook=%d create=%d",p.webhookCalls,p.calls)}
	state,ok:=s.Store.Get(req.ReferenceID)
	if !ok||state.Payment==nil||state.Payment.Status!=payment.StatusSuccess||state.Payment.ProviderReference!="midtrans-tx-webhook-1"{t.Fatalf("expected durable webhook transition: %#v",state)}
}

func TestHandlePaymentWebhookAmbiguousPersistencePreservesPendingAndForbidsRetry(t *testing.T) {
	p := &paymentSubmissionProvider{
		result: payment.PaymentResult{
			ReferenceID:       "pay-webhook-ambiguous",
			ProviderReference: "mid-webhook-ambiguous",
			Status:            payment.StatusPending,
			Amount:            60000,
			Currency:          "IDR",
		},
		webhookResult: payment.StatusResult{
			ReferenceID:       "pay-webhook-ambiguous",
			ProviderReference: "midtrans-tx-webhook-ambiguous",
			Status:            payment.StatusSuccess,
			Amount:            60000,
			Currency:          "IDR",
			Message:           "settlement",
		},
	}
	store := &ambiguousPaymentPersistenceStore{MemoryTransactionStore: NewMemoryTransactionStore()}
	reg := provider.NewRegistry()
	if err := reg.RegisterCapabilityProvider("midtrans", provider.CapabilityPayment, p, provider.CapabilityStatus{AdapterImplemented: true, Tested: true, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	s := &Service{
		Router: &Router{Registry: reg},
		Store: store,
		AuditStore: NewMemoryTransactionAuditStore(),
	}
	req := payment.PaymentRequest{ReferenceID: "pay-webhook-ambiguous", Amount: 60000, Currency: "IDR", CustomerID: "cust-webhook-ambiguous"}
	if _, err := s.SubmitPayment(context.Background(), "midtrans", req); err != nil {
		t.Fatal(err)
	}
	store.failTerminalPut = true

	result, err := s.HandlePaymentWebhook(context.Background(), "midtrans", []byte("{}"))
	if !errors.Is(err, ErrTransactionPersistenceAmbiguous) {
		t.Fatalf("expected ambiguous persistence from payment webhook, got result=%#v err=%v", result, err)
	}
	if p.calls != 1 || p.webhookCalls != 1 {
		t.Fatalf("webhook must not resubmit payment: create=%d webhook=%d", p.calls, p.webhookCalls)
	}
	state, ok := store.Get(req.ReferenceID)
	if !ok {
		t.Fatal("expected durable payment claim to remain")
	}
	if state.Payment == nil || state.Payment.Status != payment.StatusPending {
		t.Fatalf("ambiguous webhook persistence must preserve durable pending payment: %#v", state.Payment)
	}
	if _, err := s.HandlePaymentWebhook(context.Background(), "midtrans", []byte("{}")); !errors.Is(err, ErrTransactionPersistenceAmbiguous) {
		t.Fatalf("repeated webhook must remain blocked by ambiguity, got %v", err)
	}
	if p.calls != 1 || p.webhookCalls != 2 {
		t.Fatalf("repeated webhook must never resubmit payment: create=%d webhook=%d", p.calls, p.webhookCalls)
	}
}

func TestHandlePaymentWebhookRejectsProviderOwnershipMismatch(t *testing.T) {
	p:=&paymentSubmissionProvider{webhookResult:payment.StatusResult{ReferenceID:"pay-webhook-2",ProviderReference:"tx-2",Status:payment.StatusSuccess,Amount:70000,Currency:"IDR"}}
	s:=newPaymentSubmissionService(t,p,true)
	p.result=payment.PaymentResult{ReferenceID:"pay-webhook-2",Status:payment.StatusPending,Amount:70000,Currency:"IDR"}
	req:=payment.PaymentRequest{ReferenceID:"pay-webhook-2",Amount:70000,Currency:"IDR",CustomerID:"cust-webhook-2"}
	if _,err:=s.SubmitPayment(context.Background(),"midtrans",req);err!=nil{t.Fatal(err)}
	if _,err:=s.HandlePaymentWebhook(context.Background(),"other-provider",[]byte("{}"));err==nil{t.Fatal("expected provider mismatch")}
	state,_:=s.Store.Get(req.ReferenceID)
	if state.Payment==nil||state.Payment.Status!=payment.StatusPending{t.Fatalf("provider mismatch must not mutate state: %#v",state)}
	if p.webhookCalls!=0{t.Fatal("provider adapter must not receive payload for a different provider")}
}

func TestHandlePaymentWebhookDuplicateTerminalIsIdempotent(t *testing.T) {
	p:=&paymentSubmissionProvider{
		result:payment.PaymentResult{ReferenceID:"pay-webhook-3",Status:payment.StatusPending,Amount:80000,Currency:"IDR"},
		webhookResult:payment.StatusResult{ReferenceID:"pay-webhook-3",ProviderReference:"tx-3",Status:payment.StatusSuccess,Amount:80000,Currency:"IDR",Message:"settlement"},
	}
	s:=newPaymentSubmissionService(t,p,true)
	req:=payment.PaymentRequest{ReferenceID:"pay-webhook-3",Amount:80000,Currency:"IDR",CustomerID:"cust-webhook-3"}
	if _,err:=s.SubmitPayment(context.Background(),"midtrans",req);err!=nil{t.Fatal(err)}
	if _,err:=s.HandlePaymentWebhook(context.Background(),"midtrans",[]byte("{}"));err!=nil{t.Fatal(err)}
	if _,err:=s.HandlePaymentWebhook(context.Background(),"midtrans",[]byte("{}"));err!=nil{t.Fatal(err)}
	if p.webhookCalls!=2||p.calls!=1{t.Fatalf("duplicate webhook must be idempotent without resubmission: webhook=%d create=%d",p.webhookCalls,p.calls)}
}

func TestHandlePaymentWebhookDuplicatePendingIsIdempotent(t *testing.T) {
	p := &paymentSubmissionProvider{
		result: payment.PaymentResult{
			ReferenceID: "pay-webhook-pending-duplicate",
			Status:      payment.StatusPending,
			Amount:      91000,
			Currency:    "IDR",
		},
		webhookResult: payment.StatusResult{
			ReferenceID:       "pay-webhook-pending-duplicate",
			ProviderReference: "tx-pending-duplicate",
			Status:            payment.StatusPending,
			Amount:            91000,
			Currency:          "IDR",
			Message:           "pending",
		},
	}
	s := newPaymentSubmissionService(t, p, true)
	req := payment.PaymentRequest{
		ReferenceID: "pay-webhook-pending-duplicate",
		Amount:      91000,
		Currency:    "IDR",
		CustomerID:  "cust-webhook-pending-duplicate",
	}
	if _, err := s.SubmitPayment(context.Background(), "midtrans", req); err != nil {
		t.Fatal(err)
	}
	if _, err := s.HandlePaymentWebhook(context.Background(), "midtrans", []byte("{}")); err != nil {
		t.Fatal(err)
	}
	before, ok := s.Store.Get(req.ReferenceID)
	if !ok {
		t.Fatal("expected durable payment state")
	}
	if _, err := s.HandlePaymentWebhook(context.Background(), "midtrans", []byte("{}")); err != nil {
		t.Fatal(err)
	}
	after, ok := s.Store.Get(req.ReferenceID)
	if !ok {
		t.Fatal("expected durable payment state after duplicate webhook")
	}
	if after.Version != before.Version {
		t.Fatalf("identical pending webhook must not rewrite durable version: before=%d after=%d", before.Version, after.Version)
	}
	if after.Payment == nil || after.Payment.Status != payment.StatusPending || after.Payment.ProviderReference != "tx-pending-duplicate" {
		t.Fatalf("duplicate pending webhook must preserve observation: %#v", after)
	}
	if p.calls != 1 || p.webhookCalls != 2 {
		t.Fatalf("duplicate pending webhook must never resubmit payment: create=%d webhook=%d", p.calls, p.webhookCalls)
	}
}

func TestHandlePaymentWebhookConflictingTerminalIsRejected(t *testing.T) {
	p:=&paymentSubmissionProvider{
		result:payment.PaymentResult{ReferenceID:"pay-webhook-4",Status:payment.StatusPending,Amount:90000,Currency:"IDR"},
		webhookResult:payment.StatusResult{ReferenceID:"pay-webhook-4",ProviderReference:"tx-4",Status:payment.StatusSuccess,Amount:90000,Currency:"IDR",Message:"settlement"},
	}
	s:=newPaymentSubmissionService(t,p,true)
	req:=payment.PaymentRequest{ReferenceID:"pay-webhook-4",Amount:90000,Currency:"IDR",CustomerID:"cust-webhook-4"}
	if _,err:=s.SubmitPayment(context.Background(),"midtrans",req);err!=nil{t.Fatal(err)}
	if _,err:=s.HandlePaymentWebhook(context.Background(),"midtrans",[]byte("{}"));err!=nil{t.Fatal(err)}
	p.webhookResult.Status=payment.StatusFailed
	if _,err:=s.HandlePaymentWebhook(context.Background(),"midtrans",[]byte("{}"));!errors.Is(err,ErrWebhookReferenceConflict){t.Fatalf("expected terminal conflict, got %v",err)}
	state,_:=s.Store.Get(req.ReferenceID)
	if state.Payment==nil||state.Payment.Status!=payment.StatusSuccess{t.Fatalf("conflicting terminal webhook must not rewrite state: %#v",state)}
	if p.calls!=1{t.Fatal("webhook must never resubmit payment")}
}

func TestHandlePaymentWebhookRejectsUnknownReference(t *testing.T) {
	p:=&paymentSubmissionProvider{webhookResult:payment.StatusResult{ReferenceID:"missing",ProviderReference:"tx-missing",Status:payment.StatusSuccess,Amount:10000,Currency:"IDR"}}
	s:=newPaymentSubmissionService(t,p,true)
	if _,err:=s.HandlePaymentWebhook(context.Background(),"midtrans",[]byte("{}"));!errors.Is(err,ErrWebhookTransactionNotFound){t.Fatalf("expected not found, got %v",err)}
}

func TestHandlePaymentWebhookDoesNotAuthorizeWhenPaymentCapabilityDisabled(t *testing.T) {
	p:=&paymentSubmissionProvider{webhookResult:payment.StatusResult{ReferenceID:"pay-webhook-disabled",ProviderReference:"tx",Status:payment.StatusSuccess,Amount:10000,Currency:"IDR"}}
	s:=newPaymentSubmissionService(t,p,false)
	if _,err:=s.HandlePaymentWebhook(context.Background(),"midtrans",[]byte("{}"));!errors.Is(err,ErrPaymentCapabilityDisabled){t.Fatalf("expected disabled capability, got %v",err)}
	if p.webhookCalls!=0{t.Fatal("disabled capability must not invoke webhook adapter")}
}

