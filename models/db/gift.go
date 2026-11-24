package db

import "time"

// Gift maps table subscription_gifts: days the team gave a student for free.
type Gift struct {
	ID             string `db:"id" json:"id"`
	UserID         string `db:"user_id" json:"user_id"`
	SubscriptionID string `db:"subscription_id" json:"subscription_id"`
	PlanID         string `db:"plan_id" json:"plan_id"`
	Days           int    `db:"days" json:"days"`
	// GivenBy is the admin who gave the days.
	GivenBy string `db:"given_by" json:"given_by"`
	// PeriodStart and PeriodEnd are the days the gift added to the subscription.
	PeriodStart time.Time `db:"period_start" json:"period_start"`
	PeriodEnd   time.Time `db:"period_end" json:"period_end"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
}
