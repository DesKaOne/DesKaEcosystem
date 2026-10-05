package routing

import (
 "context"
 "net/http"
 "net/http/httptest"
 "errors"
 "strings"
 "testing"
 payment "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/internal/Payment"
)

func newWebhookHTTPTestService(t *testing.T, p *paymentSubmissionProvider, enabled bool) *Service { t.Helper(); return newPaymentSubmissionService(t,p,enabled) }
func submitWebhookHTTPTestPayment(t *testing.T, s *Service, p *paymentSubmissionProvider, ref string, amount int64) {
 t.Helper(); p.result=payment.PaymentResult{ReferenceID:ref,Status:payment.StatusPending,Amount:amount,Currency:"IDR"}
 _,err:=s.SubmitPayment(context.Background(),"midtrans",payment.PaymentRequest{ReferenceID:ref,Amount:amount,Currency:"IDR",CustomerID:"customer-http"}); if err!=nil { t.Fatal(err) }
}
func TestPaymentWebhookHTTPHandlerSuccess(t *testing.T) {
 p:=&paymentSubmissionProvider{webhookResult:payment.StatusResult{ReferenceID:"http-webhook-1",ProviderReference:"tx-http-1",Status:payment.StatusSuccess,Amount:10000,Currency:"IDR"}}
 s:=newWebhookHTTPTestService(t,p,true); submitWebhookHTTPTestPayment(t,s,p,"http-webhook-1",10000)
 h,err:=NewPaymentWebhookHTTPHandler(s); if err!=nil { t.Fatal(err) }
 req:=httptest.NewRequest(http.MethodPost,"/webhooks/payment/midtrans",strings.NewReader("{}")); rec:=httptest.NewRecorder(); h.ServeHTTP(rec,req)
 if rec.Code!=http.StatusOK { t.Fatalf("expected 200, got %d body=%s",rec.Code,rec.Body.String()) }; if !strings.Contains(rec.Body.String(),"http-webhook-1") { t.Fatalf("unexpected response: %s",rec.Body.String()) }; if p.calls!=1 { t.Fatalf("webhook transport must never resubmit: %d",p.calls) }
}
func TestPaymentWebhookHTTPHandlerMethodAndPath(t *testing.T) {
 s:=newWebhookHTTPTestService(t,&paymentSubmissionProvider{},true); h,_:=NewPaymentWebhookHTTPHandler(s)
 cases:=[]struct{name,method,path string; want int}{{"method",http.MethodGet,"/webhooks/payment/midtrans",405},{"path",http.MethodPost,"/wrong/midtrans",400},{"provider",http.MethodPost,"/webhooks/payment/",400},{"nested",http.MethodPost,"/webhooks/payment/midtrans/extra",400}}
 for _,tc:=range cases { t.Run(tc.name,func(t *testing.T){req:=httptest.NewRequest(tc.method,tc.path,strings.NewReader("{}"));rec:=httptest.NewRecorder();h.ServeHTTP(rec,req);if rec.Code!=tc.want{t.Fatalf("expected %d, got %d",tc.want,rec.Code)}})}
}
func TestPaymentWebhookHTTPHandlerBoundsBody(t *testing.T) {
 p:=&paymentSubmissionProvider{}; s:=newWebhookHTTPTestService(t,p,true); h,_:=NewPaymentWebhookHTTPHandler(s); h.MaxBodyBytes=4
 req:=httptest.NewRequest(http.MethodPost,"/webhooks/payment/midtrans",strings.NewReader("12345")); rec:=httptest.NewRecorder(); h.ServeHTTP(rec,req)
 if rec.Code!=http.StatusRequestEntityTooLarge { t.Fatalf("expected 413, got %d",rec.Code) }; if p.webhookCalls!=0 { t.Fatal("oversized body must not reach provider adapter") }
}
func TestPaymentWebhookHTTPHandlerMapsDomainErrorsWithoutProviderDetails(t *testing.T) {
 p:=&paymentSubmissionProvider{webhookErr:errors.New("midtrans secret signature mismatch")}; s:=newWebhookHTTPTestService(t,p,true); h,_:=NewPaymentWebhookHTTPHandler(s)
 req:=httptest.NewRequest(http.MethodPost,"/webhooks/payment/midtrans",strings.NewReader("{}")); rec:=httptest.NewRecorder(); h.ServeHTTP(rec,req)
 if rec.Code!=http.StatusBadRequest { t.Fatalf("expected 400, got %d",rec.Code) }; if strings.Contains(rec.Body.String(),"midtrans secret") { t.Fatalf("provider-specific error leaked: %s",rec.Body.String()) }
}
func TestPaymentWebhookHTTPHandlerDoesNotTrustSecuritySensitiveHeadersForRouting(t *testing.T) {
	p := &paymentSubmissionProvider{
		webhookResult: payment.StatusResult{
			ReferenceID:       "http-header-1",
			ProviderReference: "tx-header-1",
			Status:            payment.StatusPending,
			Amount:            10000,
			Currency:          "IDR",
		},
	}
	s := newWebhookHTTPTestService(t, p, true)
	submitWebhookHTTPTestPayment(t, s, p, "http-header-1", 10000)
	h, err := NewPaymentWebhookHTTPHandler(s)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/webhooks/payment/midtrans", strings.NewReader("{}"))
	req.Host = "attacker.example"
	req.Header.Set("X-Forwarded-Host", "other-provider")
	req.Header.Set("X-Forwarded-For", "203.0.113.10")
	req.Header.Set("Content-Type", "text/plain; charset=invalid")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("security-sensitive headers must not alter provider routing: status=%d body=%s", rec.Code, rec.Body.String())
	}
	if p.webhookCalls != 1 {
		t.Fatalf("expected exactly one adapter invocation, got %d", p.webhookCalls)
	}
}

func TestPaymentWebhookHTTPHandlerDisabledCapability(t *testing.T) {
 p:=&paymentSubmissionProvider{}; s:=newWebhookHTTPTestService(t,p,false); h,_:=NewPaymentWebhookHTTPHandler(s)
 req:=httptest.NewRequest(http.MethodPost,"/webhooks/payment/midtrans",strings.NewReader("{}")); rec:=httptest.NewRecorder(); h.ServeHTTP(rec,req)
 if rec.Code!=http.StatusServiceUnavailable { t.Fatalf("expected 503, got %d",rec.Code) }; if p.webhookCalls!=0 { t.Fatal("disabled capability must not reach adapter") }
}
