package db

import "time"

// Subscription maps table subscriptions.
type Subscription struct {
	ID          string    `db:"id" json:"id"`
	UserID      string    `db:"user_id" json:"user_id"`
	PlanID      string    `db:"plan_id" json:"plan_id"`
	PromoCodeID *string   `db:"promo_code_id" json:"promo_code_id"`
	Status      string    `db:"status" json:"status"`
	Amount      float64   `db:"amount" json:"amount"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}
