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
 data:=obj(d,"data"); list,_:=data["pricelist"].([]any); out:=make([]provider.Product,0,len(list))
 for _,v:=range list { x,_:=v.(map[string]any); cat:=str(x,"product_category"); active:=strings.EqualFold(str(x,"status"),"active"); if req.Category!=""&&!strings.EqualFold(strings.TrimSpace(cat),strings.TrimSpace(req.Category)){continue}; if req.Active!=nil&&active!=*req.Active{continue}; out=append(out,provider.Product{Code:str(x,"product_code"),Name:str(x,"product_description")}) }
 return out,nil
}

func (c *Client) Inquiry(ctx context.Context, req provider.InquiryRequest)(provider.InquiryResult,error) {
 if !strings.EqualFold(strings.TrimSpace(req.ProductCode),"pln"){return provider.InquiryResult{},provider.ErrUnsupportedOperation}
 if req.CustomerNo==""{return provider.InquiryResult{},errors.New("customer number is required for IAK PLN inquiry")}
 var d map[string]any
 if err:=c.do(ctx,c.inquiryPLNEndpoint,map[string]string{"username":c.username,"customer_id":req.CustomerNo,"sign":c.sig(req.CustomerNo)},&d);err!=nil{return provider.InquiryResult{},err}
 x:=obj(d,"data"); return provider.InquiryResult{Status:mapInquiry(str(x,"status")),ProviderCode:str(x,"rc"),Message:str(x,"message")},nil
}

func (c *Client) Purchase(ctx context.Context, req provider.PurchaseRequest)(provider.PurchaseResult,error) {
 if req.ProductCode==""||req.CustomerNo==""||req.ReferenceID==""{return provider.PurchaseResult{},errors.New("product code, customer number, and reference ID are required")}
 var d map[string]any
 p:=map[string]string{"username":c.username,"ref_id":req.ReferenceID,"customer_id":req.CustomerNo,"product_code":req.ProductCode,"sign":c.sig(req.ReferenceID)}
 if err:=c.do(ctx,c.topUpEndpoint,p,&d);err!=nil{return provider.PurchaseResult{},err}; return purchase(d),nil
}

func (c *Client) GetStatus(ctx context.Context, req provider.StatusRequest)(provider.PurchaseStatus,error) {
 if req.ReferenceID==""{return provider.PurchaseStatus{},errors.New("reference ID is required")}
 var d map[string]any
 if err:=c.do(ctx,c.statusEndpoint,map[string]string{"username":c.username,"ref_id":req.ReferenceID,"sign":c.sig(req.ReferenceID)},&d);err!=nil{return provider.PurchaseStatus{},err}
 x:=obj(d,"data"); return provider.PurchaseStatus{ReferenceID:str(x,"ref_id"),CustomerNo:str(x,"customer_id"),ProductCode:str(x,"product_code"),Status:status(num(x,"status")),ProviderCode:str(x,"rc"),Message:str(x,"message"),SerialNumber:str(x,"sn"),Price:int64(num(x,"price"))},nil
}

func (c *Client) GetBalance(ctx context.Context)(int64,error) {
 var d map[string]any
 if err:=c.do(ctx,c.balanceEndpoint,map[string]string{"username":c.username,"sign":c.sig("bl")},&d);err!=nil{return 0,err}; return int64(num(obj(d,"data"),"balance")),nil
}

func (c *Client) HandleWebhook(_ context.Context, req provider.WebhookRequest)(provider.WebhookEvent,error) {
 var p map[string]any; if err:=json.Unmarshal(req.Body,&p);err!=nil{return provider.WebhookEvent{},fmt.Errorf("decode IAK webhook: %w",err)}
 ref:=str(p,"ref_id"); if req.SignatureSecret!="" { got:=strings.TrimSpace(req.Signature); if got==""{got=str(p,"sign")}; want:=signature(req.SignatureSecret,c.username,ref); if subtle.ConstantTimeCompare([]byte(got),[]byte(want))!=1{return provider.WebhookEvent{},errors.New("invalid IAK webhook signature")} }
 return provider.WebhookEvent{ReferenceID:ref,CustomerNo:str(p,"hp"),ProductCode:str(p,"code"),Status:status(num(p,"status")),ProviderCode:str(p,"rc"),Message:str(p,"message"),SerialNumber:str(p,"sn"),Price:int64(num(p,"price"))},nil
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
func num(m map[string]any,k string)float64{switch x:=m[k].(type){case float64:return x;case string:n,_:=strconv.ParseFloat(x,64);return n};return 0}
func status(n float64)provider.TransactionStatus{switch int(n){case 1:return provider.StatusSuccess;case 0:return provider.StatusPending;case 2:return provider.StatusFailed;default:return provider.TransactionStatus(strconv.Itoa(int(n)))}}
func mapInquiry(s string)provider.TransactionStatus{switch strings.TrimSpace(s){case "1":return provider.StatusSuccess;case "2":return provider.StatusFailed;default:return provider.TransactionStatus(s)}}
func purchase(d map[string]any)provider.PurchaseResult{x:=obj(d,"data");return provider.PurchaseResult{ReferenceID:str(x,"ref_id"),CustomerNo:str(x,"customer_id"),ProductCode:str(x,"product_code"),Status:status(num(x,"status")),ProviderCode:str(x,"rc"),Message:str(x,"message"),SerialNumber:str(x,"sn"),Price:int64(num(x,"price"))}}
