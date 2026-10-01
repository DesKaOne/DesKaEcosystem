package midtrans

import (
	"bytes"
	"context"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	payment "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/internal/Payment"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/config"
)

const maxResponseBody = 1 << 20

var _ payment.Provider = (*Client)(nil)
var _ payment.WebhookProvider = (*Client)(nil)

type Client struct {
	cfg        config.MidtransConfig
	httpClient *http.Client
}

func New(cfg config.MidtransConfig, httpClient *http.Client) (*Client, error) {
	if cfg.ServerKey == "" {
		return nil, errors.New("midtrans server key is required")
	}
	if cfg.SnapEndpoint == "" || cfg.APIEndpoint == "" {
		return nil, errors.New("midtrans endpoints are required")
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{cfg: cfg, httpClient: httpClient}, nil
}

type snapCreateRequest struct {
	TransactionDetails snapTransactionDetails `json:"transaction_details"`
}

type snapTransactionDetails struct {
	OrderID     string `json:"order_id"`
	GrossAmount int64  `json:"gross_amount"`
}

type snapCreateResponse struct {
	Token       string `json:"token"`
	RedirectURL string `json:"redirect_url"`
}

type transactionResponse struct {
	StatusCode        string `json:"status_code"`
	StatusMessage     string `json:"status_message"`
	TransactionID     string `json:"transaction_id"`
	OrderID           string `json:"order_id"`
	TransactionStatus string `json:"transaction_status"`
	GrossAmount       string `json:"gross_amount"`
	FraudStatus       string `json:"fraud_status"`
}

type notification struct {
	StatusCode        string `json:"status_code"`
	StatusMessage     string `json:"status_message"`
	SignatureKey      string `json:"signature_key"`
	TransactionID     string `json:"transaction_id"`
	OrderID           string `json:"order_id"`
	TransactionStatus string `json:"transaction_status"`
	GrossAmount       string `json:"gross_amount"`
}

func (c *Client) CreatePayment(ctx context.Context, req payment.PaymentRequest) (payment.PaymentResult, error) {
	if err := payment.ValidateRequest(req); err != nil {
		return payment.PaymentResult{}, err
	}
	if !strings.EqualFold(req.Currency, "IDR") {
		return payment.PaymentResult{}, payment.ErrUnsupported
	}

	payload, err := json.Marshal(snapCreateRequest{TransactionDetails: snapTransactionDetails{
		OrderID: req.ReferenceID, GrossAmount: req.Amount,
	}})
	if err != nil {
		return payment.PaymentResult{}, fmt.Errorf("marshal midtrans create request: %w", err)
	}

	var response snapCreateResponse
	statusCode, err := c.doJSON(ctx, http.MethodPost, c.cfg.SnapEndpoint, payload, &response)
	if err != nil {
		return payment.PaymentResult{}, err
	}
	if statusCode != http.StatusCreated {
		return payment.PaymentResult{}, fmt.Errorf("midtrans create payment: unexpected HTTP status %d", statusCode)
	}
	if response.Token == "" {
		return payment.PaymentResult{}, errors.New("midtrans create payment: missing token")
	}

	return payment.PaymentResult{
		ReferenceID:       req.ReferenceID,
		ProviderReference: response.Token,
		Status:            payment.StatusPending,
		Amount:            req.Amount,
		Currency:          "IDR",
		Message:           "Midtrans Snap payment token created",
	}, nil
}

func (c *Client) GetPaymentStatus(ctx context.Context, req payment.StatusRequest) (payment.StatusResult, error) {
	if err := payment.ValidateStatusRequest(req); err != nil {
		return payment.StatusResult{}, err
	}

	identifier := req.ReferenceID
	if req.ProviderReference != "" {
		identifier = req.ProviderReference
	}
	url := strings.TrimRight(c.cfg.APIEndpoint, "/") + "/v2/" + identifier + "/status"

	var response transactionResponse
	statusCode, err := c.doJSON(ctx, http.MethodGet, url, nil, &response)
	if err != nil {
		return payment.StatusResult{}, err
	}
	if statusCode != http.StatusOK {
		return payment.StatusResult{}, fmt.Errorf("midtrans get status: unexpected HTTP status %d", statusCode)
	}
	if response.TransactionStatus == "" {
		return payment.StatusResult{}, errors.New("midtrans get status: missing transaction_status")
	}
	if response.OrderID == "" || response.TransactionID == "" {
		return payment.StatusResult{}, errors.New("midtrans get status: missing transaction identity")
	}
	if req.ReferenceID != "" && response.OrderID != req.ReferenceID {
		return payment.StatusResult{}, errors.New("midtrans get status: order_id mismatch")
	}
	if req.ProviderReference != "" && response.TransactionID != req.ProviderReference {
		return payment.StatusResult{}, errors.New("midtrans get status: transaction_id mismatch")
	}

	amount, err := parseAmount(response.GrossAmount)
	if err != nil {
		return payment.StatusResult{}, fmt.Errorf("midtrans get status: invalid gross_amount: %w", err)
	}
	status, err := normalizeStatus(response.TransactionStatus)
	if err != nil {
		return payment.StatusResult{}, err
	}

	return payment.StatusResult{
		ReferenceID:       response.OrderID,
		ProviderReference: response.TransactionID,
		Status:            status,
		Amount:            amount,
		Currency:          "IDR",
		Message:           response.StatusMessage,
	}, nil
}

func (c *Client) HandlePaymentWebhook(ctx context.Context, payload []byte) (payment.StatusResult, error) {
	if err := ctx.Err(); err != nil {
		return payment.StatusResult{}, err
	}

	var event notification
	if err := json.Unmarshal(payload, &event); err != nil {
		return payment.StatusResult{}, fmt.Errorf("midtrans webhook: invalid JSON: %w", err)
	}
	if event.OrderID == "" || event.TransactionID == "" || event.StatusCode == "" ||
		event.TransactionStatus == "" || event.GrossAmount == "" || event.SignatureKey == "" {
		return payment.StatusResult{}, errors.New("midtrans webhook: incomplete notification")
	}

	expected := signature(event.OrderID, event.StatusCode, event.GrossAmount, c.cfg.ServerKey)
	if subtle.ConstantTimeCompare([]byte(strings.ToLower(event.SignatureKey)), []byte(expected)) != 1 {
		return payment.StatusResult{}, errors.New("midtrans webhook: invalid signature")
	}

	amount, err := parseAmount(event.GrossAmount)
	if err != nil {
		return payment.StatusResult{}, fmt.Errorf("midtrans webhook: invalid gross_amount: %w", err)
	}
	status, err := normalizeStatus(event.TransactionStatus)
	if err != nil {
		return payment.StatusResult{}, err
	}
	if status == payment.StatusSuccess {
		if event.StatusCode != "200" {
			return payment.StatusResult{}, fmt.Errorf("midtrans webhook: success transaction has unexpected status_code %q", event.StatusCode)
		}
		if !strings.EqualFold(event.FraudStatus, "accept") {
			return payment.StatusResult{}, fmt.Errorf("midtrans webhook: success transaction has unacceptable fraud_status %q", event.FraudStatus)
		}
	}

	return payment.StatusResult{
		ReferenceID:       event.OrderID,
		ProviderReference: event.TransactionID,
		Status:            status,
		Amount:            amount,
		Currency:          "IDR",
		Message:           event.StatusMessage,
	}, nil
}

func signature(orderID, statusCode, grossAmount, serverKey string) string {
	input := orderID + statusCode + grossAmount + serverKey
	sum := sha512.Sum512([]byte(input))
	return hex.EncodeToString(sum[:])
}

func normalizeStatus(value string) (payment.Status, error) {
	switch strings.ToLower(value) {
	case "capture", "settlement":
		return payment.StatusSuccess, nil
	case "pending", "authorize":
		return payment.StatusPending, nil
	case "deny", "cancel", "expire", "failure", "refund", "partial_refund", "chargeback", "partial_chargeback":
		return payment.StatusFailed, nil
	default:
		return "", fmt.Errorf("midtrans: unsupported transaction status %q", value)
	}
}

func parseAmount(value string) (int64, error) {
	if value == "" {
		return 0, errors.New("empty amount")
	}
	if strings.Contains(value, ".") {
		f, err := strconv.ParseFloat(value, 64)
		if err != nil || f < 0 || f != float64(int64(f)) {
			return 0, errors.New("amount is not an integer IDR value")
		}
		return int64(f), nil
	}
	return strconv.ParseInt(value, 10, 64)
}

func (c *Client) doJSON(ctx context.Context, method, url string, body []byte, out any) (int, error) {
	if ctx == nil {
		return 0, errors.New("context is required")
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return 0, fmt.Errorf("midtrans request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(c.cfg.ServerKey+":")))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("midtrans HTTP request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return resp.StatusCode, fmt.Errorf("midtrans response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("midtrans HTTP status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return resp.StatusCode, fmt.Errorf("midtrans response JSON: %w", err)
		}
	}
	return resp.StatusCode, nil
}
