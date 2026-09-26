package xpsindonesia

import (
 "context"
 "crypto/subtle"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "net/http"
 "net/url"
 "strconv"
 "strings"

 "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/config"
 provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

type Client struct {
 id, key, api string
 saldoEndpoint, hargaEndpoint, daftarHargaEndpoint, orderEndpoint string
 httpClient *http.Client
}

func New(cfg config.XPSindonesiaConfig, httpClient *http.Client) (*Client, error) {
 if cfg.ID=="" || cfg.Key=="" || cfg.API=="" { return nil, errors.New("XP SINDONESIA id, key, and api are required") }
 if cfg.SaldoEndpoint=="" { cfg.SaldoEndpoint="https://xp.sindonesia.net/api/saldo.php" }
 if cfg.HargaEndpoint=="" { cfg.HargaEndpoint="https://xp.sindonesia.net/api/harga.php" }
 if cfg.DaftarHargaEndpoint=="" { cfg.DaftarHargaEndpoint="https://xp.sindonesia.net/api/daftar_harga.php" }
 if cfg.OrderEndpoint=="" { cfg.OrderEndpoint="https://xp.sindonesia.net/api/order.php" }
 if httpClient==nil { httpClient=http.DefaultClient }
 return &Client{id:cfg.ID,key:cfg.Key,api:cfg.API,saldoEndpoint:cfg.SaldoEndpoint,hargaEndpoint:cfg.HargaEndpoint,daftarHargaEndpoint:cfg.DaftarHargaEndpoint,orderEndpoint:cfg.OrderEndpoint,httpClient:httpClient},nil
}

type orderResponse struct { Success string; Error string; Status string; Trx string; Kode string; Isi string; Harga string; SN string }

func (c *Client) Purchase(ctx context.Context, req provider.PurchaseRequest) (provider.PurchaseResult,error) {
 if req.ProductCode=="" || req.CustomerNo=="" || req.ReferenceID=="" { return provider.PurchaseResult{},errors.New("product code, customer number, and reference ID are required") }
 values:=url.Values{}; values.Set("id",c.id);values.Set("key",c.key);values.Set("api",c.api);values.Set("url","");values.Set("trx",req.ReferenceID);values.Set("kod",req.ProductCode);values.Set("isi",req.CustomerNo);values.Set("sms","")
 var out orderResponse
 if err:=c.postJSON(ctx,c.orderEndpoint,values,&out);err!=nil{return provider.PurchaseResult{},err}
 if out.Trx==""||out.Kode==""||out.Isi==""{return provider.PurchaseResult{},errors.New("XP order response is missing transaction identity")}
 if out.Trx!=req.ReferenceID||out.Kode!=req.ProductCode||out.Isi!=req.CustomerNo{return provider.PurchaseResult{},errors.New("XP order response transaction identity mismatch")}
 price,err:=parseInt(out.Harga);if err!=nil{return provider.PurchaseResult{},fmt.Errorf("XP order response has invalid price: %w",err)}
 status:=mapStatus(out.Status);if out.Success=="0"&&status==""{status=provider.StatusFailed};if status==""{return provider.PurchaseResult{},fmt.Errorf("XP order response has unknown status %q",out.Status)}
 message:=out.Status;if out.Error!=""{message=out.Error+": "+out.Status}
 return provider.PurchaseResult{ReferenceID:out.Trx,CustomerNo:out.Isi,ProductCode:out.Kode,Status:status,ProviderCode:out.Error,Message:message,SerialNumber:out.SN,Price:price},nil
}

func (c *Client) GetBalance(ctx context.Context)(int64,error){
 values:=url.Values{"id":{c.id},"key":{c.key},"api":{c.api}}
 var out struct{Success string;Saldo json.RawMessage;Error string}
 if err:=c.postJSON(ctx,c.saldoEndpoint,values,&out);err!=nil{return 0,err}
 if out.Success!="1"{return 0,fmt.Errorf("XP balance request failed: %s",out.Error)}
 if len(out.Saldo)==0{return 0,errors.New("XP balance response is missing saldo")}
 var s string;if err:=json.Unmarshal(out.Saldo,&s);err==nil{return parseInt(s)}
 var n int64;if err:=json.Unmarshal(out.Saldo,&n);err!=nil{return 0,fmt.Errorf("XP balance response has invalid saldo: %w",err)};return n,nil
}

func (c *Client) GetProducts(context.Context,provider.ProductRequest)([]provider.Product,error){return nil,provider.ErrUnsupportedOperation}
func (c *Client) Inquiry(context.Context,provider.InquiryRequest)(provider.InquiryResult,error){return provider.InquiryResult{},provider.ErrUnsupportedOperation}
func (c *Client) GetStatus(context.Context,provider.StatusRequest)(provider.PurchaseStatus,error){return provider.PurchaseStatus{},provider.ErrUnsupportedOperation}

func (c *Client) HandleWebhook(_ context.Context,req provider.WebhookRequest)(provider.WebhookEvent,error){
 q,err:=url.ParseQuery(string(req.Body));if err!=nil{return provider.WebhookEvent{},fmt.Errorf("decode XP callback: %w",err)}
 if req.SignatureSecret!=""&&subtle.ConstantTimeCompare([]byte(q.Get("key")),[]byte(req.SignatureSecret))!=1{return provider.WebhookEvent{},errors.New("invalid XP callback key")}
 if q.Get("id")==""||q.Get("trx")==""||q.Get("kod")==""||q.Get("isi")==""{return provider.WebhookEvent{},errors.New("XP callback is missing transaction identity")}
 st:=mapStatus(q.Get("status"));if st==""{return provider.WebhookEvent{},fmt.Errorf("XP callback has unknown status %q",q.Get("status"))}
 return provider.WebhookEvent{ReferenceID:q.Get("trx"),CustomerNo:q.Get("isi"),ProductCode:q.Get("kod"),Status:st,SerialNumber:q.Get("sn")},nil
}

func (c *Client) postJSON(ctx context.Context,endpoint string,values url.Values,out any)error{
 req,err:=http.NewRequestWithContext(ctx,http.MethodPost,endpoint,strings.NewReader(values.Encode()));if err!=nil{return fmt.Errorf("create XP request: %w",err)}
 req.Header.Set("Content-Type","application/x-www-form-urlencoded");resp,err:=c.httpClient.Do(req);if err!=nil{return fmt.Errorf("XP request failed: %w",err)};defer resp.Body.Close()
 body,err:=io.ReadAll(resp.Body);if err!=nil{return fmt.Errorf("read XP response: %w",err)}
 if resp.StatusCode<200||resp.StatusCode>=300{return fmt.Errorf("XP HTTP status %d: %s",resp.StatusCode,strings.TrimSpace(string(body)))}
 if err:=json.Unmarshal(body,out);err!=nil{return fmt.Errorf("decode XP response: %w",err)};return nil
}
func parseInt(s string)(int64,error){return strconv.ParseInt(strings.TrimSpace(s),10,64)}
func mapStatus(s string)provider.TransactionStatus{switch strings.ToLower(strings.TrimSpace(s)){case "sukses":return provider.StatusSuccess;case "gagal":return provider.StatusFailed;case "proses","lambat":return provider.StatusPending;default:return ""}}
