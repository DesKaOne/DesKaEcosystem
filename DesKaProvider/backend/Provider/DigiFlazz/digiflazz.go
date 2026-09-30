package digiflazz

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"net/http"
	"strings"
	"time"

	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/config"
	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

var (
	ErrInvalidWebhookSignature = errors.New("invalid DigiFlazz webhook signature")
	ErrUnknownTransactionStatus = errors.New("unknown DigiFlazz transaction status")
	ErrUnknownResponseCode = errors.New("unknown DigiFlazz response code")
)

const (
	defaultEndpoint          = "https://api.digiflazz.com/v1/transaction"
	defaultBalanceEndpoint   = "https://api.digiflazz.com/v1/cek-saldo"
	defaultPriceListEndpoint = "https://api.digiflazz.com/v1/price-list"
)

type Client struct {
	username       string
	apiKey         string
	endpoint       string
	balanceEndpoint   string
	priceListEndpoint string
	httpClient        *http.Client
}

func New(cfg config.DigiFlazzConfig, httpClient *http.Client) (*Client, error) {
	if cfg.Username == "" || cfg.APIKey == "" {
		return nil, errors.New("DigiFlazz username and API key are required")
	}
	if cfg.BaseURL != "" {
		base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
		if cfg.Endpoint == "" { cfg.Endpoint = base + "/v1/transaction" }
		if cfg.BalanceEndpoint == "" { cfg.BalanceEndpoint = base + "/v1/cek-saldo" }
		if cfg.PriceListEndpoint == "" { cfg.PriceListEndpoint = base + "/v1/price-list" }
	}
	if cfg.Endpoint == "" { cfg.Endpoint = defaultEndpoint }
	if cfg.BalanceEndpoint == "" { cfg.BalanceEndpoint = defaultBalanceEndpoint }
	if cfg.PriceListEndpoint == "" { cfg.PriceListEndpoint = defaultPriceListEndpoint }
	if cfg.HTTPTimeout <= 0 { cfg.HTTPTimeout = 15 * time.Second }
	if httpClient == nil { httpClient = &http.Client{Timeout: cfg.HTTPTimeout} } else if httpClient.Timeout <= 0 { copy := *httpClient; copy.Timeout = cfg.HTTPTimeout; httpClient = &copy }
	return &Client{username: cfg.Username, apiKey: cfg.APIKey, endpoint: cfg.Endpoint, balanceEndpoint: cfg.BalanceEndpoint, priceListEndpoint: cfg.PriceListEndpoint, httpClient: httpClient}, nil
}

type transactionRequest struct {
	Username string `json:"username"`
	BuyerSKUCode string `json:"buyer_sku_code"`
	CustomerNo string `json:"customer_no"`
	ReferenceID string `json:"ref_id"`
	Sign string `json:"sign"`
	Testing bool `json:"testing,omitempty"`
}

type transactionResponse struct {
	Data struct {
		ReferenceID string `json:"ref_id"`
		CustomerNo string `json:"customer_no"`
		BuyerSKUCode string `json:"buyer_sku_code"`
		Message string `json:"message"`
		Status string `json:"status"`
		RC string `json:"rc"`
		SN string `json:"sn"`
		BuyerLastSaldo float64 `json:"buyer_last_saldo"`
		Price *int64 `json:"price"`
	} `json:"data"`
}

type balanceRequest struct {
	Cmd string `json:"cmd"`
	Username string `json:"username"`
	Sign string `json:"sign"`
}

type balanceResponse struct {
	Data struct { Deposit json.Number `json:"deposit"` } `json:"data"`
}

func (c *Client) GetBalance(ctx context.Context) (int64, error) {
	body, err := json.Marshal(balanceRequest{Cmd: "deposit", Username: c.username, Sign: c.balanceSignature()})
	if err != nil { return 0, fmt.Errorf("encode DigiFlazz balance request: %w", err) }
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.balanceEndpoint, strings.NewReader(string(body)))
	if err != nil { return 0, fmt.Errorf("create DigiFlazz balance request: %w", err) }
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil { return 0, fmt.Errorf("DigiFlazz balance request failed: %w", err) }
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil { return 0, fmt.Errorf("read DigiFlazz balance response: %w", err) }
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("DigiFlazz balance HTTP status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var decoded balanceResponse
	decoder := json.NewDecoder(strings.NewReader(string(respBody)))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil { return 0, fmt.Errorf("decode DigiFlazz balance response: %w", err) }
	balance, err := parseIntegralBalance(decoded.Data.Deposit)
	if err != nil { return 0, err }
	return balance, nil
}

type priceListRequest struct {
	Cmd string `json:"cmd"`
	Username string `json:"username"`
	Sign string `json:"sign"`
	Category string `json:"category,omitempty"`
}

type priceListResponse struct {
	Data []struct {
		ProductName string `json:"product_name"`
		BuyerSKUCode string `json:"buyer_sku_code"`
		BuyerProductStatus bool `json:"buyer_product_status"`
	} `json:"data"`
}

func (c *Client) GetProducts(ctx context.Context, req provider.ProductRequest) ([]provider.Product, error) {
	body, err := json.Marshal(priceListRequest{Cmd: "prepaid", Username: c.username, Sign: c.priceListSignature(), Category: req.Category})
	if err != nil { return nil, fmt.Errorf("encode DigiFlazz price list request: %w", err) }
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.priceListEndpoint, strings.NewReader(string(body)))
	if err != nil { return nil, fmt.Errorf("create DigiFlazz price list request: %w", err) }
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(httpReq)
	if err != nil { return nil, fmt.Errorf("DigiFlazz price list request failed: %w", err) }
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil { return nil, fmt.Errorf("read DigiFlazz price list response: %w", err) }
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("DigiFlazz price list HTTP status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var decoded priceListResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil { return nil, fmt.Errorf("decode DigiFlazz price list response: %w", err) }
	products := make([]provider.Product, 0, len(decoded.Data))
	for _, item := range decoded.Data {
		code := strings.TrimSpace(item.BuyerSKUCode)
		name := strings.TrimSpace(item.ProductName)
		if code == "" || name == "" {
			return nil, errors.New("DigiFlazz price list contains product with missing buyer SKU code or product name")
		}
		if req.Active != nil && item.BuyerProductStatus != *req.Active { continue }
		products = append(products, provider.Product{Code: code, Name: name})
	}
	return products, nil
}
func (c *Client) Inquiry(ctx context.Context, req provider.InquiryRequest) (provider.InquiryResult, error) {
	if strings.ToLower(strings.TrimSpace(req.ProductCode)) != "pln" {
		return provider.InquiryResult{}, provider.ErrUnsupportedOperation
	}
	if req.CustomerNo == "" || req.ReferenceID == "" {
		return provider.InquiryResult{}, errors.New("customer number and reference ID are required for DigiFlazz PLN inquiry")
	}
	body, err := json.Marshal(struct {
		Commands string `json:"commands"`
		Username string `json:"username"`
		BuyerSKUCode string `json:"buyer_sku_code"`
		CustomerNo string `json:"customer_no"`
		ReferenceID string `json:"ref_id"`
		Sign string `json:"sign"`
	}{Commands: "inq-pasca", Username: c.username, BuyerSKUCode: req.ProductCode, CustomerNo: req.CustomerNo, ReferenceID: req.ReferenceID, Sign: c.signature(req.ReferenceID)})
	if err != nil { return provider.InquiryResult{}, fmt.Errorf("encode DigiFlazz PLN inquiry request: %w", err) }
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, strings.NewReader(string(body)))
	if err != nil { return provider.InquiryResult{}, fmt.Errorf("create DigiFlazz PLN inquiry request: %w", err) }
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(httpReq)
	if err != nil { return provider.InquiryResult{}, fmt.Errorf("DigiFlazz PLN inquiry request failed: %w", err) }
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil { return provider.InquiryResult{}, fmt.Errorf("read DigiFlazz PLN inquiry response: %w", err) }
	if resp.StatusCode < 200 || resp.StatusCode >= 300 { return provider.InquiryResult{}, fmt.Errorf("DigiFlazz PLN inquiry HTTP status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody))) }
	var decoded struct { Data struct { ReferenceID string `json:"ref_id"`; CustomerNo string `json:"customer_no"`; BuyerSKUCode string `json:"buyer_sku_code"`; Message string `json:"message"`; Status string `json:"status"`; RC string `json:"rc"` } `json:"data"` }
	if err := json.Unmarshal(respBody, &decoded); err != nil { return provider.InquiryResult{}, fmt.Errorf("decode DigiFlazz PLN inquiry response: %w", err) }
	if decoded.Data.ReferenceID != req.ReferenceID || decoded.Data.CustomerNo != req.CustomerNo || decoded.Data.BuyerSKUCode != req.ProductCode {
		return provider.InquiryResult{}, errors.New("DigiFlazz PLN inquiry response transaction identity mismatch")
	}
	status, err := mapResponseStatus(decoded.Data.Status, decoded.Data.RC)
	if err != nil { return provider.InquiryResult{}, err }
	if strings.TrimSpace(decoded.Data.Message) == "" { return provider.InquiryResult{}, errors.New("DigiFlazz PLN inquiry response is missing required message") }
	return provider.InquiryResult{Status: status, ProviderCode: decoded.Data.RC, Message: decoded.Data.Message}, nil
}
func (c *Client) Purchase(ctx context.Context, req provider.PurchaseRequest) (provider.PurchaseResult, error) {
	if err := validateTransactionRequest(req.ProductCode, req.CustomerNo, req.ReferenceID); err != nil { return provider.PurchaseResult{}, err }
	data, err := c.transaction(ctx, transactionRequest{Username:c.username, BuyerSKUCode:req.ProductCode, CustomerNo:req.CustomerNo, ReferenceID:req.ReferenceID, Sign:c.signature(req.ReferenceID), Testing:req.Testing})
	if err != nil { return provider.PurchaseResult{}, err }
	result, err := mapPurchaseResult(data)
	if err != nil { return provider.PurchaseResult{}, err }
	if result.ReferenceID != req.ReferenceID || result.CustomerNo != req.CustomerNo || result.ProductCode != req.ProductCode { return provider.PurchaseResult{}, errors.New("DigiFlazz purchase response transaction identity mismatch") }
	if strings.TrimSpace(result.Message) == "" { return provider.PurchaseResult{}, errors.New("DigiFlazz purchase response is missing required message") }
	return result, nil
}

func (c *Client) GetStatus(ctx context.Context, req provider.StatusRequest) (provider.PurchaseStatus, error) {
	return provider.PurchaseStatus{}, provider.ErrUnsupportedOperation
}

func (c *Client) HandleWebhook(_ context.Context, req provider.WebhookRequest) (provider.WebhookEvent, error) {
	event := strings.ToLower(strings.TrimSpace(req.Event))
	if event == "" {
		return provider.WebhookEvent{}, errors.New("DigiFlazz webhook event header is required")
	}
	if event != "create" && event != "update" {
		return provider.WebhookEvent{}, fmt.Errorf("unsupported DigiFlazz prepaid webhook event: %q", req.Event)
	}
	userAgent := strings.TrimSpace(req.UserAgent)
	if userAgent == "" {
		return provider.WebhookEvent{}, errors.New("DigiFlazz webhook user-agent header is required")
	}
	if userAgent != "Digiflazz-Hookshot" {
		return provider.WebhookEvent{}, fmt.Errorf("unsupported DigiFlazz webhook user-agent: %q", req.UserAgent)
	}
	if req.SignatureSecret != "" {
		expected := hmac.New(sha1.New, []byte(req.SignatureSecret)); _, _ = expected.Write(req.Body)
		expectedHeader := "sha1=" + hex.EncodeToString(expected.Sum(nil))
		if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(req.Signature)), []byte(expectedHeader)) != 1 { return provider.WebhookEvent{}, ErrInvalidWebhookSignature }
	}
	var payload struct { Data struct {
		ReferenceID string `json:"ref_id"`; CustomerNo string `json:"customer_no"`; BuyerSKUCode string `json:"buyer_sku_code"`; Message string `json:"message"`; Status string `json:"status"`; RC string `json:"rc"`; SN string `json:"sn"`; Price int64 `json:"price"`
	} `json:"data"` }
	if err := json.Unmarshal(req.Body, &payload); err != nil { return provider.WebhookEvent{}, fmt.Errorf("decode DigiFlazz webhook: %w", err) }
	status, err := mapResponseStatus(payload.Data.Status, payload.Data.RC)
	if err != nil { return provider.WebhookEvent{}, err }
	return provider.WebhookEvent{ReferenceID:payload.Data.ReferenceID, CustomerNo:payload.Data.CustomerNo, ProductCode:payload.Data.BuyerSKUCode, Status:status, ProviderCode:payload.Data.RC, Message:payload.Data.Message, SerialNumber:payload.Data.SN, Price:payload.Data.Price}, nil
}

func (c *Client) transaction(ctx context.Context, req transactionRequest) (transactionResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return transactionResponse{}, fmt.Errorf("encode DigiFlazz request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, strings.NewReader(string(body)))
	if err != nil {
		return transactionResponse{}, fmt.Errorf("create DigiFlazz request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return transactionResponse{}, fmt.Errorf("DigiFlazz request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return transactionResponse{}, fmt.Errorf("read DigiFlazz response: %w", err)
	}

	var decoded transactionResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return transactionResponse{}, fmt.Errorf("DigiFlazz HTTP status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
		}
		return transactionResponse{}, fmt.Errorf("decode DigiFlazz response: %w", err)
	}

	// DigiFlazz can return a structured transaction result together with a
	// non-2xx HTTP status (for example an IP allowlist rejection). Preserve the
	// provider result so callers can inspect the normalized status and RC.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if decoded.Data.ReferenceID == "" &&
			decoded.Data.CustomerNo == "" &&
			decoded.Data.BuyerSKUCode == "" &&
			decoded.Data.Message == "" &&
			decoded.Data.Status == "" &&
			decoded.Data.RC == "" {
			return transactionResponse{}, fmt.Errorf("DigiFlazz HTTP status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
		}
	}

	return decoded, nil
}


func parseIntegralBalance(value json.Number) (int64, error) {
	raw := strings.TrimSpace(value.String())
	if raw == "" { return 0, errors.New("DigiFlazz balance response is missing required deposit") }
	r := new(big.Rat)
	if _, ok := r.SetString(raw); !ok { return 0, fmt.Errorf("invalid DigiFlazz deposit value %q", raw) }
	if !r.IsInt() { return 0, fmt.Errorf("DigiFlazz deposit is not an integer value: %q", raw) }
	if !r.Num().IsInt64() { return 0, fmt.Errorf("DigiFlazz deposit is outside provider-neutral balance range: %q", raw) }
	return strconv.ParseInt(r.Num().String(), 10, 64)
}

func (c *Client) signature(refID string) string { sum := md5.Sum([]byte(c.username+c.apiKey+refID)); return hex.EncodeToString(sum[:]) }
func (c *Client) balanceSignature() string { sum := md5.Sum([]byte(c.username+c.apiKey+"depo")); return hex.EncodeToString(sum[:]) }
func (c *Client) priceListSignature() string { sum := md5.Sum([]byte(c.username+c.apiKey+"pricelist")); return hex.EncodeToString(sum[:]) }
func validateTransactionRequest(productCode, customerNo, referenceID string) error { if productCode=="" || customerNo=="" || referenceID=="" { return errors.New("product code, customer number, and reference ID are required") }; return nil }
func mapPurchaseResult(data transactionResponse) (provider.PurchaseResult, error) { status, err := mapResponseStatus(data.Data.Status, data.Data.RC); if err != nil { return provider.PurchaseResult{}, err }; if data.Data.Price == nil { return provider.PurchaseResult{}, errors.New("DigiFlazz purchase response is missing required price") }; return provider.PurchaseResult{ReferenceID:data.Data.ReferenceID, CustomerNo:data.Data.CustomerNo, ProductCode:data.Data.BuyerSKUCode, Status:status, ProviderCode:data.Data.RC, Message:data.Data.Message, SerialNumber:data.Data.SN, Price:*data.Data.Price}, nil }
func mapPurchaseStatus(data transactionResponse) (provider.PurchaseStatus, error) { status, err := mapResponseStatus(data.Data.Status, data.Data.RC); if err != nil { return provider.PurchaseStatus{}, err }; if data.Data.Price == nil { return provider.PurchaseStatus{}, errors.New("DigiFlazz status response is missing required price") }; return provider.PurchaseStatus{ReferenceID:data.Data.ReferenceID, CustomerNo:data.Data.CustomerNo, ProductCode:data.Data.BuyerSKUCode, Status:status, ProviderCode:data.Data.RC, Message:data.Data.Message, SerialNumber:data.Data.SN, Price:*data.Data.Price}, nil }
func mapResponseCode(rc string) (provider.TransactionStatus, error) {
	switch strings.TrimSpace(rc) {
	case "00":
		return provider.StatusSuccess, nil
	case "03", "99":
		return provider.StatusPending, nil
	case "01", "02", "40", "41", "42", "43", "44", "45", "47",
		"49", "50", "51", "52", "53", "54", "55", "56", "57", "58",
		"59", "60", "61", "62", "63", "64", "65", "66", "67", "68",
		"69", "70", "71", "72", "73", "74", "80", "81", "82", "83",
		"84", "85", "86", "87", "88":
		return provider.StatusFailed, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrUnknownResponseCode, strings.TrimSpace(rc))
	}
}

func mapResponseStatus(status, rc string) (provider.TransactionStatus, error) {
	rawStatus := strings.TrimSpace(status)
	rawRC := strings.TrimSpace(rc)
	if rawRC != "" {
		mappedRC, err := mapResponseCode(rawRC)
		if err != nil {
			return "", err
		}
		if rawStatus != "" {
			mappedStatus, err := mapStatus(rawStatus)
			if err != nil {
				return "", err
			}
			if mappedStatus != mappedRC {
				return "", fmt.Errorf("DigiFlazz response status %q conflicts with rc %q", rawStatus, rawRC)
			}
		}
		return mappedRC, nil
	}
	return mapStatus(rawStatus)
}

func mapStatus(status string) (provider.TransactionStatus, error) { switch strings.ToLower(strings.TrimSpace(status)) { case "sukses": return provider.StatusSuccess, nil; case "pending": return provider.StatusPending, nil; case "gagal": return provider.StatusFailed, nil; default: return "", fmt.Errorf("%w: %q", ErrUnknownTransactionStatus, strings.TrimSpace(status)) } }
