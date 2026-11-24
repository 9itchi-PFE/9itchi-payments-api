package services

import (
	"context"
	"time"

	"9itchi-payments-api/models"
	"9itchi-payments-api/models/db"
	"9itchi-payments-api/repositories"
)

// tunis is where the days of 9itchi are counted (UTC+1 all year).
var tunis = time.FixedZone("Africa/Tunis", 3600)

// launchEnd reads the last day of a launch price (YYYY-MM-DD, "" for none). A launch price that ends needs a
// regular price to go back to.
func launchEnd(day string, regular *float64) (*time.Time, *models.ApiError) {
	if day == "" {
		return nil, nil
	}
	t, err := time.Parse(time.DateOnly, day)
	if err != nil {
		return nil, badRequest("launch_ends_on must be a day (YYYY-MM-DD)")
	}
	if regular == nil {
		return nil, badRequest("launch_ends_on needs a regular_price")
	}
	return &t, nil
}

// launchOver says whether the launch price of a plan is over: its last day is past in Tunis.
func launchOver(p *db.Plan, now time.Time) bool {
	if p.LaunchEndsOn == nil || p.RegularPrice == nil {
		return false
	}
	y, m, d := now.In(tunis).Date()
	ey, em, ed := p.LaunchEndsOn.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC).After(time.Date(ey, em, ed, 0, 0, 0, 0, time.UTC))
}

// priceAt is a plan as it costs at a moment. Once the launch price is over the plan costs its regular price and
// has nothing to strike through; before that it is returned as it is, with the day the launch price ends.
func priceAt(p *db.Plan, now time.Time) *db.Plan {
	if p == nil || !launchOver(p, now) {
		return p
	}
	out := *p
	out.Price, out.RegularPrice, out.LaunchEndsOn = *p.RegularPrice, nil, nil
	return &out
}

// pricedPlans reads plans at today's price. Writes go through unchanged.
type pricedPlans struct {
	repositories.PlanRepository
	now func() time.Time
}

// PricedPlans wraps a plan repository so that every plan read costs what it costs now.
func PricedPlans(repo repositories.PlanRepository, now func() time.Time) repositories.PlanRepository {
	return &pricedPlans{PlanRepository: repo, now: now}
}

func (r *pricedPlans) GetByID(ctx context.Context, id string) (*db.Plan, error) {
	p, err := r.PlanRepository.GetByID(ctx, id)
	return priceAt(p, r.now()), err
}

func (r *pricedPlans) GetByName(ctx context.Context, name string) (*db.Plan, error) {
	p, err := r.PlanRepository.GetByName(ctx, name)
	return priceAt(p, r.now()), err
}

func (r *pricedPlans) List(ctx context.Context, onlyActive bool) ([]db.Plan, error) {
	plans, err := r.PlanRepository.List(ctx, onlyActive)
	for i := range plans {
		plans[i] = *priceAt(&plans[i], r.now())
	}
	return plans, err
}
