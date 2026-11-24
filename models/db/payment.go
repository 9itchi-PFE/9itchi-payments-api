package db

import "time"

// Payment maps table payments. Amounts are integer millimes (1/1000 TND).
type Payment struct {
	ID               string     `db:"id" json:"id"`
	UserID           string     `db:"user_id" json:"user_id"`
	PlanID           string     `db:"plan_id" json:"plan_id"`
	PromoCodeID      *string    `db:"promo_code_id" json:"promo_code_id"`
	Provider         string     `db:"provider" json:"provider"`
	ProviderRef      *string    `db:"provider_ref" json:"provider_ref"`
	Status           string     `db:"status" json:"status"`
	AmountMillimes   int64      `db:"amount_millimes" json:"amount_millimes"`
	DiscountMillimes int64      `db:"discount_millimes" json:"discount_millimes"`
	CompletedAt      *time.Time `db:"completed_at" json:"completed_at"`
	CreatedAt        time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at" json:"updated_at"`
}
