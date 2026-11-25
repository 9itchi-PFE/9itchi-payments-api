package db

import "time"

// Sale maps table sales: a reduction without a code between two days, both included.
type Sale struct {
	ID       string    `db:"id" json:"id"`
	Name     string    `db:"name" json:"name"`
	Percent  int       `db:"percent" json:"percent"`
	StartsOn time.Time `db:"starts_on" json:"starts_on"`
	EndsOn   time.Time `db:"ends_on" json:"ends_on"`
	// PlanIDs are the plans the sale applies to; empty means every paid plan.
	PlanIDs   []string  `db:"plan_ids" json:"plan_ids"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// AppliesTo says whether the sale covers a plan.
func (s *Sale) AppliesTo(planID string) bool {
	if len(s.PlanIDs) == 0 {
		return true
	}
	for _, id := range s.PlanIDs {
		if id == planID {
			return true
		}
	}
	return false
}
