package api

import "time"

// CreatePlanRequest is the validated body of CreatePlan.
type CreatePlanRequest struct {
	Name         string         `json:"name" binding:"required,max=100"`
	Price        float64        `json:"price" binding:"gte=0"`
	DurationDays int            `json:"duration_days" binding:"required,gt=0"`
	Quotas       map[string]int `json:"quotas" binding:"required"`
	Active       *bool          `json:"active"`
}

// UpdatePlanRequest is the validated body of UpdatePlan (full replacement).
type UpdatePlanRequest struct {
	Name         string         `json:"name" binding:"required,max=100"`
	Price        float64        `json:"price" binding:"gte=0"`
	DurationDays int            `json:"duration_days" binding:"required,gt=0"`
	Quotas       map[string]int `json:"quotas" binding:"required"`
	Active       *bool          `json:"active"`
}

// PlanResponse is a plan as returned to clients.
type PlanResponse struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Price        float64        `json:"price"`
	DurationDays int            `json:"duration_days"`
	Quotas       map[string]int `json:"quotas"`
	Active       bool           `json:"active"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// PlanListResponse wraps a list of plans.
type PlanListResponse struct {
	Items []PlanResponse `json:"items"`
}

// QuotasResponse is the quota set that applies to a user.
type QuotasResponse struct {
	UserID   string         `json:"user_id"`
	PlanID   string         `json:"plan_id"`
	PlanName string         `json:"plan_name"`
	Quotas   map[string]int `json:"quotas"`
}
