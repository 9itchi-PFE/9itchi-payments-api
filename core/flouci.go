package core

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"9itchi-payments-api/utils"
)

const (
	FlouciProviderName = "flouci"
	// FlouciHTTPTimeout bounds every call to the gateway.
	FlouciHTTPTimeout = 30 * time.Second
	maxFlouciBody     = 1 << 20
	flouciStatusPaid  = "SUCCESS"
)

type flouciProvider struct {
	baseURL       string
	publicKey     string
	privateKey    string
	webhookSecret string
	webhookURL    string
	frontendURL   string
	client        *http.Client
}

// NewFlouciProvider builds the Flouci gateway client. A nil client gets the default timeout.
func NewFlouciProvider(cfg utils.AppConfig, client *http.Client) PaymentProvider {
	if client == nil {
		client = &http.Client{Timeout: FlouciHTTPTimeout}
	}
	return &flouciProvider{
		baseURL:       strings.TrimRight(cfg.FlouciBaseURL, "/"),
		publicKey:     cfg.FlouciPublicKey,
		privateKey:    cfg.FlouciPrivateKey,
		webhookSecret: cfg.FlouciWebhookSecret,
		webhookURL:    cfg.FlouciWebhookURL,
		frontendURL:   strings.TrimRight(cfg.FrontendURL, "/"),
		client:        client,
	}
}

func (f *flouciProvider) Available() bool { return f.publicKey != "" && f.privateKey != "" }

func (f *flouciProvider) do(ctx context.Context, method, path string, body []byte, out any) error {
	if !f.Available() {
		return ErrProviderUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, method, f.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+f.publicKey+":"+f.privateKey)
	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("flouci request: %w", ErrProviderUnavailable)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxFlouciBody))
	if err != nil {
		return fmt.Errorf("flouci read: %w", ErrProviderUnavailable)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("flouci: status %d", resp.StatusCode)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("flouci: unexpected response: %w", err)
	}
	return nil
}

func (f *flouciProvider) CreateCheckout(ctx context.Context, req CheckoutRequest) (CheckoutResult, error) {
	if req.AmountMillimes <= 0 {
		return CheckoutResult{}, fmt.Errorf("flouci: invalid amount")
	}
	payload := map[string]any{
		"amount":                req.AmountMillimes,
		"success_link":          f.frontendURL + ReturnPath(req.PaymentID, "success"),
		"fail_link":             f.frontendURL + ReturnPath(req.PaymentID, "failed"),
		"developer_tracking_id": req.PaymentID,
		"accept_card":           true,
	}
	if sendableWebhookURL(f.webhookURL) {
		payload["webhook"] = f.webhookURL
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return CheckoutResult{}, err
	}
	var out struct {
		Result struct {
			PaymentID string `json:"payment_id"`
			Link      string `json:"link"`
			Success   *bool  `json:"success"`
		} `json:"result"`
	}
	if err := f.do(ctx, http.MethodPost, "/api/v2/generate_payment", body, &out); err != nil {
		return CheckoutResult{}, err
	}
	if (out.Result.Success != nil && !*out.Result.Success) || out.Result.PaymentID == "" || out.Result.Link == "" {
		return CheckoutResult{}, fmt.Errorf("flouci: payment rejected")
	}
	return CheckoutResult{Ref: out.Result.PaymentID, URL: out.Result.Link}, nil
}

// VerifyWebhook checks HMAC-SHA256(body, secret) in hex against the signature header.
func (f *flouciProvider) VerifyWebhook(payload []byte, signature string) (string, error) {
	if f.webhookSecret == "" || signature == "" {
		return "", ErrInvalidSignature
	}
	mac := hmac.New(sha256.New, []byte(f.webhookSecret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(strings.ToLower(strings.TrimSpace(signature)))) {
		return "", ErrInvalidSignature
	}
	var body struct {
		PaymentID string `json:"payment_id"`
		ID        string `json:"id"`
		Result    struct {
			PaymentID string `json:"payment_id"`
			ID        string `json:"id"`
		} `json:"result"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		return "", ErrInvalidPayload
	}
	for _, ref := range []string{body.Result.PaymentID, body.Result.ID, body.PaymentID, body.ID} {
		if ref != "" {
			return ref, nil
		}
	}
	return "", ErrInvalidPayload
}

func (f *flouciProvider) VerifyPayment(ctx context.Context, ref string) (bool, error) {
	if ref == "" {
		return false, fmt.Errorf("flouci: payment reference required")
	}
	var out struct {
		Success bool `json:"success"`
		Result  struct {
			Status string `json:"status"`
		} `json:"result"`
	}
	if err := f.do(ctx, http.MethodGet, "/api/v2/verify_payment/"+url.PathEscape(ref), nil, &out); err != nil {
		return false, err
	}
	return out.Success && out.Result.Status == flouciStatusPaid, nil
}

// sendableWebhookURL skips unset or local URLs, which the gateway cannot reach.
func sendableWebhookURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "localhost", "127.0.0.1", "::1":
		return false
	}
	return true
}
