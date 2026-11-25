package api

import "time"

// SaleRequest is the validated body of CreateSale and UpdateSale (full replacement).
type SaleRequest struct {
	// Name is shown to students next to the price (« Rentrée »).
	Name string `json:"name" binding:"required,max=60"`
	// Percent taken off the price of the plan, 1 to 90.
	Percent int `json:"percent" binding:"required,min=1,max=90"`
	// StartsOn and EndsOn are the first and the last day of the sale (YYYY-MM-DD, counted in Tunis).
	StartsOn string `json:"starts_on" binding:"required,datetime=2006-01-02"`
	EndsOn   string `json:"ends_on" binding:"required,datetime=2006-01-02"`
	// PlanIDs limits the sale to some plans; empty or absent: every paid plan.
	PlanIDs []string `json:"plan_ids" binding:"max=50,dive,uuid"`
}

// SaleResponse is a sale as the admin sees it. State is "upcoming", "running" or "over" today.
type SaleResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Percent   int       `json:"percent"`
	StartsOn  string    `json:"starts_on"`
	EndsOn    string    `json:"ends_on"`
	PlanIDs   []string  `json:"plan_ids"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SaleListResponse wraps the sales, the ones still to come or running first.
type SaleListResponse struct {
	Items []SaleResponse `json:"items"`
}

// PlanSale is the sale a plan is in today, as a student sees it.
type PlanSale struct {
	Name    string `json:"name"`
	Percent int    `json:"percent"`
	// EndsOn is the last day of the sale (YYYY-MM-DD).
	EndsOn string `json:"ends_on"`
}
