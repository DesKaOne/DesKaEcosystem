package xpsindonesia

import (
 "context"
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "testing"
 "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/config"
 provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

func TestXPPurchaseAndBalance(t *testing.T) {
 srv:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  if err:=r.ParseForm();err!=nil{t.Fatal(err)}
  if r.URL.Path=="/api/order.php" {
   if r.Form.Get("id")!="123"||r.Form.Get("key")!="key"||r.Form.Get("api")!="api"||r.Form.Get("trx")!="ref-1"||r.Form.Get("kod")!="i5"||r.Form.Get("isi")!="0856"{t.Fatalf("unexpected order form: %#v",r.Form)}
   _=json.NewEncoder(w).Encode(map[string]string{"success":"1","status":"proses","trx":"ref-1","kode":"i5","isi":"0856","harga":"5700"});return
  }
  if r.URL.Path=="/api/saldo.php" {_=json.NewEncoder(w).Encode(map[string]string{"success":"1","id":"123","saldo":"123000"});return}
  t.Fatalf("unexpected path %s",r.URL.Path)
 }));defer srv.Close()
 c,err:=New(config.XPSindonesiaConfig{ID:"123",Key:"key",API:"api",SaldoEndpoint:srv.URL+"/api/saldo.php",OrderEndpoint:srv.URL+"/api/order.php"},srv.Client());if err!=nil{t.Fatal(err)}
 p,err:=c.Purchase(context.Background(),provider.PurchaseRequest{ProductCode:"i5",CustomerNo:"0856",ReferenceID:"ref-1"});if err!=nil{t.Fatal(err)}
 if p.Status!=provider.StatusPending||p.Price!=5700{t.Fatalf("unexpected purchase: %#v",p)}
 b,err:=c.GetBalance(context.Background());if err!=nil||b!=123000{t.Fatalf("balance=%d err=%v",b,err)}
}

func TestXPCallbackMapping(t *testing.T) {
 c,err:=New(config.XPSindonesiaConfig{ID:"123",Key:"key",API:"api"},nil);if err!=nil{t.Fatal(err)}
 e,err:=c.HandleWebhook(context.Background(),provider.WebhookRequest{Body:[]byte("id=123&key=secret&trx=ref-1&status=sukses&kod=i5&isi=0856&sn=SN1"),SignatureSecret:"secret"});if err!=nil{t.Fatal(err)}
 if e.Status!=provider.StatusSuccess||e.SerialNumber!="SN1"{t.Fatalf("unexpected event: %#v",e)}
 _,err=c.HandleWebhook(context.Background(),provider.WebhookRequest{Body:[]byte("id=123&key=bad&trx=ref-1&status=sukses&kod=i5&isi=0856"),SignatureSecret:"secret"});if err==nil{t.Fatal("expected invalid callback key")}
}

func TestXPUnsupportedOperationsAreExplicit(t *testing.T) {
 c,err:=New(config.XPSindonesiaConfig{ID:"123",Key:"key",API:"api"},nil);if err!=nil{t.Fatal(err)}
 if _,err:=c.GetProducts(context.Background(),provider.ProductRequest{});err!=provider.ErrUnsupportedOperation{t.Fatal(err)}
 if _,err:=c.Inquiry(context.Background(),provider.InquiryRequest{});err!=provider.ErrUnsupportedOperation{t.Fatal(err)}
 if _,err:=c.GetStatus(context.Background(),provider.StatusRequest{});err!=provider.ErrUnsupportedOperation{t.Fatal(err)}
}
