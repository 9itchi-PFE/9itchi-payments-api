package api

import "time"

// CheckoutRequest is the validated body of Checkout. Add binding tags as fields are defined.
type CheckoutRequest struct{}

// SubscriptionResponse is a subscription row as returned to its owner.
type SubscriptionResponse struct {
	ID          string    `json:"id"`
	PlanID      string    `json:"plan_id"`
	PromoCodeID *string   `json:"promo_code_id"`
	Status      string    `json:"status"`
	Amount      float64   `json:"amount"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
}

// MySubscriptionResponse is the active subscription with its plan, or the Free plan view (subscription null).
type MySubscriptionResponse struct {
	Subscription *SubscriptionResponse `json:"subscription"`
	Plan         PlanResponse          `json:"plan"`
	IsFree       bool                  `json:"is_free"`
}
