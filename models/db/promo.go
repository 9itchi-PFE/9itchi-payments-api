package db

import "time"

// PromoCode maps table promo_codes.
type PromoCode struct {
	ID         string    `db:"id" json:"id"`
	Code       string    `db:"code" json:"code"`
	Type       string    `db:"type" json:"type"`
	Value      float64   `db:"value" json:"value"`
	ValidFrom  time.Time `db:"valid_from" json:"valid_from"`
	ValidUntil time.Time `db:"valid_until" json:"valid_until"`
	UsageLimit int       `db:"usage_limit" json:"usage_limit"`
	UsedCount  int       `db:"used_count" json:"used_count"`
	PlanIDs    []string  `db:"plan_ids" json:"plan_ids"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at" json:"updated_at"`
}
