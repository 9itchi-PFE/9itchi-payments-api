package api

import "time"

// ValidatePromoRequest is the validated body of ValidatePromo.
type ValidatePromoRequest struct {
	Code   string `json:"code" binding:"required,max=64"`
	PlanID string `json:"plan_id" binding:"required,uuid"`
}

// CreatePromoRequest is the validated body of CreatePromo.
type CreatePromoRequest struct {
	Code       string    `json:"code" binding:"required,min=3,max=64"`
	Type       string    `json:"type" binding:"required,oneof=percentage fixed"`
	Value      float64   `json:"value" binding:"gt=0"`
	ValidFrom  time.Time `json:"valid_from" binding:"required"`
	ValidUntil time.Time `json:"valid_until" binding:"required"`
	UsageLimit int       `json:"usage_limit" binding:"gte=0"`
	PlanIDs    []string  `json:"plan_ids" binding:"dive,uuid"`
}

// UpdatePromoRequest is the validated body of UpdatePromo (full replacement, used_count is kept).
type UpdatePromoRequest struct {
	Code       string    `json:"code" binding:"required,min=3,max=64"`
	Type       string    `json:"type" binding:"required,oneof=percentage fixed"`
	Value      float64   `json:"value" binding:"gt=0"`
	ValidFrom  time.Time `json:"valid_from" binding:"required"`
	ValidUntil time.Time `json:"valid_until" binding:"required"`
	UsageLimit int       `json:"usage_limit" binding:"gte=0"`
	PlanIDs    []string  `json:"plan_ids" binding:"dive,uuid"`
}

// PromoResponse is a promo code as returned to admins.
type PromoResponse struct {
	ID         string    `json:"id"`
	Code       string    `json:"code"`
	Type       string    `json:"type"`
	Value      float64   `json:"value"`
	ValidFrom  time.Time `json:"valid_from"`
	ValidUntil time.Time `json:"valid_until"`
	UsageLimit int       `json:"usage_limit"`
	UsedCount  int       `json:"used_count"`
	PlanIDs    []string  `json:"plan_ids"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// PromoListResponse wraps a list of promo codes.
type PromoListResponse struct {
	Items []PromoResponse `json:"items"`
}

// ValidatePromoResponse is the server-side price computation for a code on a plan.
type ValidatePromoResponse struct {
	Valid          bool    `json:"valid"`
	DiscountAmount float64 `json:"discount_amount"`
	FinalPrice     float64 `json:"final_price"`
	Reason         string  `json:"reason,omitempty"`
}

// MessageResponse is a plain acknowledgement body.
type MessageResponse struct {
	Message string `json:"message"`
}
