package db

import "time"

// Plan maps table plans.
type Plan struct {
	ID           string         `db:"id" json:"id"`
	Name         string         `db:"name" json:"name"`
	Price        float64        `db:"price" json:"price"`
	RegularPrice *float64       `db:"regular_price" json:"regular_price"`
	DurationDays int            `db:"duration_days" json:"duration_days"`
	Quotas       map[string]int `db:"quotas" json:"quotas"`
	Active       bool           `db:"active" json:"active"`
	CreatedAt    time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time      `db:"updated_at" json:"updated_at"`
}
