package services

import (
	"context"
	"net/http"
	"testing"
	"time"

	"9itchi-payments-api/models/api"
	"9itchi-payments-api/models/db"
	"9itchi-payments-api/services/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func launchPlan() *db.Plan {
	regular := 50.0
	last := time.Date(2027, 1, 31, 0, 0, 0, 0, time.UTC)
	return &db.Plan{ID: "p1", Name: "Abonnement étudiant", Price: 35, RegularPrice: &regular, LaunchEndsOn: &last, DurationDays: 30, Active: true}
}

func TestPriceAt(t *testing.T) {
	p := launchPlan()
	// 22:59 UTC on the last day is still the last day in Tunis; one minute later it is February there.
	lastMinute := time.Date(2027, 1, 31, 22, 59, 0, 0, time.UTC)
	got := priceAt(p, lastMinute, nil)
	assert.Equal(t, 35.0, got.Price, "during the launch the plan costs its launch price")
	assert.Equal(t, 50.0, *got.RegularPrice)
	require.NotNil(t, got.LaunchPrice, "the launch price a subscriber keeps")
	assert.Equal(t, 35.0, *got.LaunchPrice)
	assert.Nil(t, p.LaunchPrice, "the stored plan is not changed")

	got = priceAt(p, lastMinute.Add(time.Minute), nil)
	assert.Equal(t, 50.0, got.Price, "the day after, the plan costs its regular price")
	assert.Nil(t, got.RegularPrice, "and nothing is struck through")
	assert.Nil(t, got.LaunchEndsOn)
	assert.Equal(t, 35.0, p.Price, "the stored plan is not changed")

	// No end, or no regular price to go back to: the price stays.
	noEnd := launchPlan()
	noEnd.LaunchEndsOn = nil
	assert.Equal(t, 35.0, priceAt(noEnd, lastMinute.AddDate(5, 0, 0), nil).Price)
	noRegular := launchPlan()
	noRegular.RegularPrice = nil
	assert.Equal(t, 35.0, priceAt(noRegular, lastMinute.AddDate(5, 0, 0), nil).Price)
	assert.Nil(t, priceAt(nil, lastMinute, nil))
}

func TestPricedPlansReadAtTodaysPrice(t *testing.T) {
	now := time.Date(2027, 2, 1, 9, 0, 0, 0, time.UTC)
	raw := &mocks.PlanRepository{
		GetByIDFn:   func(context.Context, string) (*db.Plan, error) { return launchPlan(), nil },
		GetByNameFn: func(context.Context, string) (*db.Plan, error) { return launchPlan(), nil },
		ListFn: func(context.Context, bool) ([]db.Plan, error) {
			return []db.Plan{*launchPlan(), {Name: FreePlanName}}, nil
		},
	}
	priced := PricedPlans(raw, &mocks.SaleRepository{}, func() time.Time { return now })
	p, err := priced.GetByID(context.Background(), "p1")
	require.NoError(t, err)
	assert.Equal(t, 50.0, p.Price)
	p, err = priced.GetByName(context.Background(), "x")
	require.NoError(t, err)
	assert.Equal(t, 50.0, p.Price)
	list, err := priced.List(context.Background(), true)
	require.NoError(t, err)
	assert.Equal(t, 50.0, list[0].Price)
	assert.Equal(t, 0.0, list[1].Price)

	now = time.Date(2027, 1, 15, 9, 0, 0, 0, time.UTC)
	p, _ = priced.GetByID(context.Background(), "p1")
	assert.Equal(t, 35.0, p.Price)
	require.NotNil(t, p.RegularPrice)
}

func TestLaunchEndOnPlans(t *testing.T) {
	var saved *db.Plan
	repo := &mocks.PlanRepository{
		GetByIDFn: func(context.Context, string) (*db.Plan, error) { return launchPlan(), nil },
		UpdateFn: func(_ context.Context, _ string, p *db.Plan) (*db.Plan, error) {
			saved = p
			return p, nil
		},
		ListFn: func(context.Context, bool) ([]db.Plan, error) { return []db.Plan{*launchPlan()}, nil },
	}
	now := time.Date(2027, 1, 15, 9, 0, 0, 0, time.UTC)
	svc := NewPlanService(repo, PricedPlans(repo, &mocks.SaleRepository{}, func() time.Time { return now }), &mocks.SubscriptionRepository{}).(*planService)
	regular := 50.0
	req := api.UpdatePlanRequest{Name: "Abonnement étudiant", Price: 35, RegularPrice: &regular, LaunchEndsOn: "2027-03-31", DurationDays: 30, Quotas: map[string]int{}}

	resp, apiErr := svc.UpdatePlan(context.Background(), "admin", "44444444-4444-4444-8444-444444444444", req)
	require.Nil(t, apiErr)
	assert.Equal(t, "2027-03-31", resp.LaunchEndsOn)
	assert.Equal(t, time.Date(2027, 3, 31, 0, 0, 0, 0, time.UTC), *saved.LaunchEndsOn)

	// Left out: the launch price has no end any more.
	req.LaunchEndsOn = ""
	resp, apiErr = svc.UpdatePlan(context.Background(), "admin", "44444444-4444-4444-8444-444444444444", req)
	require.Nil(t, apiErr)
	assert.Empty(t, resp.LaunchEndsOn)
	assert.Nil(t, saved.LaunchEndsOn)

	// An end needs a regular price to go back to, and a real day.
	req.LaunchEndsOn, req.RegularPrice = "2027-03-31", nil
	_, apiErr = svc.UpdatePlan(context.Background(), "admin", "44444444-4444-4444-8444-444444444444", req)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.Code)
	assert.EqualError(t, apiErr.Error, "launch_ends_on needs a regular_price")
	req.LaunchEndsOn, req.RegularPrice = "31/03/2027", &regular
	_, apiErr = svc.UpdatePlan(context.Background(), "admin", "44444444-4444-4444-8444-444444444444", req)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.Code)

	// Students see today's price; the admin sees what was typed.
	now = time.Date(2027, 2, 1, 9, 0, 0, 0, time.UTC)
	public, apiErr := svc.ListPlans(context.Background())
	require.Nil(t, apiErr)
	assert.Equal(t, 50.0, public.Items[0].Price)
	assert.Nil(t, public.Items[0].RegularPrice)
	assert.Empty(t, public.Items[0].LaunchEndsOn)
	admin, apiErr := svc.ListAllPlans(context.Background())
	require.Nil(t, apiErr)
	assert.Equal(t, 35.0, admin.Items[0].Price)
	assert.Equal(t, "2027-01-31", admin.Items[0].LaunchEndsOn)

	now = time.Date(2027, 1, 20, 9, 0, 0, 0, time.UTC)
	public, _ = svc.ListPlans(context.Background())
	assert.Equal(t, 35.0, public.Items[0].Price)
	assert.Equal(t, "2027-01-31", public.Items[0].LaunchEndsOn, "students can be told until when")
}

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func TestPriceAtInASale(t *testing.T) {
	rentree := db.Sale{ID: "s1", Name: "Rentrée", Percent: 30, StartsOn: day(2027, 2, 1), EndsOn: day(2027, 2, 15)}
	now := time.Date(2027, 2, 10, 9, 0, 0, 0, time.UTC)

	// A plan with one price: the sale price is charged, the usual one is struck through.
	plain := &db.Plan{ID: "p1", Name: "Mensuel", Price: 50, DurationDays: 30, Active: true}
	got := priceAt(plain, now, []db.Sale{rentree})
	assert.Equal(t, 35.0, got.Price)
	require.NotNil(t, got.RegularPrice)
	assert.Equal(t, 50.0, *got.RegularPrice)
	require.NotNil(t, got.Sale)
	assert.Equal(t, "Rentrée", got.Sale.Name)
	assert.Equal(t, day(2027, 2, 15), got.Sale.EndsOn)
	assert.Equal(t, 50.0, plain.Price, "the stored plan is not changed")
	assert.Nil(t, plain.Sale)

	// No sale running: the plan as it is stored.
	assert.Same(t, plain, priceAt(plain, now, nil))

	// The best of two sales wins; a sale for other plans does not count.
	flash := db.Sale{ID: "s2", Name: "Flash", Percent: 40, StartsOn: day(2027, 2, 10), EndsOn: day(2027, 2, 10)}
	others := db.Sale{ID: "s3", Name: "Autre", Percent: 80, StartsOn: day(2027, 2, 1), EndsOn: day(2027, 2, 28), PlanIDs: []string{"p9"}}
	got = priceAt(plain, now, []db.Sale{rentree, flash, others})
	assert.Equal(t, 30.0, got.Price)
	assert.Equal(t, "Flash", got.Sale.Name)

	// With a launch price the percentage is taken off the regular price, never the launch price.
	launch := launchPlan() // 35 instead of 50 until 31 January 2027
	january := db.Sale{ID: "s4", Name: "Janvier", Percent: 40, StartsOn: day(2027, 1, 20), EndsOn: day(2027, 2, 5)}
	got = priceAt(launch, time.Date(2027, 1, 25, 9, 0, 0, 0, time.UTC), []db.Sale{january})
	assert.Equal(t, 30.0, got.Price, "40 % off 50, not off 35")
	assert.Equal(t, 50.0, *got.RegularPrice)
	assert.Nil(t, got.LaunchEndsOn)
	assert.Equal(t, day(2027, 2, 5), got.Sale.EndsOn)
	// After the launch: the same price until the end of the sale.
	got = priceAt(launch, time.Date(2027, 2, 2, 9, 0, 0, 0, time.UTC), []db.Sale{january})
	assert.Equal(t, 30.0, got.Price)
	assert.Equal(t, 50.0, *got.RegularPrice)

	// A sale that would cost more than the launch price changes nothing: 20 % off 50 is 40, the plan stays at 35.
	small := db.Sale{ID: "s5", Name: "Petite", Percent: 20, StartsOn: day(2027, 1, 20), EndsOn: day(2027, 2, 5)}
	got = priceAt(launch, time.Date(2027, 1, 25, 9, 0, 0, 0, time.UTC), []db.Sale{small})
	assert.Equal(t, 35.0, got.Price)
	assert.Nil(t, got.Sale)
	// Once the launch price is over, that sale applies.
	got = priceAt(launch, time.Date(2027, 2, 2, 9, 0, 0, 0, time.UTC), []db.Sale{small})
	assert.Equal(t, 40.0, got.Price)
	assert.Equal(t, "Petite", got.Sale.Name)

	// Never below a millime, never the free plan.
	odd := &db.Plan{ID: "p2", Price: 49.99}
	assert.Equal(t, 33.493, priceAt(odd, now, []db.Sale{{Percent: 33, StartsOn: day(2027, 2, 1), EndsOn: day(2027, 2, 28)}}).Price)
	free := &db.Plan{ID: "p0", Name: FreePlanName}
	assert.Same(t, free, priceAt(free, now, []db.Sale{rentree}))
}

func TestPricedPlansInASale(t *testing.T) {
	now := time.Date(2027, 1, 31, 23, 30, 0, 0, time.UTC) // already 1 February in Tunis
	raw := &mocks.PlanRepository{
		GetByIDFn: func(context.Context, string) (*db.Plan, error) {
			return &db.Plan{ID: "p1", Name: "Mensuel", Price: 50, DurationDays: 30, Active: true}, nil
		},
		ListFn: func(context.Context, bool) ([]db.Plan, error) {
			return []db.Plan{{Name: FreePlanName}, {ID: "p1", Name: "Mensuel", Price: 50, Active: true}}, nil
		},
	}
	sales := &mocks.SaleRepository{Sales: []db.Sale{{ID: "s1", Name: "Rentrée", Percent: 30, StartsOn: day(2027, 2, 1), EndsOn: day(2027, 2, 15)}}}
	priced := PricedPlans(raw, sales, func() time.Time { return now })

	p, err := priced.GetByID(context.Background(), "p1")
	require.NoError(t, err)
	assert.Equal(t, 35.0, p.Price, "the sale starts with the day in Tunis")
	list, err := priced.List(context.Background(), true)
	require.NoError(t, err)
	assert.Equal(t, 0.0, list[0].Price)
	assert.Nil(t, list[0].Sale)
	assert.Equal(t, 35.0, list[1].Price)

	// What students are shown says which sale and until when; the admin still reads what was typed.
	svc := NewPlanService(raw, priced, &mocks.SubscriptionRepository{})
	public, apiErr := svc.ListPlans(context.Background())
	require.Nil(t, apiErr)
	require.NotNil(t, public.Items[1].Sale)
	assert.Equal(t, api.PlanSale{Name: "Rentrée", Percent: 30, EndsOn: "2027-02-15"}, *public.Items[1].Sale)
	assert.Equal(t, 50.0, *public.Items[1].RegularPrice)
	admin, apiErr := svc.ListAllPlans(context.Background())
	require.Nil(t, apiErr)
	assert.Equal(t, 50.0, admin.Items[1].Price)
	assert.Nil(t, admin.Items[1].Sale)

	// The day after the last day, and the day before the first.
	now = time.Date(2027, 2, 15, 23, 0, 0, 0, time.UTC)
	p, _ = priced.GetByID(context.Background(), "p1")
	assert.Equal(t, 50.0, p.Price)
	now = time.Date(2027, 1, 31, 22, 59, 0, 0, time.UTC)
	p, _ = priced.GetByID(context.Background(), "p1")
	assert.Equal(t, 50.0, p.Price)

	// A price is never guessed when the sales cannot be read.
	sales.Err = assert.AnError
	_, err = priced.GetByID(context.Background(), "p1")
	assert.Error(t, err)
	_, err = priced.List(context.Background(), true)
	assert.Error(t, err)
}
