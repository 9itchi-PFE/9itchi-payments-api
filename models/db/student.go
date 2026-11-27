package db

import "time"

// StudentDiscount maps the single row of table student_discount: the percentage taken off for student emails.
type StudentDiscount struct {
	// Percent is taken off at checkout, after any sale or promo code (0 = off).
	Percent int `db:"percent" json:"percent"`
	// Domains are lower-case email domains; a subdomain of one counts too.
	Domains   []string  `db:"domains" json:"domains"`
	UpdatedBy *string   `db:"updated_by" json:"updated_by"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}
