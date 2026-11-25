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

// today is the day it is in Tunis, as a date at midnight UTC (the way the days of a plan or a sale are stored).
func today(now time.Time) time.Time {
	y, m, d := now.In(tunis).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// bestSale is the sale that takes the most off a plan among the ones running, nil when none covers it.
func bestSale(p *db.Plan, running []db.Sale) *db.Sale {
	var best *db.Sale
	for i := range running {
		if s := &running[i]; s.AppliesTo(p.ID) && (best == nil || s.Percent > best.Percent) {
			best = s
		}
	}
	return best
}

// priceAt is a plan as it costs at a moment. Once the launch price is over the plan costs its regular price and
// has nothing to strike through. A sale running that day takes its percentage off the usual price of the plan
// (the regular price while a launch price runs, never the launch price): the plan then costs the price of the
// sale, the usual price is struck through and Sale says which sale and until when. A launch price that is already
// lower than the price of the sale stays: a sale never makes a plan cost more. The free plan is never in a sale.
func priceAt(p *db.Plan, now time.Time, running []db.Sale) *db.Plan {
	if p == nil {
		return p
	}
	over := launchOver(p, now)
	out := *p
	out.Sale = nil
	if over {
		out.Price, out.RegularPrice, out.LaunchEndsOn = *p.RegularPrice, nil, nil
	}
	unchanged := func() *db.Plan {
		if !over && p.Sale == nil {
			return p // nothing to change: the plan as it is stored
		}
		return &out
	}
	sale := bestSale(&out, running)
	if sale == nil || out.Price <= 0 {
		return unchanged()
	}
	usual := out.Price
	if out.RegularPrice != nil && *out.RegularPrice > usual {
		usual = *out.RegularPrice
	}
	reduced := float64(millimes(usual)*int64(100-sale.Percent)/100) / 1000
	if reduced >= out.Price {
		return unchanged()
	}
	shown := *sale
	out.Price, out.RegularPrice, out.LaunchEndsOn, out.Sale = reduced, &usual, nil, &shown
	return &out
}

// pricedPlans reads plans at today's price. Writes go through unchanged.
type pricedPlans struct {
	repositories.PlanRepository
	sales repositories.SaleRepository
	now   func() time.Time
}

// PricedPlans wraps a plan repository so that every plan read costs what it costs now, sales included.
func PricedPlans(repo repositories.PlanRepository, sales repositories.SaleRepository, now func() time.Time) repositories.PlanRepository {
	return &pricedPlans{PlanRepository: repo, sales: sales, now: now}
}

func (r *pricedPlans) GetByID(ctx context.Context, id string) (*db.Plan, error) {
	p, err := r.PlanRepository.GetByID(ctx, id)
	if err != nil {
		return p, err
	}
	now := r.now()
	running, err := r.sales.RunningOn(ctx, today(now))
	if err != nil {
		return nil, err
	}
	return priceAt(p, now, running), nil
}

func (r *pricedPlans) GetByName(ctx context.Context, name string) (*db.Plan, error) {
	p, err := r.PlanRepository.GetByName(ctx, name)
	if err != nil {
		return p, err
	}
	now := r.now()
	running, err := r.sales.RunningOn(ctx, today(now))
	if err != nil {
		return nil, err
	}
	return priceAt(p, now, running), nil
}

func (r *pricedPlans) List(ctx context.Context, onlyActive bool) ([]db.Plan, error) {
	plans, err := r.PlanRepository.List(ctx, onlyActive)
	if err != nil {
		return plans, err
	}
	now := r.now()
	running, err := r.sales.RunningOn(ctx, today(now))
	if err != nil {
		return nil, err
	}
	for i := range plans {
		plans[i] = *priceAt(&plans[i], now, running)
	}
	return plans, nil
}
