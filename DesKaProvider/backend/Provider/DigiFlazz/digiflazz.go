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
	"net/http"
	"strings"

	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/config"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

var ErrInvalidWebhookSignature = errors.New("invalid DigiFlazz webhook signature")

const defaultEndpoint = "https://api.digiflazz.com/v1/transaction"

type Client struct {
	username   string
	apiKey     string
	endpoint   string
	httpClient *http.Client
}

func New(cfg config.DigiFlazzConfig, httpClient *http.Client) (*Client, error) {
	if cfg.Username == "" || cfg.APIKey == "" {
		return nil, errors.New("DigiFlazz username and API key are required")
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = defaultEndpoint
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		username:   cfg.Username,
		apiKey:     cfg.APIKey,
		endpoint:   cfg.Endpoint,
		httpClient: httpClient,
	}, nil
}

type transactionRequest struct {
	Username     string `json:"username"`
	BuyerSKUCode string `json:"buyer_sku_code"`
	CustomerNo   string `json:"customer_no"`
	ReferenceID  string `json:"ref_id"`
	Sign         string `json:"sign"`
	Testing      bool   `json:"testing,omitempty"`
}

type transactionResponse struct {
	Data struct {
		ReferenceID    string  `json:"ref_id"`
		CustomerNo     string  `json:"customer_no"`
		BuyerSKUCode   string  `json:"buyer_sku_code"`
		Message        string  `json:"message"`
		Status         string  `json:"status"`
		RC             string  `json:"rc"`
		SN             string  `json:"sn"`
		BuyerLastSaldo float64 `json:"buyer_last_saldo"`
		Price          int64   `json:"price"`
	} `json:"data"`
}

func (c *Client) GetProducts(context.Context, Provider.ProductRequest) ([]Provider.Product, error) {
	return nil, Provider.ErrUnsupportedOperation
}

func (c *Client) Inquiry(context.Context, Provider.InquiryRequest) (Provider.InquiryResult, error) {
	return Provider.InquiryResult{}, Provider.ErrUnsupportedOperation
}

func (c *Client) Purchase(ctx context.Context, req Provider.PurchaseRequest) (Provider.PurchaseResult, error) {
	if err := validateTransactionRequest(req.ProductCode, req.CustomerNo, req.ReferenceID); err != nil {
		return Provider.PurchaseResult{}, err
	}
	data, err := c.transaction(ctx, transactionRequest{
		Username:     c.username,
		BuyerSKUCode: req.ProductCode,
		CustomerNo:   req.CustomerNo,
		ReferenceID:  req.ReferenceID,
		Sign:         c.signature(req.ReferenceID),
		Testing:      req.Testing,
	})
	if err != nil {
		return Provider.PurchaseResult{}, err
	}
	return mapPurchaseResult(data), nil
}

func (c *Client) GetStatus(ctx context.Context, req Provider.StatusRequest) (Provider.PurchaseStatus, error) {
	if err := validateTransactionRequest(req.ProductCode, req.CustomerNo, req.ReferenceID); err != nil {
		return Provider.PurchaseStatus{}, err
	}
	data, err := c.transaction(ctx, transactionRequest{
		Username:     c.username,
		BuyerSKUCode: req.ProductCode,
		CustomerNo:   req.CustomerNo,
		ReferenceID:  req.ReferenceID,
		Sign:         c.signature(req.ReferenceID),
	})
	if err != nil {
		return Provider.PurchaseStatus{}, err
	}
	return mapPurchaseStatus(data), nil
}

func (c *Client) HandleWebhook(_ context.Context, req Provider.WebhookRequest) (Provider.WebhookEvent, error) {
	if req.SignatureSecret != "" {
		expected := hmac.New(sha1.New, []byte(req.SignatureSecret))
		_, _ = expected.Write(req.Body)
		expectedHeader := "sha1=" + hex.EncodeToString(expected.Sum(nil))
		if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(req.Signature)), []byte(expectedHeader)) != 1 {
			return Provider.WebhookEvent{}, ErrInvalidWebhookSignature
		}
	}

	var payload struct {
		Data struct {
			ReferenceID  string `json:"ref_id"`
			CustomerNo   string `json:"customer_no"`
			BuyerSKUCode string `json:"buyer_sku_code"`
			Message      string `json:"message"`
			Status       string `json:"status"`
			RC           string `json:"rc"`
			SN           string `json:"sn"`
			Price        int64  `json:"price"`
		} `json:"data"`
	}
	if err := json.Unmarshal(req.Body, &payload); err != nil {
		return Provider.WebhookEvent{}, fmt.Errorf("decode DigiFlazz webhook: %w", err)
	}
	return Provider.WebhookEvent{
		ReferenceID:  payload.Data.ReferenceID,
		CustomerNo:   payload.Data.CustomerNo,
		ProductCode:  payload.Data.BuyerSKUCode,
		Status:       mapStatus(payload.Data.Status),
		ProviderCode: payload.Data.RC,
		Message:      payload.Data.Message,
		SerialNumber: payload.Data.SN,
		Price:        payload.Data.Price,
	}, nil
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
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return transactionResponse{}, fmt.Errorf("DigiFlazz HTTP status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var decoded transactionResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return transactionResponse{}, fmt.Errorf("decode DigiFlazz response: %w", err)
	}
	return decoded, nil
}

func (c *Client) signature(refID string) string {
	sum := md5.Sum([]byte(c.username + c.apiKey + refID))
	return hex.EncodeToString(sum[:])
}

func validateTransactionRequest(productCode, customerNo, referenceID string) error {
	if productCode == "" || customerNo == "" || referenceID == "" {
		return errors.New("product code, customer number, and reference ID are required")
	}
	return nil
}

func mapPurchaseResult(data transactionResponse) Provider.PurchaseResult {
	return Provider.PurchaseResult{
		ReferenceID:  data.Data.ReferenceID,
		CustomerNo:   data.Data.CustomerNo,
		ProductCode:  data.Data.BuyerSKUCode,
		Status:       mapStatus(data.Data.Status),
		ProviderCode: data.Data.RC,
		Message:      data.Data.Message,
		SerialNumber: data.Data.SN,
		Price:        data.Data.Price,
	}
}

func mapPurchaseStatus(data transactionResponse) Provider.PurchaseStatus {
	return Provider.PurchaseStatus{
		ReferenceID:  data.Data.ReferenceID,
		CustomerNo:   data.Data.CustomerNo,
		ProductCode:  data.Data.BuyerSKUCode,
		Status:       mapStatus(data.Data.Status),
		ProviderCode: data.Data.RC,
		Message:      data.Data.Message,
		SerialNumber: data.Data.SN,
		Price:        data.Data.Price,
	}
}

func mapStatus(status string) Provider.TransactionStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "sukses":
		return Provider.StatusSuccess
	case "pending":
		return Provider.StatusPending
	case "gagal":
		return Provider.StatusFailed
	default:
		return Provider.TransactionStatus(strings.ToLower(strings.TrimSpace(status)))
	}
}
