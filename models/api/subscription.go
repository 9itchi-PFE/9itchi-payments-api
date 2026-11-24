package api

import "time"

// CheckoutRequest is the validated body of Checkout. PromoCode is optional.
type CheckoutRequest struct {
	PlanID    string `json:"plan_id" binding:"required,uuid"`
	PromoCode string `json:"promo_code" binding:"max=64"`
}

// CheckoutResponse tells the client where to pay. PaymentURL is empty when nothing is due (activated directly).
type CheckoutResponse struct {
	PaymentURL string `json:"payment_url"`
	PaymentID  string `json:"payment_id"`
}

// SubscriptionResponse is a subscription row as returned to its owner.
type SubscriptionResponse struct {
	ID          string    `json:"id"`
	PlanID      string    `json:"plan_id"`
	PromoCodeID *string   `json:"promo_code_id"`
	Status      string    `json:"status"`
	Amount      float64   `json:"amount"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
	// Renews is true until the student stops the subscription: a new payment is then asked when the period ends
	// (nothing is charged automatically). StoppedAt is when they stopped it.
	Renews    bool       `json:"renews"`
	StoppedAt *time.Time `json:"stopped_at"`
}

// MySubscriptionResponse is the active subscription with its plan, or the Free plan view (subscription null).
type MySubscriptionResponse struct {
	Subscription *SubscriptionResponse `json:"subscription"`
	Plan         PlanResponse          `json:"plan"`
	IsFree       bool                  `json:"is_free"`
	// Lapsed is set on the Free view when the last subscription ended without its renewal being paid and the
	// student had not stopped it: the renewal is still due.
	Lapsed *SubscriptionResponse `json:"lapsed"`
}

// PaymentResponse is one payment of the user: its state after the return from the gateway, and its receipt.
// Amounts are integer millimes (1/1000 TND): amount is what was paid, discount what the promo code took off.
type PaymentResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	// ReceiptNo is "<year>-<6 digits>", empty until the payment is completed.
	ReceiptNo        string     `json:"receipt_no"`
	PlanID           string     `json:"plan_id"`
	PlanName         string     `json:"plan_name"`
	DurationDays     int        `json:"duration_days"`
	AmountMillimes   int64      `json:"amount_millimes"`
	DiscountMillimes int64      `json:"discount_millimes"`
	PromoCode        string     `json:"promo_code"`
	Provider         string     `json:"provider"`
	ProviderRef      string     `json:"provider_ref"`
	PeriodStart      *time.Time `json:"period_start"`
	PeriodEnd        *time.Time `json:"period_end"`
	CompletedAt      *time.Time `json:"completed_at"`
	CreatedAt        time.Time  `json:"created_at"`
}

// PaymentListResponse wraps the user's completed payments, newest first.
type PaymentListResponse struct {
	Items []PaymentResponse `json:"items"`
}
