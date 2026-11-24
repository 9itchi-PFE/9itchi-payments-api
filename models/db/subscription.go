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
	StartsAt    time.Time `db:"starts_at" json:"starts_at"`
	EndsAt      time.Time `db:"ends_at" json:"ends_at"`
	// StoppedAt is when the student stopped the renewal; nil means a new payment is asked when the period ends.
	StoppedAt *time.Time `db:"stopped_at" json:"stopped_at"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
}

// Renewal reminder stages.
const (
	ReminderDue    = "due"    // the period ends soon
	ReminderLapsed = "lapsed" // the period ended and was not renewed
)

// RenewalReminder is one subscription a reminder has just been claimed for, with what the reminder says.
type RenewalReminder struct {
	SubscriptionID string    `db:"id" json:"subscription_id"`
	UserID         string    `db:"user_id" json:"user_id"`
	PlanID         string    `db:"plan_id" json:"plan_id"`
	PlanName       string    `db:"name" json:"plan_name"`
	PlanPrice      float64   `db:"price" json:"plan_price"`
	EndsAt         time.Time `db:"ends_at" json:"ends_at"`
}
