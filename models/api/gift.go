package api

import "time"

// GiftResponse is a month the team gave a student: the plan, the days it added and who gave it.
type GiftResponse struct {
	ID          string    `json:"id"`
	PlanID      string    `json:"plan_id"`
	PlanName    string    `json:"plan_name"`
	Days        int       `json:"days"`
	GivenBy     string    `json:"given_by"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	CreatedAt   time.Time `json:"created_at"`
}

// GiftListResponse wraps the gifts of one student, newest first.
type GiftListResponse struct {
	Items []GiftResponse `json:"items"`
}
