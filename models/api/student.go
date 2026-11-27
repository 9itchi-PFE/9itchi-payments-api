package api

import "time"

// UpdateStudentDiscountRequest is the validated body of UpdateStudentDiscount (full replacement).
type UpdateStudentDiscountRequest struct {
	Percent *int     `json:"percent" binding:"required,min=0,max=100"`
	Domains []string `json:"domains" binding:"max=200,dive,max=253"`
}

// StudentDiscountResponse is the student discount as the admin set it.
type StudentDiscountResponse struct {
	Percent   int       `json:"percent"`
	Domains   []string  `json:"domains"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MyStudentDiscountResponse says whether the signed-in user gets the student discount at checkout.
// Domain is the student domain their email matched; NeedsVerifiedEmail is true when the domain matches but
// the email was not vouched for by a Google or LinkedIn sign-in.
type MyStudentDiscountResponse struct {
	Eligible           bool   `json:"eligible"`
	Percent            int    `json:"percent"`
	Domain             string `json:"domain,omitempty"`
	NeedsVerifiedEmail bool   `json:"needs_verified_email,omitempty"`
}
