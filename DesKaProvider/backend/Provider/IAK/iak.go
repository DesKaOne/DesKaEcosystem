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
 "net/http"
 "strconv"
 "strings"

 "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/config"
 provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

type Client struct {
 username, apiKey string
 priceListEndpoint, inquiryPLNEndpoint, topUpEndpoint, statusEndpoint, balanceEndpoint string
 httpClient *http.Client
}

func New(cfg config.IAKConfig, httpClient *http.Client) (*Client,error) {
 if cfg.Username==""||cfg.APIKey=="" { return nil,errors.New("IAK username and API key are required") }
 if httpClient==nil { httpClient=http.DefaultClient }
 return &Client{cfg.Username,cfg.APIKey,cfg.PriceListEndpoint,cfg.InquiryPLNEndpoint,cfg.TopUpEndpoint,cfg.StatusEndpoint,cfg.BalanceEndpoint,httpClient},nil
}

func (c *Client) GetProducts(ctx context.Context, req provider.ProductRequest)([]provider.Product,error) {
 p:=map[string]string{"username":c.username,"sign":c.sig("pl"),"status":"all"}
 if req.Active!=nil { if *req.Active {p["status"]="active"} else {p["status"]="non active"} }
 var d map[string]any
 if err:=c.do(ctx,c.priceListEndpoint,p,&d);err!=nil{return nil,err}
 data:=obj(d,"data"); list,ok:=data["pricelist"].([]any); if !ok { return nil, iakResponseError(d, "pricelist") }; out:=make([]provider.Product,0,len(list))
 for _,v:=range list { x,ok:=v.(map[string]any); if !ok { return nil, errors.New("IAK response contains invalid pricelist item") }; code,name:=str(x,"product_code"),str(x,"product_description"); if code==""||name=="" { return nil, errors.New("IAK response contains incomplete pricelist item") }; cat:=str(x,"product_category"); active:=strings.EqualFold(str(x,"status"),"active"); if req.Category!=""&&!strings.EqualFold(strings.TrimSpace(cat),strings.TrimSpace(req.Category)){continue}; if req.Active!=nil&&active!=*req.Active{continue}; out=append(out,provider.Product{Code:code,Name:name}) }
 return out,nil
}

func (c *Client) Inquiry(ctx context.Context, req provider.InquiryRequest)(provider.InquiryResult,error) {
 if !strings.EqualFold(strings.TrimSpace(req.ProductCode),"pln"){return provider.InquiryResult{},provider.ErrUnsupportedOperation}
 if req.CustomerNo==""{return provider.InquiryResult{},errors.New("customer number is required for IAK PLN inquiry")}
 var d map[string]any
 if err:=c.do(ctx,c.inquiryPLNEndpoint,map[string]string{"username":c.username,"customer_id":req.CustomerNo,"sign":c.sig(req.CustomerNo)},&d);err!=nil{return provider.InquiryResult{},err}
 x:=obj(d,"data"); status:=mapInquiry(str(x,"status")); message:=str(x,"message"); if status=="" { return provider.InquiryResult{}, errors.New("IAK inquiry response is missing data.status") }; if message=="" { return provider.InquiryResult{}, errors.New("IAK inquiry response is missing data.message") }; return provider.InquiryResult{Status:status,ProviderCode:str(x,"rc"),Message:message},nil
}

func (c *Client) Purchase(ctx context.Context, req provider.PurchaseRequest)(provider.PurchaseResult,error) {
 if req.ProductCode==""||req.CustomerNo==""||req.ReferenceID==""{return provider.PurchaseResult{},errors.New("product code, customer number, and reference ID are required")}
 var d map[string]any
 p:=map[string]string{"username":c.username,"ref_id":req.ReferenceID,"customer_id":req.CustomerNo,"product_code":req.ProductCode,"sign":c.sig(req.ReferenceID)}
 if err:=c.do(ctx,c.topUpEndpoint,p,&d);err!=nil{return provider.PurchaseResult{},err}
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
 if err:=c.do(ctx,c.statusEndpoint,map[string]string{"username":c.username,"ref_id":req.ReferenceID,"sign":c.sig(req.ReferenceID)},&d);err!=nil{return provider.PurchaseStatus{},err}
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
 if err:=c.do(ctx,c.balanceEndpoint,map[string]string{"username":c.username,"sign":c.sig("bl")},&d);err!=nil{return 0,err}; x:=obj(d,"data"); raw,ok:=x["balance"]; if !ok{return 0,errors.New("IAK balance response is missing data.balance")}; switch v:=raw.(type){case float64:return int64(v),nil;case string:n,err:=strconv.ParseInt(strings.TrimSpace(v),10,64);if err!=nil{return 0,fmt.Errorf("invalid IAK balance: %w",err)};return n,nil;default:return 0,fmt.Errorf("invalid IAK balance type %T",raw)}
}

func (c *Client) HandleWebhook(_ context.Context, req provider.WebhookRequest)(provider.WebhookEvent,error) {
 var p map[string]any; if err:=json.Unmarshal(req.Body,&p);err!=nil{return provider.WebhookEvent{},fmt.Errorf("decode IAK webhook: %w",err)}
 ref:=str(p,"ref_id"); if req.SignatureSecret!="" { got:=strings.TrimSpace(req.Signature); if got==""{got=str(p,"sign")}; want:=signature(req.SignatureSecret,c.username,ref); if subtle.ConstantTimeCompare([]byte(got),[]byte(want))!=1{return provider.WebhookEvent{},errors.New("invalid IAK webhook signature")} }
 customerNo, productCode := str(p,"hp"), str(p,"code")
 st, ok := transactionStatus(p["status"])
 if ref == "" || customerNo == "" || productCode == "" { return provider.WebhookEvent{}, errors.New("IAK webhook response is missing transaction identity") }
 if !ok { return provider.WebhookEvent{}, errors.New("IAK webhook response has unknown status") }
 message:=str(p,"message"); price,priceOK:=requiredNum(p,"price"); if message=="" { return provider.WebhookEvent{}, errors.New("IAK webhook response is missing message") }; if !priceOK { return provider.WebhookEvent{}, errors.New("IAK webhook response is missing or invalid price") }; return provider.WebhookEvent{ReferenceID:ref,CustomerNo:customerNo,ProductCode:productCode,Status:st,ProviderCode:str(p,"rc"),Message:message,SerialNumber:str(p,"sn"),Price:int64(price)},nil
}

func (c *Client) do(ctx context.Context, endpoint string, payload any, out *map[string]any) error {
 b,e:=json.Marshal(payload);if e!=nil{return fmt.Errorf("encode IAK request: %w",e)}
 r,e:=http.NewRequestWithContext(ctx,http.MethodPost,endpoint,strings.NewReader(string(b)));if e!=nil{return fmt.Errorf("create IAK request: %w",e)};r.Header.Set("Content-Type","application/json")
 resp,e:=c.httpClient.Do(r);if e!=nil{return fmt.Errorf("IAK request failed: %w",e)};defer resp.Body.Close();body,e:=io.ReadAll(resp.Body);if e!=nil{return fmt.Errorf("read IAK response: %w",e)}
 if resp.StatusCode<200||resp.StatusCode>=300{return fmt.Errorf("IAK HTTP status %d: %s",resp.StatusCode,strings.TrimSpace(string(body)))};if e=json.Unmarshal(body,out);e!=nil{return fmt.Errorf("decode IAK response: %w",e)};return nil
}
func (c *Client) sig(add string)string{return signature(c.apiKey,c.username,add)}
func signature(secret,user,add string)string{s:=md5.Sum([]byte(user+secret+add));return hex.EncodeToString(s[:])}
func obj(m map[string]any,k string)map[string]any{x,_:=m[k].(map[string]any);return x}
func str(m map[string]any,k string)string{x,_:=m[k].(string);return x}
func num(m map[string]any,k string)float64{n,_:=requiredNum(m,k);return n}
func requiredNum(m map[string]any,k string)(float64,bool){x,ok:=m[k];if !ok{return 0,false};switch v:=x.(type){case float64:return v,true;case string:n,err:=strconv.ParseFloat(strings.TrimSpace(v),64);return n,err==nil};return 0,false}
func status(n float64)provider.TransactionStatus{switch int(n){case 1:return provider.StatusSuccess;case 0:return provider.StatusPending;case 2:return provider.StatusFailed;default:return provider.TransactionStatus(strconv.Itoa(int(n)))}}
func transactionStatus(v any)(provider.TransactionStatus,bool){switch x:=v.(type){case float64:switch int(x){case 0:return provider.StatusPending,true;case 1:return provider.StatusSuccess,true;case 2:return provider.StatusFailed,true};case string:switch strings.TrimSpace(x){case "0":return provider.StatusPending,true;case "1":return provider.StatusSuccess,true;case "2":return provider.StatusFailed,true}};return "",false}
func mapInquiry(s string)provider.TransactionStatus{switch strings.TrimSpace(s){case "1":return provider.StatusSuccess;case "2":return provider.StatusFailed;default:return provider.TransactionStatus("")}}
func iakResponseError(d map[string]any, field string) error { x:=obj(d,"data"); if msg:=str(d,"message"); msg!="" { return fmt.Errorf("IAK response missing data.%s: %s",field,msg) }; if msg:=str(x,"message"); msg!="" { return fmt.Errorf("IAK response missing data.%s: %s",field,msg) }; return fmt.Errorf("IAK response missing data.%s",field) }
func purchase(d map[string]any)(provider.PurchaseResult,error){x:=obj(d,"data");if len(x)==0{return provider.PurchaseResult{},errors.New("IAK purchase response is missing data")};ref,customer,product:=str(x,"ref_id"),str(x,"customer_id"),str(x,"product_code");st,ok:=transactionStatus(x["status"]);if ref==""||customer==""||product==""{return provider.PurchaseResult{},errors.New("IAK purchase response is missing transaction identity")};if !ok{return provider.PurchaseResult{},errors.New("IAK purchase response has unknown status")};message:=str(x,"message"); price,priceOK:=requiredNum(x,"price"); if message=="" { return provider.PurchaseResult{}, errors.New("IAK purchase response is missing message") }; if !priceOK { return provider.PurchaseResult{}, errors.New("IAK purchase response is missing or invalid price") }; return provider.PurchaseResult{ReferenceID:ref,CustomerNo:customer,ProductCode:product,Status:st,ProviderCode:str(x,"rc"),Message:message,SerialNumber:str(x,"sn"),Price:int64(price)},nil}
func purchaseStatus(x map[string]any)(provider.PurchaseStatus,error){if len(x)==0{return provider.PurchaseStatus{},errors.New("IAK status response is missing data")};ref,customer,product:=str(x,"ref_id"),str(x,"customer_id"),str(x,"product_code");st,ok:=transactionStatus(x["status"]);if ref==""||customer==""||product==""{return provider.PurchaseStatus{},errors.New("IAK status response is missing transaction identity")};if !ok{return provider.PurchaseStatus{},errors.New("IAK status response has unknown status")};message:=str(x,"message"); price,priceOK:=requiredNum(x,"price"); if message=="" { return provider.PurchaseStatus{}, errors.New("IAK status response is missing message") }; if !priceOK { return provider.PurchaseStatus{}, errors.New("IAK status response is missing or invalid price") }; return provider.PurchaseStatus{ReferenceID:ref,CustomerNo:customer,ProductCode:product,Status:st,ProviderCode:str(x,"rc"),Message:message,SerialNumber:str(x,"sn"),Price:int64(price)},nil}
