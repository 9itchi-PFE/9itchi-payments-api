package services

import (
	"context"
	"testing"
	"time"

	"9itchi-payments-api/core"
	"9itchi-payments-api/models/api"
	"9itchi-payments-api/models/db"
	"9itchi-payments-api/repositories"
	"9itchi-payments-api/services/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func millimesPtr(v int64) *int64 { return &v }

// checkoutWith runs a checkout of planA at the given price (as priced today) for a student whose current or last
// subscription is sub (active when it ends after fixedNow), and returns the payment opened.
func checkoutWith(t *testing.T, plan *db.Plan, sub *db.Subscription) *db.Payment {
	t.Helper()
	plans := &mocks.PlanRepository{GetByIDFn: func(context.Context, string) (*db.Plan, error) { c := *plan; return &c, nil }}
	subs := &mocks.SubscriptionRepository{
		GetActiveByUserFn: func(context.Context, string) (*db.Subscription, error) {
			if sub != nil && sub.EndsAt.After(fixedNow) {
				return sub, nil
			}
			return nil, repositories.ErrNotFound
		},
		LastEndedFn: func(context.Context, string) (*db.Subscription, error) {
			if sub != nil && !sub.EndsAt.After(fixedNow) {
				return sub, nil
			}
			return nil, repositories.ErrNotFound
		},
	}
	var saved *db.Payment
	pays := &mocks.PaymentRepository{CreateFn: func(_ context.Context, p *db.Payment) (*db.Payment, error) {
		saved = p
		c := *p
		c.ID = payID
		return &c, nil
	}}
	prov := &mocks.PaymentProvider{CreateCheckoutFn: func(_ context.Context, req core.CheckoutRequest) (core.CheckoutResult, error) {
		assert.Equal(t, saved.AmountMillimes, req.AmountMillimes)
		return core.CheckoutResult{Ref: "ref", URL: "https://pay.example/ref"}, nil
	}}
	_, apiErr := newSubSvc(subs, plans, &mocks.PromoRepository{}, pays, prov).Checkout(context.Background(), userID, api.CheckoutRequest{PlanID: planA})
	require.Nil(t, apiErr)
	return saved
}

func TestLaunchPriceIsKeptWhenRenewingInTime(t *testing.T) {
	regular, launch := 50.0, 35.0
	during := &db.Plan{ID: planA, Price: 35, RegularPrice: &regular, LaunchPrice: &launch, DurationDays: 30, Active: true}
	after := &db.Plan{ID: planA, Price: 50, DurationDays: 30, Active: true}
	subscriber := func(endsIn time.Duration, planID string) *db.Subscription {
		return &db.Subscription{ID: "s", PlanID: planID, EndsAt: fixedNow.Add(endsIn), LaunchMillimes: millimesPtr(35000)}
	}

	pay := checkoutWith(t, during, nil)
	assert.EqualValues(t, 35000, pay.AmountMillimes)
	require.NotNil(t, pay.LaunchMillimes, "a payment at the launch price says so, for the subscription")
	assert.EqualValues(t, 35000, *pay.LaunchMillimes)

	for name, sub := range map[string]*db.Subscription{
		"renewed before the end":       subscriber(3*24*time.Hour, planA),
		"renewed 6 days after":         subscriber(-6*24*time.Hour, planA),
		"renewed exactly 7 days after": subscriber(-db.KeepGap, planA),
	} {
		pay = checkoutWith(t, after, sub)
		assert.EqualValues(t, 35000, pay.AmountMillimes, name)
		require.NotNil(t, pay.RegularMillimes, name)
		assert.EqualValues(t, 50000, *pay.RegularMillimes, name+": the receipt strikes today's price through")
		assert.EqualValues(t, 35000, *pay.LaunchMillimes, name)
	}

	for name, sub := range map[string]*db.Subscription{
		"8 days after":        subscriber(-8*24*time.Hour, planA),
		"another plan":        subscriber(3*24*time.Hour, planB),
		"no launch price":     {ID: "s", PlanID: planA, EndsAt: fixedNow.Add(time.Hour)},
		"no subscription yet": nil,
	} {
		pay = checkoutWith(t, after, sub)
		assert.EqualValues(t, 50000, pay.AmountMillimes, name)
		assert.Nil(t, pay.LaunchMillimes, name)
	}

	// A sale cheaper than the kept price wins, and the launch price is still carried for the next renewals.
	sale := 30.0
	inSale := &db.Plan{ID: planA, Price: sale, RegularPrice: &regular, DurationDays: 30, Active: true, Sale: &db.Sale{Name: "Rentrée"}}
	pay = checkoutWith(t, inSale, subscriber(-2*24*time.Hour, planA))
	assert.EqualValues(t, 30000, pay.AmountMillimes)
	assert.Equal(t, "Rentrée", pay.SaleName)
	assert.EqualValues(t, 35000, *pay.LaunchMillimes)
}

func TestTheSubscriptionKeepsTheLaunchPriceOfItsPayment(t *testing.T) {
	var created *db.Subscription
	subs := &mocks.SubscriptionRepository{
		GetActiveByUserFn: func(context.Context, string) (*db.Subscription, error) { return nil, repositories.ErrNotFound },
		CreateFn: func(_ context.Context, s *db.Subscription) (*db.Subscription, error) {
			created = s
			return s, nil
		},
	}
	plans := &mocks.PlanRepository{GetByIDFn: func(context.Context, string) (*db.Plan, error) {
		return &db.Plan{ID: planA, Price: 35, DurationDays: 30}, nil
	}}
	pays := &mocks.PaymentRepository{MarkCompletedFn: func(context.Context, string) (bool, error) { return true, nil }}
	svc := newSubSvc(subs, plans, &mocks.PromoRepository{}, pays, nil)
	applied, subCreated, err := svc.complete(context.Background(), &db.Payment{ID: payID, UserID: userID, PlanID: planA, AmountMillimes: 35000, LaunchMillimes: millimesPtr(35000)})
	require.NoError(t, err)
	assert.True(t, applied)
	assert.True(t, subCreated)
	require.NotNil(t, created.LaunchMillimes)
	assert.EqualValues(t, 35000, *created.LaunchMillimes)
}

func TestMySubscriptionShowsTheKeptPrice(t *testing.T) {
	plans := &mocks.PlanRepository{
		GetByIDFn: func(context.Context, string) (*db.Plan, error) {
			return &db.Plan{ID: planA, Name: "Mensuel", Price: 50}, nil
		},
		GetByNameFn: func(context.Context, string) (*db.Plan, error) { return freePlan, nil },
	}
	active := &db.Subscription{ID: "s", PlanID: planA, Status: "active", EndsAt: fixedNow.Add(24 * time.Hour), LaunchMillimes: millimesPtr(35000)}
	subs := &mocks.SubscriptionRepository{GetActiveByUserFn: func(context.Context, string) (*db.Subscription, error) { return active, nil }}
	res, apiErr := newSubSvc(subs, plans, nil, nil, nil).MySubscription(context.Background(), userID)
	require.Nil(t, apiErr)
	require.NotNil(t, res.KeptPrice)
	assert.Equal(t, 35.0, *res.KeptPrice)
	assert.Equal(t, 35.0, res.Plan.Price, "the renewal is shown at the kept price")
	require.NotNil(t, res.Plan.RegularPrice)
	assert.Equal(t, 50.0, *res.Plan.RegularPrice)

	lapsed := func(ago time.Duration) *api.MySubscriptionResponse {
		last := &db.Subscription{ID: "s", PlanID: planA, Status: "active", EndsAt: fixedNow.Add(-ago), LaunchMillimes: millimesPtr(35000)}
		subs := &mocks.SubscriptionRepository{
			GetActiveByUserFn: func(context.Context, string) (*db.Subscription, error) { return nil, repositories.ErrNotFound },
			LastEndedFn:       func(context.Context, string) (*db.Subscription, error) { return last, nil },
		}
		res, apiErr := newSubSvc(subs, plans, nil, nil, nil).MySubscription(context.Background(), userID)
		require.Nil(t, apiErr)
		return res
	}
	res = lapsed(5 * 24 * time.Hour)
	require.NotNil(t, res.Lapsed)
	require.NotNil(t, res.KeptPrice, "still 2 days to renew at the launch price")
	assert.Equal(t, 35.0, *res.KeptPrice)
	res = lapsed(9 * 24 * time.Hour)
	require.NotNil(t, res.Lapsed)
	assert.Nil(t, res.KeptPrice, "too late: today's price")
}
