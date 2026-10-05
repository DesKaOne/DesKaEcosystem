package routing

import (
  "encoding/json"
  "errors"
  "io"
  "net/http"
  "strings"
  payment "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/internal/Payment"
)

const DefaultPaymentWebhookMaxBodyBytes int64 = 1 << 20

// PaymentWebhookHTTPHandler is the provider-neutral HTTP transport boundary.
type PaymentWebhookHTTPHandler struct { Service *Service; MaxBodyBytes int64 }

func NewPaymentWebhookHTTPHandler(service *Service) (*PaymentWebhookHTTPHandler, error) {
  if service == nil { return nil, errors.New("payment webhook service is required") }
  return &PaymentWebhookHTTPHandler{Service: service, MaxBodyBytes: DefaultPaymentWebhookMaxBodyBytes}, nil
}

func (h *PaymentWebhookHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  if h == nil || h.Service == nil { writePaymentWebhookError(w, http.StatusInternalServerError, "webhook service unavailable"); return }
  if r.Method != http.MethodPost { w.Header().Set("Allow", http.MethodPost); writePaymentWebhookError(w, http.StatusMethodNotAllowed, "method not allowed"); return }
  providerName := paymentWebhookProviderFromPath(r.URL.Path)
  if providerName == "" { writePaymentWebhookError(w, http.StatusBadRequest, "provider name is required"); return }
  maxBody := h.MaxBodyBytes; if maxBody <= 0 { maxBody = DefaultPaymentWebhookMaxBodyBytes }
  body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
  if err != nil {
    if strings.Contains(strings.ToLower(err.Error()), "request body too large") { writePaymentWebhookError(w, http.StatusRequestEntityTooLarge, "request body too large"); return }
    writePaymentWebhookError(w, http.StatusBadRequest, "invalid request body"); return
  }
  result, err := h.Service.HandlePaymentWebhook(r.Context(), providerName, body)
  if err != nil { writePaymentWebhookError(w, paymentWebhookHTTPStatus(err), paymentWebhookPublicError(err)); return }
  writePaymentWebhookResult(w, http.StatusOK, result)
}

type paymentWebhookHTTPResult struct { ReferenceID string; ProviderReference string; Status string }
type paymentWebhookHTTPError struct { Error string }

func writePaymentWebhookResult(w http.ResponseWriter, status int, result payment.StatusResult) {
  w.Header().Set("Content-Type", "application/json"); w.WriteHeader(status)
  _ = json.NewEncoder(w).Encode(paymentWebhookHTTPResult{ReferenceID: result.ReferenceID, ProviderReference: result.ProviderReference, Status: string(result.Status)})
}
func writePaymentWebhookError(w http.ResponseWriter, status int, message string) {
  w.Header().Set("Content-Type", "application/json"); w.WriteHeader(status); _ = json.NewEncoder(w).Encode(paymentWebhookHTTPError{Error: message})
}
func paymentWebhookProviderFromPath(path string) string {
  const prefix = "/webhooks/payment/"; if !strings.HasPrefix(path, prefix) { return "" }
  rest := strings.TrimPrefix(path, prefix); if rest == "" || strings.Contains(rest, "/") { return "" }; return strings.TrimSpace(rest)
}
func paymentWebhookHTTPStatus(err error) int {
  switch { case errors.Is(err, ErrInvalidWebhookEvent): return http.StatusBadRequest; case errors.Is(err, ErrWebhookTransactionNotFound): return http.StatusNotFound; case errors.Is(err, ErrWebhookReferenceConflict): return http.StatusConflict; case errors.Is(err, ErrPaymentCapabilityDisabled): return http.StatusServiceUnavailable; default: return http.StatusBadRequest }
}
func paymentWebhookPublicError(err error) string {
  switch { case errors.Is(err, ErrWebhookTransactionNotFound): return "payment transaction not found"; case errors.Is(err, ErrWebhookReferenceConflict): return "payment webhook conflicts with stored transaction"; case errors.Is(err, ErrPaymentCapabilityDisabled): return "payment capability unavailable"; case errors.Is(err, ErrInvalidWebhookEvent): return "invalid payment webhook"; default: return "payment webhook rejected" }
}