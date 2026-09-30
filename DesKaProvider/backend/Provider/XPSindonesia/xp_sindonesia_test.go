package xpsindonesia

import (
 "context"
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "testing"
 "time"

 "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/config"
 provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

func TestXPPurchaseAndBalance(t *testing.T) {
 srv:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  if err:=r.ParseForm();err!=nil{t.Fatal(err)}
  if r.URL.Path=="/api/order.php" {
   if r.Form.Get("id")!="123"||r.Form.Get("key")!="key"||r.Form.Get("api")!="api"||r.Form.Get("url")!="https://callback.example/xp"||r.Form.Get("trx")!="ref-1"||r.Form.Get("kod")!="i5"||r.Form.Get("isi")!="0856"{t.Fatalf("unexpected order form: %#v",r.Form)}
   _=json.NewEncoder(w).Encode(map[string]string{"success":"1","status":"proses","trx":"ref-1","kode":"i5","isi":"0856","harga":"5700"});return
  }
  if r.URL.Path=="/api/saldo.php" {_=json.NewEncoder(w).Encode(map[string]string{"success":"1","id":"123","saldo":"123000"});return}
  t.Fatalf("unexpected path %s",r.URL.Path)
 }));defer srv.Close()
 c,err:=New(config.XPSindonesiaConfig{ID:"123",Key:"key",API:"api",SaldoEndpoint:srv.URL+"/api/saldo.php",OrderEndpoint:srv.URL+"/api/order.php",CallbackURL:"https://callback.example/xp"},srv.Client());if err!=nil{t.Fatal(err)}
 p,err:=c.Purchase(context.Background(),provider.PurchaseRequest{ProductCode:"i5",CustomerNo:"0856",ReferenceID:"ref-1"});if err!=nil{t.Fatal(err)}
 if p.Status!=provider.StatusPending||p.Price!=5700{t.Fatalf("unexpected purchase: %#v",p)}
 b,err:=c.GetBalance(context.Background());if err!=nil||b!=123000{t.Fatalf("balance=%d err=%v",b,err)}
}

func TestXPBalanceRequestUsesDocumentedFormFields(t *testing.T) {
 srv:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  if err:=r.ParseForm();err!=nil{t.Fatal(err)}
  if r.Form.Get("id")!="123"||r.Form.Get("key")!="key"||r.Form.Get("api")!="api" {
   t.Fatalf("unexpected balance form: %#v",r.Form)
  }
  _=json.NewEncoder(w).Encode(map[string]string{"success":"1","id":"123","saldo":"123000"})
 }));defer srv.Close()
 c,err:=New(config.XPSindonesiaConfig{ID:"123",Key:"key",API:"api",SaldoEndpoint:srv.URL},srv.Client());if err!=nil{t.Fatal(err)}
 b,err:=c.GetBalance(context.Background());if err!=nil||b!=123000{t.Fatalf("balance=%d err=%v",b,err)}
}

func TestXPBalanceRejectsInvalidSaldo(t *testing.T) {
 srv:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  _=json.NewEncoder(w).Encode(map[string]string{"success":"1","id":"123","saldo":"not-a-number"})
 }));defer srv.Close()
 c,err:=New(config.XPSindonesiaConfig{ID:"123",Key:"key",API:"api",SaldoEndpoint:srv.URL},srv.Client());if err!=nil{t.Fatal(err)}
 if _,err:=c.GetBalance(context.Background());err==nil{t.Fatal("expected invalid saldo error")}
}

func TestXPBalanceRejectsInvalidSuccessDiscriminator(t *testing.T) {
 srv:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  _=json.NewEncoder(w).Encode(map[string]string{"success":"2","id":"123","saldo":"123000"})
 }));defer srv.Close()
 c,err:=New(config.XPSindonesiaConfig{ID:"123",Key:"key",API:"api",SaldoEndpoint:srv.URL},srv.Client());if err!=nil{t.Fatal(err)}
 if _,err:=c.GetBalance(context.Background());err==nil{t.Fatal("expected invalid success discriminator error")}
}

func TestXPBalanceRejectsMemberIDMismatch(t *testing.T) {
 srv:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  _=json.NewEncoder(w).Encode(map[string]string{"success":"1","id":"999","saldo":"123000"})
 }));defer srv.Close()
 c,err:=New(config.XPSindonesiaConfig{ID:"123",Key:"key",API:"api",SaldoEndpoint:srv.URL},srv.Client());if err!=nil{t.Fatal(err)}
 if _,err:=c.GetBalance(context.Background());err==nil{t.Fatal("expected balance member ID mismatch")}
}

func TestXPBalanceRejectsProviderError(t *testing.T) {
 srv:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  _=json.NewEncoder(w).Encode(map[string]string{"success":"0","error":"invalid api"})
 }));defer srv.Close()
 c,err:=New(config.XPSindonesiaConfig{ID:"123",Key:"key",API:"api",SaldoEndpoint:srv.URL+"/api/saldo.php"},srv.Client());if err!=nil{t.Fatal(err)}
 if _,err:=c.GetBalance(context.Background());err==nil{t.Fatal("expected provider error")}
}

func TestXPBalanceRejectsMissingSaldo(t *testing.T) {
 srv:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  _=json.NewEncoder(w).Encode(map[string]string{"success":"1","id":"123"})
 }));defer srv.Close()
 c,err:=New(config.XPSindonesiaConfig{ID:"123",Key:"key",API:"api",SaldoEndpoint:srv.URL+"/api/saldo.php"},srv.Client());if err!=nil{t.Fatal(err)}
 if _,err:=c.GetBalance(context.Background());err==nil{t.Fatal("expected missing saldo error")}
}

func TestXPBalanceAcceptsNumericSaldo(t *testing.T) {
 srv:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  _=json.NewEncoder(w).Encode(map[string]any{"success":"1","id":"123","saldo":123000})
 }));defer srv.Close()
 c,err:=New(config.XPSindonesiaConfig{ID:"123",Key:"key",API:"api",SaldoEndpoint:srv.URL+"/api/saldo.php"},srv.Client());if err!=nil{t.Fatal(err)}
 b,err:=c.GetBalance(context.Background());if err!=nil||b!=123000{t.Fatalf("balance=%d err=%v",b,err)}
}

func TestXPCallbackMapping(t *testing.T) {
 c,err:=New(config.XPSindonesiaConfig{ID:"123",Key:"key",API:"api"},nil);if err!=nil{t.Fatal(err)}
 e,err:=c.HandleWebhook(context.Background(),provider.WebhookRequest{Body:[]byte("id=123&key=secret&trx=ref-1&status=sukses&kod=i5&isi=0856&sn=SN1"),SignatureSecret:"secret"});if err!=nil{t.Fatal(err)}
 if e.Status!=provider.StatusSuccess||e.SerialNumber!="SN1"{t.Fatalf("unexpected event: %#v",e)}
 _,err=c.HandleWebhook(context.Background(),provider.WebhookRequest{Body:[]byte("id=123&key=bad&trx=ref-1&status=sukses&kod=i5&isi=0856"),SignatureSecret:"secret"});if err==nil{t.Fatal("expected invalid callback key")}
 _,err=c.HandleWebhook(context.Background(),provider.WebhookRequest{Body:[]byte("id=123&key=secret&trx=ref-1&status=proses&kod=i5&isi=0856"),SignatureSecret:"secret"});if err==nil{t.Fatal("expected unsupported callback status")}
}

func TestXPUnsupportedOperationsAreExplicit(t *testing.T) {
 c,err:=New(config.XPSindonesiaConfig{ID:"123",Key:"key",API:"api"},nil);if err!=nil{t.Fatal(err)}
 if _,err:=c.GetProducts(context.Background(),provider.ProductRequest{});err!=provider.ErrUnsupportedOperation{t.Fatal(err)}
 if _,err:=c.Inquiry(context.Background(),provider.InquiryRequest{});err!=provider.ErrUnsupportedOperation{t.Fatal(err)}
 if _,err:=c.GetStatus(context.Background(),provider.StatusRequest{});err!=provider.ErrUnsupportedOperation{t.Fatal(err)}
}

func TestXPPurchaseRequiresCallbackURL(t *testing.T) {
 c,err:=New(config.XPSindonesiaConfig{ID:"123",Key:"key",API:"api",OrderEndpoint:"https://example.invalid/order.php"},nil);if err!=nil{t.Fatal(err)}
 _,err=c.Purchase(context.Background(),provider.PurchaseRequest{ProductCode:"i5",CustomerNo:"0856",ReferenceID:"ref-1"});if err==nil{t.Fatal("expected callback URL requirement")}
}

func TestXPStatusMappingCoversDocumentedOrderStates(t *testing.T) {
 cases:=map[string]provider.TransactionStatus{"sukses":provider.StatusSuccess,"gagal":provider.StatusFailed,"proses":provider.StatusPending,"lambat":provider.StatusPending}
 for raw,want:=range cases {if got:=mapStatus(raw);got!=want{t.Fatalf("%q => %q, want %q",raw,got,want)}}
 if got:=mapStatus("unknown");got!=""{t.Fatalf("unknown status must fail closed, got %q",got)}
}

func TestXPOrderStatusMappingCoversDocumentedErrorStates(t *testing.T) {
 cases := map[string]provider.TransactionStatus{
  "gagal (batalkan manual di hisoty order)": provider.StatusFailed,
  "kosong (batalkan manual di hisoty order)": provider.StatusFailed,
  "proses": provider.StatusPending,
  "lambat": provider.StatusPending,
  "sukses": provider.StatusSuccess,
 }
 for raw, want := range cases {
  if got := mapOrderStatus(raw); got != want {
   t.Fatalf("%q => %q, want %q", raw, got, want)
  }
 }
 if got := mapOrderStatus("unknown"); got != "" {
  t.Fatalf("unknown order status must fail closed, got %q", got)
 }
}


func TestXPPurchaseRejectsInvalidSuccessDiscriminator(t *testing.T) {
 srv:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  _=json.NewEncoder(w).Encode(map[string]string{"success":"2","status":"proses","trx":"ref-1","kode":"i5","isi":"0856","harga":"5700"})
 }));defer srv.Close()
 c,err:=New(config.XPSindonesiaConfig{ID:"123",Key:"key",API:"api",OrderEndpoint:srv.URL,CallbackURL:"https://callback.example/xp"},srv.Client());if err!=nil{t.Fatal(err)}
 _,err=c.Purchase(context.Background(),provider.PurchaseRequest{ProductCode:"i5",CustomerNo:"0856",ReferenceID:"ref-1"})
 if err==nil{t.Fatal("expected invalid success discriminator rejection")}
}

func TestXPCallbackRejectsMemberIDMismatch(t *testing.T) {
 c,err:=New(config.XPSindonesiaConfig{ID:"123",Key:"key",API:"api"},nil);if err!=nil{t.Fatal(err)}
 _,err=c.HandleWebhook(context.Background(),provider.WebhookRequest{Body:[]byte("id=999&key=secret&trx=ref-1&status=sukses&kod=i5&isi=0856"),SignatureSecret:"secret"})
 if err==nil{t.Fatal("expected callback member ID mismatch")}
}

func TestXPPurchaseMapsDocumentedEmptyFailure(t *testing.T) {
 srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  _ = json.NewEncoder(w).Encode(map[string]string{
   "success": "0",
   "error": "already",
   "status": "kosong (batalkan manual di hisoty order)",
   "trx": "ref-1",
   "kode": "i5",
   "isi": "0856",
   "harga": "5700",
  })
 }))
 defer srv.Close()

 c, err := New(config.XPSindonesiaConfig{
  ID: "123", Key: "key", API: "api",
  OrderEndpoint: srv.URL,
  CallbackURL: "https://callback.example/xp",
 }, srv.Client())
 if err != nil { t.Fatal(err) }

 result, err := c.Purchase(context.Background(), provider.PurchaseRequest{
  ProductCode: "i5", CustomerNo: "0856", ReferenceID: "ref-1",
 })
 if err != nil { t.Fatal(err) }
 if result.Status != provider.StatusFailed {
  t.Fatalf("unexpected purchase status: %#v", result)
 }
 if result.ProviderCode != "already" {
  t.Fatalf("unexpected provider code: %#v", result)
 }
}


func TestXPHTTPClientGetsBoundedTimeout(t *testing.T) {
 c, err := New(config.XPSindonesiaConfig{ID: "123", Key: "key", API: "api"}, nil)
 if err != nil { t.Fatal(err) }
 if c.httpClient.Timeout != defaultHTTPTimeout {
  t.Fatalf("default timeout=%v, want %v", c.httpClient.Timeout, defaultHTTPTimeout)
 }

 zero := &http.Client{}
 c, err = New(config.XPSindonesiaConfig{ID: "123", Key: "key", API: "api"}, zero)
 if err != nil { t.Fatal(err) }
 if c.httpClient.Timeout != defaultHTTPTimeout {
  t.Fatalf("zero-client timeout=%v, want %v", c.httpClient.Timeout, defaultHTTPTimeout)
 }
 if zero.Timeout != 0 {
  t.Fatalf("caller client was mutated: timeout=%v", zero.Timeout)
 }

 custom := &http.Client{Timeout: 2 * time.Second}
 c, err = New(config.XPSindonesiaConfig{ID: "123", Key: "key", API: "api"}, custom)
 if err != nil { t.Fatal(err) }
 if c.httpClient.Timeout != 2*time.Second {
  t.Fatalf("custom timeout=%v, want %v", c.httpClient.Timeout, 2*time.Second)
 }
 if c.httpClient != custom {
  t.Fatal("custom client with explicit timeout should be preserved")
 }
}

