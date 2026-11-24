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
	got := priceAt(p, lastMinute)
	assert.Same(t, p, got, "during the launch the plan is returned as it is")
	assert.Equal(t, 35.0, got.Price)

	got = priceAt(p, lastMinute.Add(time.Minute))
	assert.Equal(t, 50.0, got.Price, "the day after, the plan costs its regular price")
	assert.Nil(t, got.RegularPrice, "and nothing is struck through")
	assert.Nil(t, got.LaunchEndsOn)
	assert.Equal(t, 35.0, p.Price, "the stored plan is not changed")

	// No end, or no regular price to go back to: the price stays.
	noEnd := launchPlan()
	noEnd.LaunchEndsOn = nil
	assert.Equal(t, 35.0, priceAt(noEnd, lastMinute.AddDate(5, 0, 0)).Price)
	noRegular := launchPlan()
	noRegular.RegularPrice = nil
	assert.Equal(t, 35.0, priceAt(noRegular, lastMinute.AddDate(5, 0, 0)).Price)
	assert.Nil(t, priceAt(nil, lastMinute))
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
	priced := PricedPlans(raw, func() time.Time { return now })
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
	svc := NewPlanService(repo, &mocks.SubscriptionRepository{}).(*planService)
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
	svc.now = func() time.Time { return time.Date(2027, 2, 1, 9, 0, 0, 0, time.UTC) }
	public, apiErr := svc.ListPlans(context.Background())
	require.Nil(t, apiErr)
	assert.Equal(t, 50.0, public.Items[0].Price)
	assert.Nil(t, public.Items[0].RegularPrice)
	assert.Empty(t, public.Items[0].LaunchEndsOn)
	admin, apiErr := svc.ListAllPlans(context.Background())
	require.Nil(t, apiErr)
	assert.Equal(t, 35.0, admin.Items[0].Price)
	assert.Equal(t, "2027-01-31", admin.Items[0].LaunchEndsOn)

	svc.now = func() time.Time { return time.Date(2027, 1, 20, 9, 0, 0, 0, time.UTC) }
	public, _ = svc.ListPlans(context.Background())
	assert.Equal(t, 35.0, public.Items[0].Price)
	assert.Equal(t, "2027-01-31", public.Items[0].LaunchEndsOn, "students can be told until when")
}
