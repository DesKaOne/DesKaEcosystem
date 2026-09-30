package iak

import (
 "context"
 "crypto/md5"
 "encoding/hex"
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"
 "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/config"
 provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

func iakTestConfig(base string) config.IAKConfig{return config.IAKConfig{Username:"user",APIKey:"secret",PriceListEndpoint:base+"/api/pricelist",InquiryPLNEndpoint:base+"/api/inquiry-pln",TopUpEndpoint:base+"/api/top-up",StatusEndpoint:base+"/api/check-status",BalanceEndpoint:base+"/api/check-balance"}}
func ts(s string)string{x:=md5.Sum([]byte("user"+"secret"+s));return hex.EncodeToString(x[:])}

func TestIAKAdapter(t *testing.T){
 srv:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  var p map[string]string;_ = json.NewDecoder(r.Body).Decode(&p)
  if p["username"]!="user"{t.Errorf("username=%q",p["username"])}
  switch r.URL.Path{
  case "/api/pricelist": if p["sign"]!=ts("pl"){t.Errorf("bad price signature")};w.Write([]byte(`{"data":{"pricelist":[{"product_code":"xld25000","product_description":"XL 25K","product_details":"XL 25K","product_nominal":"25000","product_price":25000,"product_type":"pulsa","active_period":"30","status":"active","icon_url":"-","product_category":"pulsa"}],"rc":"00","message":"SUCCESS"}}`))
  case "/api/inquiry-pln": if p["sign"]!=ts("12345678901"){t.Errorf("bad inquiry signature")};w.Write([]byte(`{"data":{"status":"1","customer_id":"12345678901","meter_no":"548933889287","subscriber_id":"12345678901","name":"Sintya Oktaviani","segment_power":"R1 /000001300","message":"SUCCESS","rc":"00"}}`))
  case "/api/top-up": if p["sign"]!=ts("order-1"){t.Errorf("bad purchase signature")};w.Write([]byte(`{"data":{"ref_id":"order-1","status":0,"product_code":"xld25000","customer_id":"08123","price":25000,"balance":997061249,"tr_id":3482,"message":"PROCESS","rc":"39"}}`))
  case "/api/check-status":w.Write([]byte(`{"data":{"ref_id":"order-1","status":1,"product_code":"xld25000","customer_id":"08123","price":25000,"balance":997061249,"tr_id":3482,"message":"SUCCESS","rc":"00","sn":"SN123"}}`))
  case "/api/check-balance":if p["sign"]!=ts("bl"){t.Errorf("bad balance signature")};w.Write([]byte(`{"data":{"balance":123456}}`))
  default:t.Errorf("unexpected path %s",r.URL.Path)
  }
 }));defer srv.Close()
 c,err:=New(iakTestConfig(srv.URL),srv.Client());if err!=nil{t.Fatal(err)}
 active:=true;products,err:=c.GetProducts(context.Background(),provider.ProductRequest{Category:"pulsa",Active:&active});if err!=nil||len(products)!=1{t.Fatalf("products=%#v err=%v",products,err)}
 inq,err:=c.Inquiry(context.Background(),provider.InquiryRequest{ProductCode:"pln",CustomerNo:"12345678901"});if err!=nil||inq.Status!=provider.StatusSuccess{t.Fatalf("inquiry=%#v err=%v",inq,err)}
 p,err:=c.Purchase(context.Background(),provider.PurchaseRequest{ProductCode:"xld25000",CustomerNo:"08123",ReferenceID:"order-1"});if err!=nil||p.Status!=provider.StatusPending{t.Fatalf("purchase=%#v err=%v",p,err)}
 s,err:=c.GetStatus(context.Background(),provider.StatusRequest{ProductCode:"xld25000",CustomerNo:"08123",ReferenceID:"order-1"});if err!=nil||s.Status!=provider.StatusSuccess||s.SerialNumber!="SN123"{t.Fatalf("status=%#v err=%v",s,err)}
 b,err:=c.GetBalance(context.Background());if err!=nil||b!=123456{t.Fatalf("balance=%d err=%v",b,err)}
}

func TestIAKWebhookSignature(t *testing.T){
 c,_:=New(config.IAKConfig{Username:"user",APIKey:"secret",PriceListEndpoint:"https://x",InquiryPLNEndpoint:"https://x",TopUpEndpoint:"https://x",StatusEndpoint:"https://x",BalanceEndpoint:"https://x"},http.DefaultClient)
 body:=[]byte(`{"ref_id":"order-1","status":1,"code":"xld25000","hp":"08123","price":25000,"balance":997061249,"tr_id":3482,"message":"SUCCESS","rc":"00"}`)
 e:=ts("order-1");event,err:=c.HandleWebhook(context.Background(),provider.WebhookRequest{Body:body,SignatureSecret:"secret",Signature:e});if err!=nil||event.Status!=provider.StatusSuccess{t.Fatalf("event=%#v err=%v",event,err)}
}

func TestIAKWebhookRequiresDocumentedFieldsAndState(t *testing.T) {
 c,_:=New(config.IAKConfig{Username:"user",APIKey:"secret"},http.DefaultClient)
 cases:=[]struct{name,body string}{
  {"missing rc", `{"ref_id":"order-1","status":1,"code":"xld25000","hp":"08123","price":25000,"balance":997061249,"tr_id":3482,"message":"SUCCESS","sign":"sig"}`},
  {"missing sign", `{"ref_id":"order-1","status":1,"code":"xld25000","hp":"08123","price":25000,"balance":997061249,"tr_id":3482,"message":"SUCCESS","rc":"00"}`},
  {"pending callback", `{"ref_id":"order-1","status":0,"code":"xld25000","hp":"08123","price":25000,"balance":997061249,"tr_id":3482,"message":"PROCESS","rc":"39","sign":"sig"}`},
  {"conflicting status and rc", `{"ref_id":"order-1","status":1,"code":"xld25000","hp":"08123","price":25000,"balance":997061249,"tr_id":3482,"message":"SUCCESS","rc":"39","sign":"sig"}`},
  {"missing balance", `{"ref_id":"order-1","status":1,"code":"xld25000","hp":"08123","price":25000,"tr_id":3482,"message":"SUCCESS","rc":"00","sign":"sig"}`},
  {"missing tr_id", `{"ref_id":"order-1","status":1,"code":"xld25000","hp":"08123","price":25000,"balance":997061249,"message":"SUCCESS","rc":"00","sign":"sig"}`},
 }
 for _,tc:=range cases {
  t.Run(tc.name,func(t *testing.T){
   _,err:=c.HandleWebhook(context.Background(),provider.WebhookRequest{Body:[]byte(tc.body)})
   if err==nil { t.Fatal("expected callback contract error") }
  })
 }
}

func TestIAKWebhookRejectsInvalidSignature(t *testing.T) {
 c,_:=New(config.IAKConfig{Username:"user",APIKey:"secret"},http.DefaultClient)
 body:=[]byte(`{"ref_id":"order-1","status":1,"code":"xld25000","hp":"08123","price":25000,"balance":997061249,"tr_id":3482,"message":"SUCCESS","rc":"00","sign":"bad"}`)
 _,err:=c.HandleWebhook(context.Background(),provider.WebhookRequest{Body:body,SignatureSecret:"secret"})
 if err==nil { t.Fatal("expected invalid callback signature error") }
}

func TestIAKWebhookAcceptsDocumentedFailedState(t *testing.T) {
 c,_:=New(config.IAKConfig{Username:"user",APIKey:"secret"},http.DefaultClient)
 body:=[]byte(`{"ref_id":"order-1","status":2,"code":"xld25000","hp":"08123","price":25000,"balance":997061249,"tr_id":3482,"message":"FAILED","rc":"07","sign":"sig"}`)
 event,err:=c.HandleWebhook(context.Background(),provider.WebhookRequest{Body:body})
 if err!=nil || event.Status!=provider.StatusFailed || event.ProviderCode!="07" { t.Fatalf("event=%#v err=%v",event,err) }
}

func TestIAKUnsupportedInquiry(t *testing.T){c,_:=New(config.IAKConfig{Username:"u",APIKey:"k"},http.DefaultClient);_,err:=c.Inquiry(context.Background(),provider.InquiryRequest{ProductCode:"foo",CustomerNo:"1"});if err!=provider.ErrUnsupportedOperation{t.Fatalf("err=%v",err)}}

func newIAKJSONServer(body string) (*httptest.Server, *http.Client) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	return srv, srv.Client()
}

func TestIAKBalanceResponseValidation(t *testing.T) {
	cases := []struct{name, body string; wantErr bool}{
		{"missing balance", `{"data":{}}`, true},
		{"invalid balance", `{"data":{"balance":"not-a-number"}}`, true},
		{"string balance", `{"data":{"balance":"123456"}}`, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			srv, client := newIAKJSONServer(testCase.body)
			defer srv.Close()
			cfg := config.IAKConfig{Username:"user", APIKey:"secret", BalanceEndpoint:srv.URL}
			c, err := New(cfg, client)
			if err != nil { t.Fatal(err) }
			balance, err := c.GetBalance(context.Background())
			if testCase.wantErr && err == nil { t.Fatalf("expected error, balance=%d", balance) }
			if !testCase.wantErr && (err != nil || balance != 123456) { t.Fatalf("balance=%d err=%v", balance, err) }
		})
	}
}

func TestIAKImplementsProviderCapabilities(t *testing.T) {
	c, err := New(config.IAKConfig{
		Username:"user", APIKey:"secret",
		PriceListEndpoint:"https://example.invalid/pricelist",
		InquiryPLNEndpoint:"https://example.invalid/inquiry",
		TopUpEndpoint:"https://example.invalid/top-up",
		StatusEndpoint:"https://example.invalid/status",
		BalanceEndpoint:"https://example.invalid/balance",
	}, http.DefaultClient)
	if err != nil { t.Fatal(err) }
	if _, ok := any(c).(provider.PPOBProvider); !ok { t.Fatal("IAK client must implement PPOBProvider") }
	if _, ok := any(c).(provider.BalanceProvider); !ok { t.Fatal("IAK client must implement BalanceProvider") }
}

func TestIAKProductListRequiresDocumentedMessage(t *testing.T) {
 srv, client := newIAKJSONServer(`{"data":{"pricelist":[],"rc":"00"}}`)
 defer srv.Close()
 c, err := New(iakTestConfig(srv.URL), client)
 if err != nil { t.Fatal(err) }
 _, err = c.GetProducts(context.Background(), provider.ProductRequest{})
 if err == nil { t.Fatal("expected missing pricelist message error") }
}

func TestIAKProductListRejectsIncompleteDocumentedItem(t *testing.T) {
 cases := []struct{name, item string}{
  {"missing product_details", `{"product_code":"xld25000","product_description":"XL 25K","product_nominal":"25000","product_price":25000,"product_type":"pulsa","active_period":"30","status":"active","icon_url":"-","product_category":"pulsa"}`},
  {"missing product_price", `{"product_code":"xld25000","product_description":"XL 25K","product_details":"XL 25K","product_nominal":"25000","product_type":"pulsa","active_period":"30","status":"active","icon_url":"-","product_category":"pulsa"}`},
  {"missing status", `{"product_code":"xld25000","product_description":"XL 25K","product_details":"XL 25K","product_nominal":"25000","product_price":25000,"product_type":"pulsa","active_period":"30","icon_url":"-","product_category":"pulsa"}`},
  {"missing category", `{"product_code":"xld25000","product_description":"XL 25K","product_details":"XL 25K","product_nominal":"25000","product_price":25000,"product_type":"pulsa","active_period":"30","status":"active","icon_url":"-"}`},
 }
 for _,tc:=range cases {
  t.Run(tc.name,func(t *testing.T){
   srv,client:=newIAKJSONServer(`{"data":{"pricelist":[`+tc.item+`],"rc":"00","message":"SUCCESS"}}`)
   defer srv.Close()
   c,err:=New(iakTestConfig(srv.URL),client);if err!=nil{t.Fatal(err)}
   if _,err=c.GetProducts(context.Background(),provider.ProductRequest{});err==nil{t.Fatal("expected incomplete pricelist item error")}
  })
 }
}

func TestIAKProductListRejectsInvalidItemStatus(t *testing.T) {
 srv, client := newIAKJSONServer(`{"data":{"pricelist":[{"product_code":"xld25000","product_description":"XL 25K","product_details":"XL 25K","product_nominal":"25000","product_price":25000,"product_type":"pulsa","active_period":"30","status":"unknown","icon_url":"-","product_category":"pulsa"}],"rc":"00","message":"SUCCESS"}}`)
 defer srv.Close()
 c, err := New(iakTestConfig(srv.URL), client)
 if err != nil { t.Fatal(err) }
 if _, err = c.GetProducts(context.Background(), provider.ProductRequest{}); err == nil { t.Fatal("expected invalid pricelist item status error") }
}

func TestIAKProductListRejectsDocumentedFailedResponseCode(t *testing.T) {
 srv, client := newIAKJSONServer(`{"data":{"pricelist":[],"rc":"20","message":"CODE NOT FOUND"}}`)
 defer srv.Close()
 c, err := New(iakTestConfig(srv.URL), client)
 if err != nil { t.Fatal(err) }
 _, err = c.GetProducts(context.Background(), provider.ProductRequest{})
 if err == nil { t.Fatal("expected documented failed pricelist response code error") }
}

func TestIAKProductListRejectsPendingResponseCode(t *testing.T) {
 srv, client := newIAKJSONServer(`{"data":{"pricelist":[],"rc":"39","message":"PROCESS"}}`)
 defer srv.Close()
 c, err := New(iakTestConfig(srv.URL), client)
 if err != nil { t.Fatal(err) }
 _, err = c.GetProducts(context.Background(), provider.ProductRequest{})
 if err == nil { t.Fatal("expected pending pricelist response code error") }
}

func TestIAKProductListRejectsUnknownResponseCode(t *testing.T) {
 srv, client := newIAKJSONServer(`{"data":{"pricelist":[],"rc":"999","message":"UNKNOWN"}}`)
 defer srv.Close()
 c, err := New(iakTestConfig(srv.URL), client)
 if err != nil { t.Fatal(err) }
 _, err = c.GetProducts(context.Background(), provider.ProductRequest{})
 if err == nil { t.Fatal("expected unknown pricelist response code error") }
}

func TestIAKHTTP400ExposesErrorDetails(t *testing.T) {
  srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
    w.WriteHeader(http.StatusBadRequest)
    _, _ = w.Write([]byte(`{"error_details":"missing username"}`))
  }))
  defer srv.Close()
  c, err := New(iakTestConfig(srv.URL), srv.Client())
  if err != nil { t.Fatal(err) }
  _, err = c.GetProducts(context.Background(), provider.ProductRequest{})
  if err == nil || !strings.Contains(err.Error(), "missing username") {
    t.Fatalf("expected IAK HTTP 400 error_details, err=%v", err)
  }
}

func TestIAKHTTP400PreservesStructuredErrorDetails(t *testing.T) {
  srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
    w.WriteHeader(http.StatusBadRequest)
    _, _ = w.Write([]byte(`{"error_details":{"field":"sign","message":"invalid"}}`))
  }))
  defer srv.Close()
  c, err := New(iakTestConfig(srv.URL), srv.Client())
  if err != nil { t.Fatal(err) }
  _, err = c.GetProducts(context.Background(), provider.ProductRequest{})
  if err == nil || !strings.Contains(err.Error(), "invalid") {
    t.Fatalf("expected structured IAK HTTP 400 error_details, err=%v", err)
  }
}

func TestIAKProductListRequiresPricelist(t *testing.T) {
	srv, client := newIAKJSONServer(`{"data":{"message":"FAILED","rc":"XX"}}`)
	defer srv.Close()
	c, err := New(iakTestConfig(srv.URL), client)
	if err != nil { t.Fatal(err) }
	_, err = c.GetProducts(context.Background(), provider.ProductRequest{})
	if err == nil { t.Fatal("expected malformed product-list response error") }
}

func TestIAKInquiryRequiresStatus(t *testing.T) {
	srv, client := newIAKJSONServer(`{"data":{"message":"FAILED","rc":"XX"}}`)
	defer srv.Close()
	c, err := New(iakTestConfig(srv.URL), client)
	if err != nil { t.Fatal(err) }
	_, err = c.Inquiry(context.Background(), provider.InquiryRequest{ProductCode:"pln", CustomerNo:"12345678901"})
	if err == nil { t.Fatal("expected missing inquiry status error") }
}

func TestIAKInquiryRequiresDocumentedFields(t *testing.T) {
 cases:=[]struct{name,body string}{
  {"missing rc", `{"data":{"status":"1","customer_id":"12345678901","meter_no":"548933889287","subscriber_id":"12345678901","name":"Sintya","segment_power":"R1 /000001300","message":"SUCCESS"}}`},
  {"missing meter_no", `{"data":{"status":"1","customer_id":"12345678901","subscriber_id":"12345678901","name":"Sintya","segment_power":"R1 /000001300","message":"SUCCESS","rc":"00"}}`},
  {"missing subscriber_id", `{"data":{"status":"1","customer_id":"12345678901","meter_no":"548933889287","name":"Sintya","segment_power":"R1 /000001300","message":"SUCCESS","rc":"00"}}`},
  {"missing name", `{"data":{"status":"1","customer_id":"12345678901","meter_no":"548933889287","subscriber_id":"12345678901","segment_power":"R1 /000001300","message":"SUCCESS","rc":"00"}}`},
  {"missing segment_power", `{"data":{"status":"1","customer_id":"12345678901","meter_no":"548933889287","subscriber_id":"12345678901","name":"Sintya","message":"SUCCESS","rc":"00"}}`},
 }
 for _,tc:=range cases {
  t.Run(tc.name,func(t *testing.T){
   srv,client:=newIAKJSONServer(tc.body);defer srv.Close()
   c,err:=New(iakTestConfig(srv.URL),client);if err!=nil{t.Fatal(err)}
   _,err=c.Inquiry(context.Background(),provider.InquiryRequest{ProductCode:"pln",CustomerNo:"12345678901"})
   if err==nil{t.Fatal("expected documented inquiry field validation error")}
  })
 }
}

func TestIAKInquiryAcceptsDocumentedFailedState(t *testing.T) {
 srv,client:=newIAKJSONServer(`{"data":{"status":"2","customer_id":"12345678901","meter_no":"548933889287","subscriber_id":"12345678901","name":"Sintya Oktaviani","segment_power":"R1 /000001300","message":"FAILED","rc":"07"}}`)
 defer srv.Close()
 c,err:=New(iakTestConfig(srv.URL),client);if err!=nil{t.Fatal(err)}
 result,err:=c.Inquiry(context.Background(),provider.InquiryRequest{ProductCode:"pln",CustomerNo:"12345678901"})
 if err!=nil||result.Status!=provider.StatusFailed||result.ProviderCode!="07"{t.Fatalf("result=%#v err=%v",result,err)}
}

func TestIAKInquiryRejectsStatusAndRCConflict(t *testing.T) {
 srv,client:=newIAKJSONServer(`{"data":{"status":"1","customer_id":"12345678901","meter_no":"548933889287","subscriber_id":"12345678901","name":"Sintya","segment_power":"R1 /000001300","message":"SUCCESS","rc":"07"}}`)
 defer srv.Close()
 c,err:=New(iakTestConfig(srv.URL),client);if err!=nil{t.Fatal(err)}
 _,err=c.Inquiry(context.Background(),provider.InquiryRequest{ProductCode:"pln",CustomerNo:"12345678901"})
 if err==nil{t.Fatal("expected inquiry status/rc conflict error")}
}

func TestIAKInquiryUnknownStatusIsRejected(t *testing.T) {
	srv, client := newIAKJSONServer(`{"data":{"status":"9","message":"UNKNOWN","rc":"XX"}}`)
	defer srv.Close()
	c, err := New(iakTestConfig(srv.URL), client)
	if err != nil { t.Fatal(err) }
	_, err = c.Inquiry(context.Background(), provider.InquiryRequest{ProductCode:"pln", CustomerNo:"12345678901"})
	if err == nil { t.Fatal("expected unknown inquiry status error") }
}

func TestIAKPurchaseResponseValidation(t *testing.T) {
	cases := []struct{name, body string; wantErr bool}{
		{"missing identity", `{"data":{"status":0}}`, true},
		{"unknown status", `{"data":{"ref_id":"order-1","customer_id":"08123","product_code":"xld25000","status":9}}`, true},
		{"missing message", `{"data":{"ref_id":"order-1","customer_id":"08123","product_code":"xld25000","status":0,"price":25000}}`, true},
		{"missing price", `{"data":{"ref_id":"order-1","customer_id":"08123","product_code":"xld25000","status":0,"message":"PROCESS"}}`, true},
		{"valid pending", `{"data":{"ref_id":"order-1","customer_id":"08123","product_code":"xld25000","status":0,"price":25000,"balance":997061249,"tr_id":3482,"message":"PROCESS","rc":"39"}}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, client := newIAKJSONServer(tc.body)
			defer srv.Close()
			cfg := iakTestConfig(srv.URL)
			c, err := New(cfg, client)
			if err != nil { t.Fatal(err) }
			result, err := c.Purchase(context.Background(), provider.PurchaseRequest{ProductCode:"xld25000", CustomerNo:"08123", ReferenceID:"order-1"})
			if tc.wantErr && err == nil { t.Fatalf("expected purchase response error: %#v", result) }
			if !tc.wantErr && (err != nil || result.Status != provider.StatusPending) { t.Fatalf("result=%#v err=%v", result, err) }
		})
	}
}

func TestIAKTransactionResponseRejectsConflictingStatusAndRC(t *testing.T) {
 cases := []struct{name, body string}{
  {"purchase", `{"data":{"ref_id":"order-1","customer_id":"08123","product_code":"xld25000","status":1,"price":25000,"balance":997061249,"tr_id":3482,"message":"SUCCESS","rc":"39"}}`},
  {"status", `{"data":{"ref_id":"order-1","customer_id":"08123","product_code":"xld25000","status":2,"price":25000,"balance":997061249,"tr_id":3482,"message":"FAILED","rc":"00"}}`},
 }
 for _, tc := range cases {
  t.Run(tc.name, func(t *testing.T) {
   srv, client := newIAKJSONServer(tc.body)
   defer srv.Close()
   c, err := New(iakTestConfig(srv.URL), client)
   if err != nil { t.Fatal(err) }
   if tc.name=="purchase" {
    _, err = c.Purchase(context.Background(), provider.PurchaseRequest{ProductCode:"xld25000",CustomerNo:"08123",ReferenceID:"order-1"})
   } else {
    _, err = c.GetStatus(context.Background(), provider.StatusRequest{ProductCode:"xld25000",CustomerNo:"08123",ReferenceID:"order-1"})
   }
   if err == nil { t.Fatal("expected status/rc conflict error") }
  })
 }
}

func TestIAKTransactionRejectsBlankMessage(t *testing.T) {
 for _, message := range []string{"", "   ", "\t"} {
  t.Run("message_"+strings.TrimSpace(message), func(t *testing.T) {
   srv, client := newIAKJSONServer(`{"data":{"ref_id":"order-1","customer_id":"08123","product_code":"xld25000","status":1,"price":25000,"balance":997061249,"tr_id":3482,"message":"`+message+`","rc":"00"}}`)
   defer srv.Close()
   c, err := New(iakTestConfig(srv.URL), client)
   if err != nil { t.Fatal(err) }
   if _, err = c.GetStatus(context.Background(), provider.StatusRequest{ReferenceID:"order-1",CustomerNo:"08123",ProductCode:"xld25000"}); err == nil { t.Fatal("expected blank message to be rejected") }
  })
 }
}

func TestIAKTransactionResponseRequiresRC(t *testing.T) {
 srv, client := newIAKJSONServer(`{"data":{"ref_id":"order-1","customer_id":"08123","product_code":"xld25000","status":0,"price":25000,"balance":997061249,"tr_id":3482,"message":"PROCESS"}}`)
 defer srv.Close()
 c, err := New(iakTestConfig(srv.URL), client)
 if err != nil { t.Fatal(err) }
 _, err = c.GetStatus(context.Background(), provider.StatusRequest{ProductCode:"xld25000",CustomerNo:"08123",ReferenceID:"order-1"})
 if err == nil { t.Fatal("expected missing rc error") }
}

func TestIAKTransactionResponseRequiresBalanceAndTransactionID(t *testing.T) {
 cases := []struct{name, body string}{
  {"missing balance", `{"data":{"ref_id":"order-1","customer_id":"08123","product_code":"xld25000","status":0,"price":25000,"tr_id":3482,"message":"PROCESS","rc":"39"}}`},
  {"missing tr_id", `{"data":{"ref_id":"order-1","customer_id":"08123","product_code":"xld25000","status":0,"price":25000,"balance":997061249,"message":"PROCESS","rc":"39"}}`},
 }
 for _, tc := range cases {
  t.Run(tc.name, func(t *testing.T) {
   srv, client := newIAKJSONServer(tc.body)
   defer srv.Close()
   c, err := New(iakTestConfig(srv.URL), client)
   if err != nil { t.Fatal(err) }
   _, err = c.Purchase(context.Background(), provider.PurchaseRequest{ProductCode:"xld25000",CustomerNo:"08123",ReferenceID:"order-1"})
   if err == nil { t.Fatal("expected mandatory transaction field error") }
  })
 }
}

func TestIAKStatusResponseRequiresIdentity(t *testing.T) {
 cases:=[]string{
  `{"data":{"status":1,"price":25000,"balance":997061249,"tr_id":3482,"message":"SUCCESS","rc":"00"}}`,
  `{"data":{"ref_id":"order-1","status":1,"price":25000,"balance":997061249,"tr_id":3482,"message":"SUCCESS","rc":"00"}}`,
  `{"data":{"ref_id":"order-1","customer_id":"08123","status":1,"price":25000,"balance":997061249,"tr_id":3482,"message":"SUCCESS","rc":"00"}}`,
 }
 for i,body:=range cases {
  t.Run(fmt.Sprintf("missing_identity_%d",i),func(t *testing.T){
   srv,client:=newIAKJSONServer(body);defer srv.Close()
   c,err:=New(iakTestConfig(srv.URL),client);if err!=nil{t.Fatal(err)}
   if _,err=c.GetStatus(context.Background(),provider.StatusRequest{ReferenceID:"order-1",CustomerNo:"08123",ProductCode:"xld25000"});err==nil{t.Fatal("expected missing status identity error")}
  })
 }
}

func TestIAKPurchaseResponseIdentityMismatch(t *testing.T) {
	srv, client := newIAKJSONServer(`{"data":{"ref_id":"other","customer_id":"08123","product_code":"xld25000","status":0}}`)
	defer srv.Close()
	c, err := New(iakTestConfig(srv.URL), client)
	if err != nil { t.Fatal(err) }
	_, err = c.Purchase(context.Background(), provider.PurchaseRequest{ProductCode:"xld25000", CustomerNo:"08123", ReferenceID:"order-1"})
	if err == nil { t.Fatal("expected reference identity mismatch") }
}

func TestIAKStatusResponseValidation(t *testing.T) {
	cases := []struct{name, body string; wantErr bool}{
		{"missing identity", `{"data":{"status":1}}`, true},
		{"unknown status", `{"data":{"ref_id":"order-1","customer_id":"08123","product_code":"xld25000","status":"9"}}`, true},
		{"missing message", `{"data":{"ref_id":"order-1","customer_id":"08123","product_code":"xld25000","status":"1","price":25000}}`, true},
		{"missing price", `{"data":{"ref_id":"order-1","customer_id":"08123","product_code":"xld25000","status":"1","message":"SUCCESS"}}`, true},
		{"valid success", `{"data":{"ref_id":"order-1","customer_id":"08123","product_code":"xld25000","status":"1","price":25000,"balance":997061249,"tr_id":3482,"message":"SUCCESS","rc":"00"}}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, client := newIAKJSONServer(tc.body)
			defer srv.Close()
			c, err := New(iakTestConfig(srv.URL), client)
			if err != nil { t.Fatal(err) }
			result, err := c.GetStatus(context.Background(), provider.StatusRequest{ProductCode:"xld25000", CustomerNo:"08123", ReferenceID:"order-1"})
			if tc.wantErr && err == nil { t.Fatalf("expected status response error: %#v", result) }
			if !tc.wantErr && (err != nil || result.Status != provider.StatusSuccess) { t.Fatalf("result=%#v err=%v", result, err) }
		})
	}
}

func TestIAKInquiryRejectsMissingCustomerID(t *testing.T) {
 srv, client := newIAKJSONServer(`{"data":{"status":"1","message":"SUCCESS","rc":"00"}}`)
 defer srv.Close()
 c, err := New(iakTestConfig(srv.URL), client)
 if err != nil { t.Fatal(err) }
 _, err = c.Inquiry(context.Background(), provider.InquiryRequest{ProductCode:"pln", CustomerNo:"12345678901"})
 if err == nil { t.Fatal("expected missing inquiry customer ID error") }
}

func TestIAKInquiryRejectsCustomerIDMismatch(t *testing.T) {
 srv, client := newIAKJSONServer(`{"data":{"status":"1","customer_id":"99999999999","message":"SUCCESS","rc":"00"}}`)
 defer srv.Close()
 c, err := New(iakTestConfig(srv.URL), client)
 if err != nil { t.Fatal(err) }
 _, err = c.Inquiry(context.Background(), provider.InquiryRequest{ProductCode:"pln", CustomerNo:"12345678901"})
 if err == nil { t.Fatal("expected inquiry customer ID mismatch") }
}

func TestIAKInquiryRejectsInvalidStatus(t *testing.T) {
 srv, client := newIAKJSONServer(`{"data":{"status":"0","customer_id":"12345678901","message":"PROCESS","rc":"39"}}`)
 defer srv.Close()
 c, err := New(iakTestConfig(srv.URL), client)
 if err != nil { t.Fatal(err) }
 _, err = c.Inquiry(context.Background(), provider.InquiryRequest{ProductCode:"pln", CustomerNo:"12345678901"})
 if err == nil { t.Fatal("expected invalid inquiry status error") }
}

func TestIAKInquiryRequiresMessage(t *testing.T) {
	srv, client := newIAKJSONServer(`{"data":{"status":"1","rc":"00"}}`)
	defer srv.Close()
	c, err := New(iakTestConfig(srv.URL), client)
	if err != nil { t.Fatal(err) }
	_, err = c.Inquiry(context.Background(), provider.InquiryRequest{ProductCode:"pln", CustomerNo:"12345678901"})
	if err == nil { t.Fatal("expected missing inquiry message error") }
}

func TestIAKWebhookAcceptsDocumentedV2IdentityFields(t *testing.T) {
 c,_:=New(config.IAKConfig{Username:"user",APIKey:"secret"},http.DefaultClient)
 body:=[]byte(`{"ref_id":"order-v2","status":"1","product_code":"xld25000","customer_id":"08123","price":"25000","balance":"997061249","tr_id":"3482","message":"SUCCESS","rc":"00","sign":"sig"}`)
 event,err:=c.HandleWebhook(context.Background(),provider.WebhookRequest{Body:body})
 if err!=nil || event.ReferenceID!="order-v2" || event.CustomerNo!="08123" || event.ProductCode!="xld25000" || event.Status!=provider.StatusSuccess || event.Price!=25000 {
  t.Fatalf("event=%#v err=%v",event,err)
 }
}

func TestIAKWebhookRejectsConflictingIdentityAliases(t *testing.T) {
 c,_:=New(config.IAKConfig{Username:"user",APIKey:"secret"},http.DefaultClient)
 cases:=[]string{
  `{"ref_id":"order-1","status":"1","product_code":"xld25000","code":"xld50000","customer_id":"08123","hp":"08123","price":"25000","balance":"997061249","tr_id":"3482","message":"SUCCESS","rc":"00","sign":"sig"}`,
  `{"ref_id":"order-1","status":"1","product_code":"xld25000","customer_id":"08123","hp":"08999","price":"25000","balance":"997061249","tr_id":"3482","message":"SUCCESS","rc":"00","sign":"sig"}`,
 }
 for _,body:=range cases {
  if _,err:=c.HandleWebhook(context.Background(),provider.WebhookRequest{Body:[]byte(body)}); err==nil {
   t.Fatalf("expected conflicting identity aliases to be rejected: %s",body)
  }
 }
}

func TestIAKWebhookRejectsMalformedTransaction(t *testing.T) {
	c, _ := New(config.IAKConfig{Username:"user", APIKey:"secret"}, http.DefaultClient)
	_, err := c.HandleWebhook(context.Background(), provider.WebhookRequest{Body:[]byte(`{"ref_id":"order-1","status":"9","code":"xld25000","hp":"08123"}`), SignatureSecret:"secret", Signature:ts("order-1")})
	if err == nil { t.Fatal("expected malformed webhook error") }
}

func TestIAKWebhookRequiresMessageAndPrice(t *testing.T) {
	c, _ := New(config.IAKConfig{Username:"user", APIKey:"secret"}, http.DefaultClient)
	cases := []string{`{"ref_id":"order-1","status":"1","code":"xld25000","hp":"08123","price":25000}`, `{"ref_id":"order-1","status":"1","code":"xld25000","hp":"08123","message":"SUCCESS"}`}
	for _, body := range cases {
		_, err := c.HandleWebhook(context.Background(), provider.WebhookRequest{Body:[]byte(body), SignatureSecret:"secret", Signature:ts("order-1")})
		if err == nil { t.Fatalf("expected webhook field validation error for %s", body) }
	}
}

func TestIAKProductListItemValidation(t *testing.T) {
	cases := []struct{name, body string}{
		{"invalid item type", `{"data":{"pricelist":["bad"]}}`},
		{"missing product code", `{"data":{"pricelist":[{"product_description":"XL 25K","product_category":"pulsa","status":"active"}]}}`},
		{"missing product description", `{"data":{"pricelist":[{"product_code":"xld25000","product_category":"pulsa","status":"active"}]}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, client := newIAKJSONServer(tc.body)
			defer srv.Close()
			c, err := New(iakTestConfig(srv.URL), client)
			if err != nil { t.Fatal(err) }
			_, err = c.GetProducts(context.Background(), provider.ProductRequest{})
			if err == nil { t.Fatal("expected invalid pricelist item error") }
		})
	}
}

func TestIAKBalanceRejectsFractionalJSONNumber(t *testing.T) {
 srv, client := newIAKJSONServer(`{"data":{"balance":997136249.5}}`)
 defer srv.Close()
 c, err := New(iakTestConfig(srv.URL), client)
 if err != nil { t.Fatal(err) }
 _, err = c.GetBalance(context.Background())
 if err == nil { t.Fatal("expected fractional IAK balance to be rejected") }
}

func TestIAKBalanceAcceptsIntegerJSONNumber(t *testing.T) {
 srv, client := newIAKJSONServer(`{"data":{"balance":997136249}}`)
 defer srv.Close()
 c, err := New(iakTestConfig(srv.URL), client)
 if err != nil { t.Fatal(err) }
 got, err := c.GetBalance(context.Background())
 if err != nil || got != 997136249 { t.Fatalf("balance=%d err=%v", got, err) }
}

func TestIAKMapResponseCodeCoversDocumentedCodes(t *testing.T) {
	cases:=map[string]provider.TransactionStatus{"00":provider.StatusSuccess,"39":provider.StatusPending,"201":provider.StatusPending,"06":provider.StatusFailed,"07":provider.StatusFailed,"10":provider.StatusFailed,"12":provider.StatusFailed,"13":provider.StatusFailed,"14":provider.StatusFailed,"16":provider.StatusFailed,"17":provider.StatusFailed,"18":provider.StatusFailed,"19":provider.StatusFailed,"20":provider.StatusFailed,"21":provider.StatusFailed,"102":provider.StatusFailed,"106":provider.StatusFailed,"107":provider.StatusFailed,"110":provider.StatusFailed,"117":provider.StatusFailed,"121":provider.StatusFailed,"131":provider.StatusFailed,"132":provider.StatusFailed,"141":provider.StatusFailed,"142":provider.StatusFailed,"202":provider.StatusFailed,"203":provider.StatusFailed,"204":provider.StatusFailed,"205":provider.StatusFailed,"206":provider.StatusFailed,"207":provider.StatusFailed}
	for rc,want:=range cases { t.Run(rc,func(t *testing.T){ got,err:=mapResponseCode(rc); if err!=nil||got!=want{t.Fatalf("rc %s => %q,%v; want %q",rc,got,err,want)} }) }
}
func TestIAKMapResponseCodeRejectsUnknown(t *testing.T) {
	if _,err:=mapResponseCode("999"); err==nil{t.Fatal("expected unknown response code error")}
}


func TestIAKResponseCodeMapperMatchesCurrentPrepaidContract(t *testing.T) {
	success := []string{"00"}
	pending := []string{"39", "201"}
	failed := []string{"06", "07", "10", "12", "13", "14", "16", "17", "18", "19", "20", "21", "102", "106", "107", "110", "117", "121", "131", "132", "141", "142", "202", "203", "204", "205", "206", "207"}
	for _, rc := range success {
		if got, err := mapResponseCode(rc); err != nil || got != provider.StatusSuccess { t.Fatalf("rc %q: got=%q err=%v, want success", rc, got, err) }
	}
	for _, rc := range pending {
		if got, err := mapResponseCode(rc); err != nil || got != provider.StatusPending { t.Fatalf("rc %q: got=%q err=%v, want pending", rc, got, err) }
	}
	for _, rc := range failed {
		if got, err := mapResponseCode(rc); err != nil || got != provider.StatusFailed { t.Fatalf("rc %q: got=%q err=%v, want failed", rc, got, err) }
	}
	for _, rc := range []string{"01", "02", "03", "04", "05", "08", "09", "11", "15", "30", "40", "43", "76", "77", "91", "92", "93", "94", "100", "101", "103", "105", "108", "109", "143", "301", "999"} {
		if _, err := mapResponseCode(rc); err == nil { t.Fatalf("undocumented prepaid response code %q must fail closed", rc) }
	}
}

func TestIAKTransactionRejectsFractionalTrID(t *testing.T) {
 srv, client := newIAKJSONServer(`{"data":{"ref_id":"order-1","status":1,"product_code":"xld25000","customer_id":"08123","price":25000,"balance":997061249,"tr_id":3482.5,"message":"SUCCESS","rc":"00"}}`)
 defer srv.Close()
 c, err := New(iakTestConfig(srv.URL), client)
 if err != nil { t.Fatal(err) }
 _, err = c.GetStatus(context.Background(), provider.StatusRequest{ReferenceID:"order-1", CustomerNo:"08123", ProductCode:"xld25000"})
 if err == nil { t.Fatal("expected fractional tr_id to be rejected") }
}

func TestIAKTransactionRejectsFractionalStatus(t *testing.T) {
 srv, client := newIAKJSONServer(`{"data":{"ref_id":"order-1","status":1.5,"product_code":"xld25000","customer_id":"08123","price":25000,"balance":997061249,"tr_id":3482,"message":"SUCCESS","rc":"00"}}`)
 defer srv.Close()
 c, err := New(iakTestConfig(srv.URL), client)
 if err != nil { t.Fatal(err) }
 _, err = c.GetStatus(context.Background(), provider.StatusRequest{ReferenceID:"order-1", CustomerNo:"08123", ProductCode:"xld25000"})
 if err == nil { t.Fatal("expected fractional status to be rejected") }
}

func TestIAKTransactionRejectsFractionalPrice(t *testing.T) {
 srv, client := newIAKJSONServer(`{"data":{"ref_id":"order-1","status":1,"product_code":"xld25000","customer_id":"08123","price":25000.5,"balance":997061249,"tr_id":3482,"message":"SUCCESS","rc":"00"}}`)
 defer srv.Close()
 c, err := New(iakTestConfig(srv.URL), client)
 if err != nil { t.Fatal(err) }
 _, err = c.GetStatus(context.Background(), provider.StatusRequest{ReferenceID:"order-1", CustomerNo:"08123", ProductCode:"xld25000"})
 if err == nil { t.Fatal("expected fractional price to be rejected") }
}

func TestIAKTransactionAcceptsIntegerPrice(t *testing.T) {
 srv, client := newIAKJSONServer(`{"data":{"ref_id":"order-1","status":1,"product_code":"xld25000","customer_id":"08123","price":25000,"balance":997061249,"tr_id":3482,"message":"SUCCESS","rc":"00"}}`)
 defer srv.Close()
 c, err := New(iakTestConfig(srv.URL), client)
 if err != nil { t.Fatal(err) }
 got, err := c.GetStatus(context.Background(), provider.StatusRequest{ReferenceID:"order-1", CustomerNo:"08123", ProductCode:"xld25000"})
 if err != nil || got.Status != provider.StatusSuccess || got.Price != 25000 { t.Fatalf("status=%#v err=%v", got, err) }
}

func TestIAKTransactionAcceptsIntegerTrID(t *testing.T) {
 srv, client := newIAKJSONServer(`{"data":{"ref_id":"order-1","status":1,"product_code":"xld25000","customer_id":"08123","price":25000,"balance":997061249,"tr_id":3482,"message":"SUCCESS","rc":"00"}}`)
 defer srv.Close()
 c, err := New(iakTestConfig(srv.URL), client)
 if err != nil { t.Fatal(err) }
 got, err := c.GetStatus(context.Background(), provider.StatusRequest{ReferenceID:"order-1", CustomerNo:"08123", ProductCode:"xld25000"})
 if err != nil || got.Status != provider.StatusSuccess { t.Fatalf("status=%#v err=%v", got, err) }
}


func TestIAKBalanceRejectsOutOfRangeJSONNumber(t *testing.T) {
 for _, raw := range []string{`9223372036854775808`, `-9223372036854775809`} {
  t.Run(raw, func(t *testing.T) {
   srv, client := newIAKJSONServer(`{"data":{"balance":`+raw+`}}`)
   defer srv.Close()
   c, err := New(iakTestConfig(srv.URL), client)
   if err != nil { t.Fatal(err) }
   _, err = c.GetBalance(context.Background())
   if err == nil { t.Fatal("expected out-of-range IAK balance to be rejected") }
  })
 }
}

func TestIAKTransactionRejectsOutOfRangePrice(t *testing.T) {
 srv, client := newIAKJSONServer(`{"data":{"ref_id":"order-1","status":1,"product_code":"xld25000","customer_id":"08123","price":9223372036854775808,"balance":997061249,"tr_id":3482,"message":"SUCCESS","rc":"00"}}`)
 defer srv.Close()
 c, err := New(iakTestConfig(srv.URL), client)
 if err != nil { t.Fatal(err) }
 _, err = c.GetStatus(context.Background(), provider.StatusRequest{ReferenceID:"order-1", CustomerNo:"08123", ProductCode:"xld25000"})
 if err == nil { t.Fatal("expected out-of-range transaction price to be rejected") }
}

func TestIAKWebhookRejectsOutOfRangePrice(t *testing.T) {
 c, _ := New(config.IAKConfig{Username:"user", APIKey:"secret"}, http.DefaultClient)
 body := []byte(`{"ref_id":"order-1","status":1,"code":"xld25000","hp":"08123","price":"9223372036854775808","balance":"997061249","tr_id":"3482","message":"SUCCESS","rc":"00","sign":"sig"}`)
 _, err := c.HandleWebhook(context.Background(), provider.WebhookRequest{Body:body})
 if err == nil { t.Fatal("expected out-of-range webhook price to be rejected") }
}

func TestIAKTransactionAcceptsInt64BoundaryPrice(t *testing.T) {
 srv, client := newIAKJSONServer(`{"data":{"ref_id":"order-1","status":1,"product_code":"xld25000","customer_id":"08123","price":"9223372036854775807","balance":997061249,"tr_id":3482,"message":"SUCCESS","rc":"00"}}`)
 defer srv.Close()
 c, err := New(iakTestConfig(srv.URL), client)
 if err != nil { t.Fatal(err) }
 got, err := c.GetStatus(context.Background(), provider.StatusRequest{ReferenceID:"order-1", CustomerNo:"08123", ProductCode:"xld25000"})
 if err != nil || got.Price != 9223372036854775807 { t.Fatalf("status=%#v err=%v", got, err) }
}


func TestIAKTransactionRejectsOutOfRangeTrID(t *testing.T) {
 srv, client := newIAKJSONServer(`{"data":{"ref_id":"order-1","status":1,"product_code":"xld25000","customer_id":"08123","price":25000,"balance":997061249,"tr_id":"9223372036854775808","message":"SUCCESS","rc":"00"}}`)
 defer srv.Close()
 c, err := New(iakTestConfig(srv.URL), client)
 if err != nil { t.Fatal(err) }
 _, err = c.GetStatus(context.Background(), provider.StatusRequest{ReferenceID:"order-1", CustomerNo:"08123", ProductCode:"xld25000"})
 if err == nil { t.Fatal("expected out-of-range transaction ID to be rejected") }
}


func TestIAKWebhookRejectsOutOfRangeTrID(t *testing.T) {
 c, _ := New(config.IAKConfig{Username:"user", APIKey:"secret"}, http.DefaultClient)
 body := []byte(`{"ref_id":"order-1","status":"1","code":"xld25000","hp":"08123","price":"25000","balance":"997061249","tr_id":"9223372036854775808","message":"SUCCESS","rc":"00","sign":"sig"}`)
 _, err := c.HandleWebhook(context.Background(), provider.WebhookRequest{Body:body})
 if err == nil { t.Fatal("expected out-of-range webhook transaction ID to be rejected") }
}

func TestIAKWebhookRejectsNonFiniteBalance(t *testing.T) {
 c, _ := New(config.IAKConfig{Username:"user", APIKey:"secret"}, http.DefaultClient)
 for _, balance := range []string{"NaN", "+Inf", "-Inf"} {
  t.Run(balance, func(t *testing.T) {
   body := []byte(`{"ref_id":"order-1","status":"1","code":"xld25000","hp":"08123","price":"25000","balance":"`+balance+`","tr_id":"3482","message":"SUCCESS","rc":"00","sign":"sig"}`)
   _, err := c.HandleWebhook(context.Background(), provider.WebhookRequest{Body:body})
   if err == nil { t.Fatalf("expected non-finite webhook balance %q to be rejected", balance) }
  })
 }
}
