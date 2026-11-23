package core

import "context"

// PaymentProvider is one pluggable gateway (Stripe, local). Register in the factory.
type PaymentProvider interface {
	Name() string
	CreateCheckout(ctx context.Context, userID, planID string, amount float64) (checkoutURL string, err error)
	VerifyWebhook(payload []byte, signature string) (eventID string, err error)
}

type ProviderFactory map[string]func() PaymentProvider
