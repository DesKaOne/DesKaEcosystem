package iak

import (
 "context"
 "crypto/md5"
 "encoding/hex"
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "testing"
 "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/config"
 provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

func tc(base string) config.IAKConfig{return config.IAKConfig{Username:"user",APIKey:"secret",PriceListEndpoint:base+"/api/pricelist",InquiryPLNEndpoint:base+"/api/inquiry-pln",TopUpEndpoint:base+"/api/top-up",StatusEndpoint:base+"/api/check-status",BalanceEndpoint:base+"/api/check-balance"}}
func ts(s string)string{x:=md5.Sum([]byte("user"+"secret"+s));return hex.EncodeToString(x[:])}

func TestIAKAdapter(t *testing.T){
 srv:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  var p map[string]string;_ = json.NewDecoder(r.Body).Decode(&p)
  if p["username"]!="user"{t.Errorf("username=%q",p["username"])}
  switch r.URL.Path{
  case "/api/pricelist": if p["sign"]!=ts("pl"){t.Errorf("bad price signature")};w.Write([]byte(`{"data":{"pricelist":[{"product_code":"xld25000","product_description":"XL 25K","product_category":"pulsa","status":"active"}]}}`))
  case "/api/inquiry-pln": if p["sign"]!=ts("12345678901"){t.Errorf("bad inquiry signature")};w.Write([]byte(`{"data":{"status":"1","message":"SUCCESS","rc":"00"}}`))
  case "/api/top-up": if p["sign"]!=ts("order-1"){t.Errorf("bad purchase signature")};w.Write([]byte(`{"data":{"ref_id":"order-1","status":0,"product_code":"xld25000","customer_id":"08123","price":25000,"message":"PROCESS","rc":"39"}}`))
  case "/api/check-status":w.Write([]byte(`{"data":{"ref_id":"order-1","status":1,"product_code":"xld25000","customer_id":"08123","price":25000,"message":"SUCCESS","rc":"00","sn":"SN123"}}`))
  case "/api/check-balance":if p["sign"]!=ts("bl"){t.Errorf("bad balance signature")};w.Write([]byte(`{"data":{"balance":123456}}`))
  default:t.Errorf("unexpected path %s",r.URL.Path)
  }
 }));defer srv.Close()
 c,err:=New(tc(srv.URL),srv.Client());if err!=nil{t.Fatal(err)}
 active:=true;products,err:=c.GetProducts(context.Background(),provider.ProductRequest{Category:"pulsa",Active:&active});if err!=nil||len(products)!=1{t.Fatalf("products=%#v err=%v",products,err)}
 inq,err:=c.Inquiry(context.Background(),provider.InquiryRequest{ProductCode:"pln",CustomerNo:"12345678901"});if err!=nil||inq.Status!=provider.StatusSuccess{t.Fatalf("inquiry=%#v err=%v",inq,err)}
 p,err:=c.Purchase(context.Background(),provider.PurchaseRequest{ProductCode:"xld25000",CustomerNo:"08123",ReferenceID:"order-1"});if err!=nil||p.Status!=provider.StatusPending{t.Fatalf("purchase=%#v err=%v",p,err)}
 s,err:=c.GetStatus(context.Background(),provider.StatusRequest{ProductCode:"xld25000",CustomerNo:"08123",ReferenceID:"order-1"});if err!=nil||s.Status!=provider.StatusSuccess||s.SerialNumber!="SN123"{t.Fatalf("status=%#v err=%v",s,err)}
 b,err:=c.GetBalance(context.Background());if err!=nil||b!=123456{t.Fatalf("balance=%d err=%v",b,err)}
}

func TestIAKWebhookSignature(t *testing.T){
 c,_:=New(config.IAKConfig{Username:"user",APIKey:"secret",PriceListEndpoint:"https://x",InquiryPLNEndpoint:"https://x",TopUpEndpoint:"https://x",StatusEndpoint:"https://x",BalanceEndpoint:"https://x"},http.DefaultClient)
 body:=[]byte(`{"ref_id":"order-1","status":1,"code":"xld25000","hp":"08123","message":"SUCCESS","rc":"00"}`)
 e:=ts("order-1");event,err:=c.HandleWebhook(context.Background(),provider.WebhookRequest{Body:body,SignatureSecret:"secret",Signature:e});if err!=nil||event.Status!=provider.StatusSuccess{t.Fatalf("event=%#v err=%v",event,err)}
}

func TestIAKUnsupportedInquiry(t *testing.T){c,_:=New(config.IAKConfig{Username:"u",APIKey:"k"},http.DefaultClient);_,err:=c.Inquiry(context.Background(),provider.InquiryRequest{ProductCode:"foo",CustomerNo:"1"});if err!=provider.ErrUnsupportedOperation{t.Fatalf("err=%v",err)}}


func TestIAKBalanceResponseValidation(t *testing.T) {
	cases := []struct {
		name string
		body string
		wantErr bool
	}{
		{name: "missing balance", body: `{"data":{}}`, wantErr: true},
		{name: "invalid balance", body: `{"data":{"balance":"not-a-number"}}`, wantErr: true},
		{name: "string balance", body: `{"data":{"balance":"123456"}}`, wantErr: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			cfg := config.IAKConfig{
				Username: "user", APIKey: "secret",
				BalanceEndpoint: srv.URL,
			}
			c, err := New(cfg, srv.Client())
			if err != nil { t.Fatal(err) }
			balance, err := c.GetBalance(context.Background())
			if tc.wantErr && err == nil { t.Fatalf("expected error, balance=%d", balance) }
			if !tc.wantErr && (err != nil || balance != 123456) { t.Fatalf("balance=%d err=%v", balance, err) }
		})
	}
}

func TestIAKImplementsProviderCapabilities(t *testing.T) {
	c, err := New(config.IAKConfig{
		Username: "user", APIKey: "secret",
		PriceListEndpoint: "https://example.invalid/pricelist",
		InquiryPLNEndpoint: "https://example.invalid/inquiry",
		TopUpEndpoint: "https://example.invalid/top-up",
		StatusEndpoint: "https://example.invalid/status",
		BalanceEndpoint: "https://example.invalid/balance",
	}, http.DefaultClient)
	if err != nil { t.Fatal(err) }
	if _, ok := any(c).(provider.PPOBProvider); !ok { t.Fatal("IAK client must implement PPOBProvider") }
	if _, ok := any(c).(provider.BalanceProvider); !ok { t.Fatal("IAK client must implement BalanceProvider") }
}


func TestIAKProductListRequiresPricelist(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r) {
		_, _ = w.Write([]byte(`{"data":{"message":"FAILED","rc":"XX"}}`))
	}))
	defer srv.Close()
	c, err := New(tc(srv.URL), srv.Client())
	if err != nil { t.Fatal(err) }
	_, err = c.GetProducts(context.Background(), provider.ProductRequest{})
	if err == nil { t.Fatal("expected malformed product-list response error") }
}

func TestIAKInquiryRequiresStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r) {
		_, _ = w.Write([]byte(`{"data":{"message":"FAILED","rc":"XX"}}`))
	}))
	defer srv.Close()
	c, err := New(tc(srv.URL), srv.Client())
	if err != nil { t.Fatal(err) }
	_, err = c.Inquiry(context.Background(), provider.InquiryRequest{ProductCode: "pln", CustomerNo: "12345678901"})
	if err == nil { t.Fatal("expected missing inquiry status error") }
}

func TestIAKInquiryUnknownStatusIsRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r) {
		_, _ = w.Write([]byte(`{"data":{"status":"9","message":"UNKNOWN","rc":"XX"}}`))
	}))
	defer srv.Close()
	c, err := New(tc(srv.URL), srv.Client())
	if err != nil { t.Fatal(err) }
	_, err = c.Inquiry(context.Background(), provider.InquiryRequest{ProductCode: "pln", CustomerNo: "12345678901"})
	if err == nil { t.Fatal("expected unknown inquiry status error") }
}
