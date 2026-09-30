package iak

import (
 "context"
 "crypto/md5"
 "crypto/subtle"
 "encoding/hex"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "math"
 "net/http"
 "strconv"
 "strings"
 "time"

 "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/config"
 provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

const defaultHTTPTimeout = 15 * time.Second

type iakHTTPStatusError struct { StatusCode int; Body string }
func (e *iakHTTPStatusError) Error() string {
 if strings.TrimSpace(e.Body) != "" { return fmt.Sprintf("IAK HTTP status %d: %s", e.StatusCode, strings.TrimSpace(e.Body)) }
 return fmt.Sprintf("IAK HTTP status %d", e.StatusCode)
}

type Client struct {
 username, apiKey string
 priceListEndpoint, inquiryPLNEndpoint, topUpEndpoint, statusEndpoint, balanceEndpoint string
 httpClient *http.Client
}

func New(cfg config.IAKConfig, httpClient *http.Client) (*Client,error) {
 if cfg.Username==""||cfg.APIKey=="" { return nil,errors.New("IAK username and API key are required") }
 if httpClient==nil { httpClient=&http.Client{Timeout:defaultHTTPTimeout} } else if httpClient.Timeout<=0 { copy:=*httpClient; copy.Timeout=defaultHTTPTimeout; httpClient=&copy }
 return &Client{cfg.Username,cfg.APIKey,cfg.PriceListEndpoint,cfg.InquiryPLNEndpoint,cfg.TopUpEndpoint,cfg.StatusEndpoint,cfg.BalanceEndpoint,httpClient},nil
}

func (c *Client) GetProducts(ctx context.Context, req provider.ProductRequest)([]provider.Product,error) {
 p:=map[string]string{"username":c.username,"sign":c.sig("pl"),"status":"all"}
 if req.Active!=nil { if *req.Active {p["status"]="active"} else {p["status"]="non active"} }
 var d map[string]any
 if err:=c.do(ctx,c.priceListEndpoint,p,&d);err!=nil{return nil,err}
 data:=obj(d,"data")
 if rc:=str(data,"rc"); rc!="" {
  mapped,err:=mapResponseCode(rc)
  if err!=nil { return nil, err }
  if mapped!=provider.StatusSuccess { return nil, fmt.Errorf("IAK pricelist response code %q is %q", strings.TrimSpace(rc), mapped) }
 } else {
  return nil, errors.New("IAK pricelist response is missing data.rc")
 }
 message:=strings.TrimSpace(str(data,"message"))
 if message=="" { return nil, errors.New("IAK pricelist response is missing data.message") }
 if _,ok:=data["pricelist"]; !ok { return nil, iakResponseError(d, "pricelist") }
 list,ok:=data["pricelist"].([]any); if !ok { return nil, iakResponseError(d, "pricelist") }; out:=make([]provider.Product,0,len(list))
 for _,v:=range list {
  x,ok:=v.(map[string]any); if !ok { return nil, errors.New("IAK response contains invalid pricelist item") }
  for _,field:=range []string{"product_code","product_description","product_details","product_nominal","product_type","active_period","status","icon_url","product_category"} {
   if strings.TrimSpace(str(x,field))=="" { return nil, fmt.Errorf("IAK pricelist item is missing %s",field) }
  }
  if _,ok:=requiredNum(x,"product_price"); !ok { return nil, errors.New("IAK pricelist item has missing or invalid product_price") }
  code,name:=strings.TrimSpace(str(x,"product_code")),strings.TrimSpace(str(x,"product_description"))
  cat:=strings.TrimSpace(str(x,"product_category"))
  rawStatus:=strings.TrimSpace(str(x,"status"))
  if !strings.EqualFold(rawStatus,"active") && !strings.EqualFold(rawStatus,"non active") { return nil, fmt.Errorf("IAK pricelist item has invalid status %q",rawStatus) }
  active:=strings.EqualFold(rawStatus,"active")
  if req.Category!=""&&!strings.EqualFold(cat,strings.TrimSpace(req.Category)){continue}
  if req.Active!=nil&&active!=*req.Active{continue}
  out=append(out,provider.Product{Code:code,Name:name})
 }
 return out,nil
}

func (c *Client) Inquiry(ctx context.Context, req provider.InquiryRequest)(provider.InquiryResult,error) {
 if !strings.EqualFold(strings.TrimSpace(req.ProductCode),"pln"){return provider.InquiryResult{},provider.ErrUnsupportedOperation}
 if req.CustomerNo==""{return provider.InquiryResult{},errors.New("customer number is required for IAK PLN inquiry")}
 var d map[string]any
 if err:=c.do(ctx,c.inquiryPLNEndpoint,map[string]string{"username":c.username,"customer_id":req.CustomerNo,"sign":c.sig(req.CustomerNo)},&d);err!=nil{return provider.InquiryResult{},err}
 x:=obj(d,"data")
 rawStatus:=strings.TrimSpace(str(x,"status"))
 if rawStatus!="1" && rawStatus!="2" { return provider.InquiryResult{}, fmt.Errorf("IAK inquiry response has invalid data.status %q", rawStatus) }
 customerID:=strings.TrimSpace(str(x,"customer_id"))
 if customerID=="" { return provider.InquiryResult{}, errors.New("IAK inquiry response is missing data.customer_id") }
 if customerID!=strings.TrimSpace(req.CustomerNo) { return provider.InquiryResult{}, errors.New("IAK inquiry response customer ID mismatch") }
 for _, field := range []string{"meter_no","subscriber_id","name","segment_power"} {
  if strings.TrimSpace(str(x,field))=="" { return provider.InquiryResult{}, fmt.Errorf("IAK inquiry response is missing data.%s", field) }
 }
 rc:=strings.TrimSpace(str(x,"rc"))
 if rc=="" { return provider.InquiryResult{}, errors.New("IAK inquiry response is missing data.rc") }
 mapped, mapErr:=mapResponseCode(rc)
 if mapErr!=nil { return provider.InquiryResult{}, mapErr }
 status,_:=transactionStatus(rawStatus)
 if mapped!=status { return provider.InquiryResult{}, fmt.Errorf("IAK inquiry response status %q conflicts with rc %q", status, rc) }
 message:=strings.TrimSpace(str(x,"message"))
 if message=="" { return provider.InquiryResult{}, errors.New("IAK inquiry response is missing data.message") }
 return provider.InquiryResult{Status:status,ProviderCode:rc,Message:message},nil
}

func (c *Client) Purchase(ctx context.Context, req provider.PurchaseRequest)(provider.PurchaseResult,error) {
 if req.ProductCode==""||req.CustomerNo==""||req.ReferenceID==""{return provider.PurchaseResult{},errors.New("product code, customer number, and reference ID are required")}
 var d map[string]any
 p:=map[string]string{"username":c.username,"ref_id":req.ReferenceID,"customer_id":req.CustomerNo,"product_code":req.ProductCode,"sign":c.sig(req.ReferenceID)}
 if err:=c.do(ctx,c.topUpEndpoint,p,&d);err!=nil{
  var httpErr *iakHTTPStatusError
  if errors.As(err,&httpErr) && httpErr.StatusCode != http.StatusBadRequest {
   return provider.PurchaseResult{ReferenceID:req.ReferenceID,CustomerNo:req.CustomerNo,ProductCode:req.ProductCode,Status:provider.StatusPending,Message:fmt.Sprintf("IAK HTTP status %d",httpErr.StatusCode)},nil
  }
  return provider.PurchaseResult{},err
 }
 result, err := purchase(d)
 if err != nil { return provider.PurchaseResult{}, err }
 if result.ReferenceID != req.ReferenceID { return provider.PurchaseResult{}, errors.New("IAK purchase response reference ID mismatch") }
 if result.CustomerNo != req.CustomerNo { return provider.PurchaseResult{}, errors.New("IAK purchase response customer ID mismatch") }
 if result.ProductCode != req.ProductCode { return provider.PurchaseResult{}, errors.New("IAK purchase response product code mismatch") }
 return result,nil
}

func (c *Client) GetStatus(ctx context.Context, req provider.StatusRequest)(provider.PurchaseStatus,error) {
 if req.ReferenceID==""{return provider.PurchaseStatus{},errors.New("reference ID is required")}
 var d map[string]any
 if err:=c.do(ctx,c.statusEndpoint,map[string]string{"username":c.username,"ref_id":req.ReferenceID,"sign":c.sig(req.ReferenceID)},&d);err!=nil{
  var httpErr *iakHTTPStatusError
  if errors.As(err,&httpErr) && httpErr.StatusCode != http.StatusBadRequest {
   return provider.PurchaseStatus{ReferenceID:req.ReferenceID,CustomerNo:req.CustomerNo,ProductCode:req.ProductCode,Status:provider.StatusPending,Message:fmt.Sprintf("IAK HTTP status %d",httpErr.StatusCode)},nil
  }
  return provider.PurchaseStatus{},err
 }
 x:=obj(d,"data")
 result, err := purchaseStatus(x)
 if err != nil { return provider.PurchaseStatus{}, err }
 if result.ReferenceID != req.ReferenceID { return provider.PurchaseStatus{}, errors.New("IAK status response reference ID mismatch") }
 if req.CustomerNo != "" && result.CustomerNo != req.CustomerNo { return provider.PurchaseStatus{}, errors.New("IAK status response customer ID mismatch") }
 if req.ProductCode != "" && result.ProductCode != req.ProductCode { return provider.PurchaseStatus{}, errors.New("IAK status response product code mismatch") }
 return result,nil
}

func (c *Client) GetBalance(ctx context.Context)(int64,error) {
 var d map[string]any
 if err:=c.do(ctx,c.balanceEndpoint,map[string]string{"username":c.username,"sign":c.sig("bl")},&d);err!=nil{return 0,err}; x:=obj(d,"data"); raw,ok:=x["balance"]; if !ok{return 0,errors.New("IAK balance response is missing data.balance")}; switch v:=raw.(type){case float64:
 if math.Trunc(v)!=v || v <= -math.Exp2(63) || v >= math.Exp2(63) { return 0,fmt.Errorf("invalid IAK balance: value outside int64 range or non-integer %v",v) }
 return int64(v),nil;case string:n,err:=strconv.ParseInt(strings.TrimSpace(v),10,64);if err!=nil{return 0,fmt.Errorf("invalid IAK balance: %w",err)};return n,nil;default:return 0,fmt.Errorf("invalid IAK balance type %T",raw)}
}

func (c *Client) HandleWebhook(_ context.Context, req provider.WebhookRequest)(provider.WebhookEvent,error) {
 var p map[string]any
 if err:=json.Unmarshal(req.Body,&p);err!=nil{return provider.WebhookEvent{},fmt.Errorf("decode IAK webhook: %w",err)}
 payload:=p
 if rawData, ok := p["data"]; ok {
  data, ok := rawData.(map[string]any)
  if !ok { return provider.WebhookEvent{}, errors.New("IAK webhook response has invalid data envelope") }
  payload=data
 }

 ref:=strings.TrimSpace(str(payload,"ref_id"))
 customerV2:=strings.TrimSpace(str(payload,"customer_id"))
 customerV1:=strings.TrimSpace(str(payload,"hp"))
 if customerV2!="" && customerV1!="" && customerV2!=customerV1 {
  return provider.WebhookEvent{}, errors.New("IAK webhook response has conflicting customer ID fields")
 }
 customerNo:=customerV2
 if customerNo=="" { customerNo=customerV1 }
 productV2:=strings.TrimSpace(str(payload,"product_code"))
 productV1:=strings.TrimSpace(str(payload,"code"))
 if productV2!="" && productV1!="" && productV2!=productV1 {
  return provider.WebhookEvent{}, errors.New("IAK webhook response has conflicting product code fields")
 }
 productCode:=productV2
 if productCode=="" { productCode=productV1 }
 rc:=strings.TrimSpace(str(payload,"rc"))
 bodySign:=strings.TrimSpace(str(payload,"sign"))
 if ref == "" || customerNo == "" || productCode == "" {
  return provider.WebhookEvent{}, errors.New("IAK webhook response is missing transaction identity")
 }
 if rc == "" { return provider.WebhookEvent{}, errors.New("IAK webhook response is missing rc") }
 if bodySign == "" {
  return provider.WebhookEvent{}, errors.New("IAK webhook response is missing sign")
 }
 if req.SignatureSecret!="" {
  want:=signature(req.SignatureSecret,c.username,ref)
  if subtle.ConstantTimeCompare([]byte(bodySign),[]byte(want))!=1{return provider.WebhookEvent{},errors.New("invalid IAK webhook signature")}
 }

 rawStatus:=fmt.Sprint(payload["status"])
 status, statusOK:=transactionStatus(payload["status"])
 if !statusOK || (status != provider.StatusSuccess && status != provider.StatusFailed) {
  return provider.WebhookEvent{}, fmt.Errorf("IAK webhook response has invalid callback status %q", strings.TrimSpace(rawStatus))
 }
 mapped, mapErr:=mapResponseCode(rc)
 if mapErr!=nil { return provider.WebhookEvent{}, mapErr }
 if mapped!=status {
  return provider.WebhookEvent{}, fmt.Errorf("IAK webhook response status %q conflicts with rc %q", status, rc)
 }

 message:=strings.TrimSpace(str(payload,"message"))
 price,priceOK:=requiredInt64Num(payload,"price")
 _,balanceOK:=requiredNum(payload,"balance")
 _,trIDOK:=requiredInt64Num(payload,"tr_id")
 if message=="" { return provider.WebhookEvent{}, errors.New("IAK webhook response is missing message") }
 if !priceOK { return provider.WebhookEvent{}, errors.New("IAK webhook response is missing or invalid price") }
 if !balanceOK { return provider.WebhookEvent{}, errors.New("IAK webhook response is missing or invalid balance") }
 if !trIDOK { return provider.WebhookEvent{}, errors.New("IAK webhook response is missing or invalid tr_id") }
 if status != provider.StatusSuccess && strings.TrimSpace(str(payload,"sn")) != "" { return provider.WebhookEvent{}, errors.New("IAK webhook response has serial number for non-success status") }

 return provider.WebhookEvent{ReferenceID:ref,CustomerNo:customerNo,ProductCode:productCode,Status:status,ProviderCode:rc,Message:message,SerialNumber:str(payload,"sn"),Price:price},nil
}

func (c *Client) do(ctx context.Context, endpoint string, payload any, out *map[string]any) error {
 b,e:=json.Marshal(payload);if e!=nil{return fmt.Errorf("encode IAK request: %w",e)}
 r,e:=http.NewRequestWithContext(ctx,http.MethodPost,endpoint,strings.NewReader(string(b)));if e!=nil{return fmt.Errorf("create IAK request: %w",e)};r.Header.Set("Content-Type","application/json")
 resp,e:=c.httpClient.Do(r);if e!=nil{return fmt.Errorf("IAK request failed: %w",e)};defer resp.Body.Close();body,e:=io.ReadAll(resp.Body);if e!=nil{return fmt.Errorf("read IAK response: %w",e)}
 if resp.StatusCode<200||resp.StatusCode>=300{
  if resp.StatusCode==http.StatusBadRequest {
    var errorResponse struct{ ErrorDetails any `json:"error_details"` }
    if json.Unmarshal(body,&errorResponse)==nil && errorResponse.ErrorDetails!=nil {
      return fmt.Errorf("IAK HTTP status 400: error_details=%v",errorResponse.ErrorDetails)
    }
  }
  if resp.StatusCode != http.StatusBadRequest {
   return &iakHTTPStatusError{StatusCode:resp.StatusCode, Body:string(body)}
  }
  return fmt.Errorf("IAK HTTP status %d: %s",resp.StatusCode,strings.TrimSpace(string(body)))
}
if e=json.Unmarshal(body,out);e!=nil{return fmt.Errorf("decode IAK response: %w",e)};return nil
}
func (c *Client) sig(add string)string{return signature(c.apiKey,c.username,add)}
func signature(secret,user,add string)string{s:=md5.Sum([]byte(user+secret+add));return hex.EncodeToString(s[:])}
func obj(m map[string]any,k string)map[string]any{x,_:=m[k].(map[string]any);return x}
func str(m map[string]any,k string)string{x,_:=m[k].(string);return x}
func num(m map[string]any,k string)float64{n,_:=requiredNum(m,k);return n}
func requiredNum(m map[string]any,k string)(float64,bool){x,ok:=m[k];if !ok{return 0,false};switch v:=x.(type){case float64:if math.IsNaN(v)||math.IsInf(v,0){return 0,false};return v,true;case string:n,err:=strconv.ParseFloat(strings.TrimSpace(v),64);if err!=nil||math.IsNaN(n)||math.IsInf(n,0){return 0,false};return n,true};return 0,false}
func requiredIntegerNum(m map[string]any,k string)(float64,bool){n,ok:=requiredNum(m,k);if !ok||math.Trunc(n)!=n{return 0,false};return n,true}
func requiredInt64Num(m map[string]any,k string)(int64,bool){x,ok:=m[k];if !ok{return 0,false};if s,ok:=x.(string);ok{n,err:=strconv.ParseInt(strings.TrimSpace(s),10,64);return n,err==nil};n,ok:=requiredIntegerNum(m,k);if !ok||n <= -math.Exp2(63) || n >= math.Exp2(63){return 0,false};return int64(n),true}
func status(n float64)provider.TransactionStatus{switch int(n){case 1:return provider.StatusSuccess;case 0:return provider.StatusPending;case 2:return provider.StatusFailed;default:return provider.TransactionStatus(strconv.Itoa(int(n)))}}
func transactionStatus(v any)(provider.TransactionStatus,bool){switch x:=v.(type){case float64:if math.Trunc(x)!=x{return "",false};switch int(x){case 0:return provider.StatusPending,true;case 1:return provider.StatusSuccess,true;case 2:return provider.StatusFailed,true};case string:switch strings.TrimSpace(x){case "0":return provider.StatusPending,true;case "1":return provider.StatusSuccess,true;case "2":return provider.StatusFailed,true}};return "",false}
func mapResponseCode(rc string) (provider.TransactionStatus, error) {
	switch strings.TrimSpace(rc) {
	case "00":
		return provider.StatusSuccess, nil
	case "39", "201":
		return provider.StatusPending, nil
	case "06", "07", "10", "12", "13", "14", "16", "17", "18", "19", "20", "21", "102", "106", "107", "110", "117", "121", "131", "132", "141", "142", "202", "203", "204", "205", "206", "207", "301":
		return provider.StatusFailed, nil
	default:
		return "", fmt.Errorf("unknown IAK response code %q", strings.TrimSpace(rc))
	}
}

func mapResponseStatus(rawStatus, rc string) (provider.TransactionStatus, error) {
	if strings.TrimSpace(rc) != "" {
		return mapResponseCode(rc)
	}
	st, ok := transactionStatus(rawStatus)
	if !ok {
		return "", fmt.Errorf("unknown IAK transaction status %q", strings.TrimSpace(rawStatus))
	}
	return st, nil
}

func mapInquiry(s string)provider.TransactionStatus{switch strings.TrimSpace(s){case "1":return provider.StatusSuccess;case "2":return provider.StatusFailed;default:return provider.TransactionStatus("")}}
func iakResponseError(d map[string]any, field string) error { x:=obj(d,"data"); if msg:=str(d,"message"); msg!="" { return fmt.Errorf("IAK response missing data.%s: %s",field,msg) }; if msg:=str(x,"message"); msg!="" { return fmt.Errorf("IAK response missing data.%s: %s",field,msg) }; return fmt.Errorf("IAK response missing data.%s",field) }
func validateTransactionResponse(x map[string]any, operation string) (provider.TransactionStatus, error) {
 if len(x)==0 { return "", fmt.Errorf("IAK %s response is missing data", operation) }
 rawStatus, statusOK := transactionStatus(x["status"])
 if !statusOK { return "", fmt.Errorf("IAK %s response has unknown status", operation) }
 rc := strings.TrimSpace(str(x,"rc"))
 if rc=="" { return "", fmt.Errorf("IAK %s response is missing rc", operation) }
 mapped, err := mapResponseCode(rc)
 if err != nil { return "", err }
 if mapped != rawStatus { return "", fmt.Errorf("IAK %s response status %q conflicts with rc %q", operation, rawStatus, rc) }
 if strings.TrimSpace(str(x,"message"))=="" { return "", fmt.Errorf("IAK %s response is missing message", operation) }
 if _, ok := requiredInt64Num(x,"price"); !ok { return "", fmt.Errorf("IAK %s response is missing or invalid price", operation) }
 if _, ok := requiredNum(x,"balance"); !ok { return "", fmt.Errorf("IAK %s response is missing or invalid balance", operation) }
 if _, ok := requiredInt64Num(x,"tr_id"); !ok { return "", fmt.Errorf("IAK %s response is missing or invalid tr_id", operation) }
 if mapped != provider.StatusSuccess && strings.TrimSpace(str(x,"sn")) != "" { return "", fmt.Errorf("IAK %s response has serial number for non-success status", operation) }
 return mapped, nil
}

func purchase(d map[string]any)(provider.PurchaseResult,error){
 x:=obj(d,"data")
 st,err:=validateTransactionResponse(x,"purchase")
 if err!=nil{return provider.PurchaseResult{},err}
 ref,customer,product:=str(x,"ref_id"),str(x,"customer_id"),str(x,"product_code")
 if ref==""||customer==""||product==""{return provider.PurchaseResult{},errors.New("IAK purchase response is missing transaction identity")}
 price,_:=requiredInt64Num(x,"price")
 return provider.PurchaseResult{ReferenceID:ref,CustomerNo:customer,ProductCode:product,Status:st,ProviderCode:str(x,"rc"),Message:strings.TrimSpace(str(x,"message")),Price:int64(price)},nil
}

func purchaseStatus(x map[string]any)(provider.PurchaseStatus,error){
 st,err:=validateTransactionResponse(x,"status")
 if err!=nil{return provider.PurchaseStatus{},err}
 ref,customer,product:=str(x,"ref_id"),str(x,"customer_id"),str(x,"product_code")
 if ref==""||customer==""||product==""{return provider.PurchaseStatus{},errors.New("IAK status response is missing transaction identity")}
 price,_:=requiredInt64Num(x,"price")
 return provider.PurchaseStatus{ReferenceID:ref,CustomerNo:customer,ProductCode:product,Status:st,ProviderCode:str(x,"rc"),Message:strings.TrimSpace(str(x,"message")),SerialNumber:str(x,"sn"),Price:price},nil
}
