package core

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"9itchi-payments-api/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func flouciCfg(baseURL string) utils.AppConfig {
	return utils.AppConfig{FlouciBaseURL: baseURL, FlouciPublicKey: "pub", FlouciPrivateKey: "priv",
		FlouciWebhookSecret: "whsec", FlouciWebhookURL: "https://api.example.com/webhooks/flouci", FrontendURL: "https://app.example.com/"}
}

func sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestFactoryRegistersFlouci(t *testing.T) {
	f := NewProviderFactory(flouciCfg("http://x"), nil)
	require.Contains(t, f, "flouci")
	assert.True(t, f["flouci"]().Available())
	assert.NotContains(t, f, "stripe")
}

func TestFlouciCreateCheckout(t *testing.T) {
	var got map[string]any
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/generate_payment", r.URL.Path)
		auth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"result":{"payment_id":"ref-1","link":"https://pay.example/ref-1","success":true}}`))
	}))
	defer srv.Close()

	res, err := NewFlouciProvider(flouciCfg(srv.URL), srv.Client()).CreateCheckout(context.Background(), CheckoutRequest{PaymentID: "pay-1", AmountMillimes: 40000})
	require.NoError(t, err)
	assert.Equal(t, CheckoutResult{Ref: "ref-1", URL: "https://pay.example/ref-1"}, res)
	assert.Equal(t, "Bearer pub:priv", auth)
	assert.EqualValues(t, 40000, got["amount"])
	assert.Equal(t, "pay-1", got["developer_tracking_id"])
	assert.Equal(t, "https://app.example.com/settings/billing?payment=success&id=pay-1", got["success_link"])
	assert.Equal(t, "https://app.example.com/settings/billing?payment=failed&id=pay-1", got["fail_link"])
	assert.Equal(t, "https://api.example.com/webhooks/flouci", got["webhook"])
}

func TestFlouciCreateCheckoutErrors(t *testing.T) {
	status, body := http.StatusOK, `{"result":{"success":false}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	p := NewFlouciProvider(flouciCfg(srv.URL), srv.Client())
	req := CheckoutRequest{PaymentID: "pay-1", AmountMillimes: 1000}

	_, err := p.CreateCheckout(context.Background(), req)
	assert.Error(t, err, "success=false")
	status = http.StatusInternalServerError
	_, err = p.CreateCheckout(context.Background(), req)
	assert.Error(t, err, "http 500")
	_, err = p.CreateCheckout(context.Background(), CheckoutRequest{PaymentID: "pay-1"})
	assert.Error(t, err, "zero amount")

	noKeys := NewFlouciProvider(utils.AppConfig{FlouciBaseURL: srv.URL}, srv.Client())
	assert.False(t, noKeys.Available())
	_, err = noKeys.CreateCheckout(context.Background(), req)
	assert.ErrorIs(t, err, ErrProviderUnavailable)
}

func TestFlouciVerifyWebhook(t *testing.T) {
	p := NewFlouciProvider(flouciCfg("http://x"), nil)
	body := []byte(`{"result":{"payment_id":"ref-1"}}`)

	ref, err := p.VerifyWebhook(body, sign("whsec", body))
	require.NoError(t, err)
	assert.Equal(t, "ref-1", ref)

	flat := []byte(`{"id":"ref-2"}`)
	ref, err = p.VerifyWebhook(flat, sign("whsec", flat))
	require.NoError(t, err)
	assert.Equal(t, "ref-2", ref)

	for name, sig := range map[string]string{"wrong secret": sign("other", body), "empty": "", "garbage": "zz"} {
		_, err = p.VerifyWebhook(body, sig)
		assert.ErrorIs(t, err, ErrInvalidSignature, name)
	}

	noSecret := NewFlouciProvider(utils.AppConfig{}, nil)
	_, err = noSecret.VerifyWebhook(body, sign("", body))
	assert.ErrorIs(t, err, ErrInvalidSignature, "unset secret never validates")

	empty := []byte(`{}`)
	_, err = p.VerifyWebhook(empty, sign("whsec", empty))
	assert.ErrorIs(t, err, ErrInvalidPayload)
}

func TestFlouciVerifyPayment(t *testing.T) {
	status := "SUCCESS"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/verify_payment/ref-1", r.URL.Path)
		assert.Equal(t, "Bearer pub:priv", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"success":true,"result":{"status":"` + status + `"}}`))
	}))
	defer srv.Close()
	p := NewFlouciProvider(flouciCfg(srv.URL), srv.Client())

	paid, err := p.VerifyPayment(context.Background(), "ref-1")
	require.NoError(t, err)
	assert.True(t, paid)

	status = "PENDING"
	paid, err = p.VerifyPayment(context.Background(), "ref-1")
	require.NoError(t, err)
	assert.False(t, paid)

	_, err = p.VerifyPayment(context.Background(), "")
	assert.Error(t, err)
}
