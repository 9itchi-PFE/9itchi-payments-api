package core

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"9itchi-payments-api/utils"
)

var (
	// ErrProviderUnavailable means the gateway is not configured or could not be reached.
	ErrProviderUnavailable = errors.New("payment provider unavailable")
	// ErrInvalidSignature means the webhook was not signed with the shared secret.
	ErrInvalidSignature = errors.New("invalid webhook signature")
	// ErrInvalidPayload means a correctly signed webhook body has no payment reference.
	ErrInvalidPayload = errors.New("invalid webhook payload")
)

// CheckoutRequest is what a provider needs to open a payment. AmountMillimes is in 1/1000 TND.
type CheckoutRequest struct {
	PaymentID      string
	AmountMillimes int64
}

// ReturnPath is the page of the client the gateway sends the user back to; the client then asks
// POST /payments/:id/verify what really happened (the query string is never trusted).
func ReturnPath(paymentID, outcome string) string {
	return "/settings/billing?payment=" + outcome + "&id=" + url.QueryEscape(paymentID)
}

// CheckoutResult is the provider's reference for the payment and the URL the user pays at.
type CheckoutResult struct {
	Ref string
	URL string
}

// PaymentProvider is one pluggable gateway (Flouci, ...). Register it in NewProviderFactory.
type PaymentProvider interface {
	// Available reports whether the gateway credentials are configured.
	Available() bool
	CreateCheckout(ctx context.Context, req CheckoutRequest) (CheckoutResult, error)
	// VerifyWebhook checks the signature in constant time and returns the provider payment reference.
	VerifyWebhook(payload []byte, signature string) (ref string, err error)
	// VerifyPayment asks the gateway whether the payment is really paid (the webhook body is never trusted).
	VerifyPayment(ctx context.Context, ref string) (paid bool, err error)
}

type ProviderFactory map[string]func() PaymentProvider

// NewProviderFactory registers every provider by name; callers dispatch with a map lookup.
func NewProviderFactory(cfg utils.AppConfig, client *http.Client) ProviderFactory {
	return ProviderFactory{
		FlouciProviderName: func() PaymentProvider { return NewFlouciProvider(cfg, client) },
	}
}
