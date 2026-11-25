package db

import "time"

// Plan maps table plans.
type Plan struct {
	ID           string   `db:"id" json:"id"`
	Name         string   `db:"name" json:"name"`
	Price        float64  `db:"price" json:"price"`
	RegularPrice *float64 `db:"regular_price" json:"regular_price"`
	// LaunchEndsOn is the last day Price is charged (a day, at midnight UTC); from the next day in Tunis the plan
	// costs RegularPrice. Nil: no end.
	LaunchEndsOn *time.Time     `db:"launch_ends_on" json:"launch_ends_on"`
	DurationDays int            `db:"duration_days" json:"duration_days"`
	Quotas       map[string]int `db:"quotas" json:"quotas"`
	Active       bool           `db:"active" json:"active"`
	CreatedAt    time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time      `db:"updated_at" json:"updated_at"`
	// Sale is the sale the plan is in at the moment it was priced (not a column): Price is then the price of
	// the sale and RegularPrice the price struck through.
	Sale *Sale `db:"-" json:"-"`
}
