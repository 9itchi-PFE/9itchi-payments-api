package api

import "time"

// CreatePlanRequest is the validated body of CreatePlan.
type CreatePlanRequest struct {
	Name  string  `json:"name" binding:"required,max=100"`
	Price float64 `json:"price" binding:"gte=0"`
	// RegularPrice is the price shown struck through; it must be above Price. Leave it out when there is none.
	RegularPrice *float64 `json:"regular_price" binding:"omitempty,gt=0"`
	// LaunchEndsOn is the last day Price is charged (YYYY-MM-DD); from the next day the plan costs RegularPrice.
	// It needs a RegularPrice. Leave it out when the launch price has no end.
	LaunchEndsOn string         `json:"launch_ends_on" binding:"omitempty,datetime=2006-01-02"`
	DurationDays int            `json:"duration_days" binding:"required,gt=0"`
	Quotas       map[string]int `json:"quotas" binding:"required"`
	Active       *bool          `json:"active"`
}

// UpdatePlanRequest is the validated body of UpdatePlan (full replacement).
type UpdatePlanRequest struct {
	Name  string  `json:"name" binding:"required,max=100"`
	Price float64 `json:"price" binding:"gte=0"`
	// RegularPrice is the price shown struck through; it must be above Price. Leave it out when there is none.
	RegularPrice *float64 `json:"regular_price" binding:"omitempty,gt=0"`
	// LaunchEndsOn is the last day Price is charged (YYYY-MM-DD); from the next day the plan costs RegularPrice.
	// It needs a RegularPrice. Leave it out when the launch price has no end.
	LaunchEndsOn string         `json:"launch_ends_on" binding:"omitempty,datetime=2006-01-02"`
	DurationDays int            `json:"duration_days" binding:"required,gt=0"`
	Quotas       map[string]int `json:"quotas" binding:"required"`
	Active       *bool          `json:"active"`
}

// PlanResponse is a plan as returned to clients.
type PlanResponse struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Price        float64  `json:"price"`
	RegularPrice *float64 `json:"regular_price,omitempty"`
	// LaunchEndsOn is the last day of the launch price (YYYY-MM-DD), while there is one.
	LaunchEndsOn string `json:"launch_ends_on,omitempty"`
	// Sale is the sale the plan is in today: Price is then the price of the sale, RegularPrice the usual one.
	Sale         *PlanSale      `json:"sale,omitempty"`
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
	// PeriodStart is when the current period of the subscription began: limits "per period" (the downloads of a
	// CV) count from there. Null without a subscription.
	PeriodStart *time.Time `json:"period_start"`
}
