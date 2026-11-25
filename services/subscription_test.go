package services

import (
	"context"
	"errors"
	"net/http"
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

func TestMySubscription(t *testing.T) {
	plans := &mocks.PlanRepository{
		GetByIDFn:   func(context.Context, string) (*db.Plan, error) { return &db.Plan{ID: planA, Name: "Pro"}, nil },
		GetByNameFn: func(context.Context, string) (*db.Plan, error) { return freePlan, nil },
	}
	t.Run("active subscription with plan", func(t *testing.T) {
		subs := &mocks.SubscriptionRepository{GetActiveByUserFn: func(_ context.Context, uid string) (*db.Subscription, error) {
			assert.Equal(t, userID, uid)
			return &db.Subscription{ID: "s", PlanID: planA, Status: "active", EndsAt: time.Now().Add(time.Hour)}, nil
		}}
		res, apiErr := newSubSvc(subs, plans, nil, nil, nil).MySubscription(context.Background(), userID)
		require.Nil(t, apiErr)
		require.NotNil(t, res.Subscription)
		assert.Equal(t, "s", res.Subscription.ID)
		assert.Equal(t, "Pro", res.Plan.Name)
		assert.False(t, res.IsFree)
	})
	t.Run("free fallback", func(t *testing.T) {
		subs := &mocks.SubscriptionRepository{GetActiveByUserFn: func(context.Context, string) (*db.Subscription, error) {
			return nil, repositories.ErrNotFound
		}}
		res, apiErr := newSubSvc(subs, plans, nil, nil, nil).MySubscription(context.Background(), userID)
		require.Nil(t, apiErr)
		assert.Nil(t, res.Subscription)
		assert.True(t, res.IsFree)
		assert.Equal(t, FreePlanName, res.Plan.Name)
	})
	t.Run("repo failure", func(t *testing.T) {
		subs := &mocks.SubscriptionRepository{GetActiveByUserFn: func(context.Context, string) (*db.Subscription, error) {
			return nil, errors.New("db down")
		}}
		_, apiErr := newSubSvc(subs, plans, nil, nil, nil).MySubscription(context.Background(), userID)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
	})
	t.Run("bad user id", func(t *testing.T) {
		_, apiErr := newSubSvc(nil, nil, nil, nil, nil).MySubscription(context.Background(), "x")
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusBadRequest, apiErr.Code)
	})
}

const payID = "99999999-9999-9999-9999-999999999999"

// newSubSvc wires fakes with a fixed clock; the publisher is reachable as svc.events.
func newSubSvc(subs *mocks.SubscriptionRepository, plans *mocks.PlanRepository, promos *mocks.PromoRepository,
	pays *mocks.PaymentRepository, prov *mocks.PaymentProvider) *subscriptionService {
	factory := core.ProviderFactory{}
	if prov != nil {
		factory[core.FlouciProviderName] = func() core.PaymentProvider { return prov }
	}
	return &subscriptionService{repo: subs, plans: plans, promos: promos, payments: pays, tx: mocks.Transactor{},
		providers: factory, events: &mocks.Publisher{}, now: func() time.Time { return fixedNow }}
}

func eventsOf(s *subscriptionService) []string { return s.events.(*mocks.Publisher).Events }

func TestCheckout(t *testing.T) {
	ctx := context.Background()
	save := &mocks.PromoRepository{GetByCodeFn: func(_ context.Context, code string) (*db.PromoCode, error) {
		assert.Equal(t, "SAVE", code, "code is normalized")
		return &db.PromoCode{ID: "p1", Code: "SAVE", Type: "percentage", Value: 20,
			ValidFrom: fixedNow.Add(-time.Hour), ValidUntil: fixedNow.Add(time.Hour)}, nil
	}}
	okProvider := func() *mocks.PaymentProvider {
		return &mocks.PaymentProvider{CreateCheckoutFn: func(_ context.Context, req core.CheckoutRequest) (core.CheckoutResult, error) {
			assert.Equal(t, payID, req.PaymentID)
			assert.EqualValues(t, 40000, req.AmountMillimes, "20% off 50 TND")
			return core.CheckoutResult{Ref: "ref-1", URL: "https://pay.example/ref-1"}, nil
		}}
	}
	createFn := func(_ context.Context, p *db.Payment) (*db.Payment, error) { c := *p; c.ID = payID; return &c, nil }

	t.Run("success with promo", func(t *testing.T) {
		var stored string
		pays := &mocks.PaymentRepository{CreateFn: func(ctx context.Context, p *db.Payment) (*db.Payment, error) {
			assert.EqualValues(t, 40000, p.AmountMillimes)
			assert.EqualValues(t, 10000, p.DiscountMillimes)
			require.NotNil(t, p.PromoCodeID)
			assert.Equal(t, "p1", *p.PromoCodeID)
			assert.Equal(t, core.FlouciProviderName, p.Provider)
			return createFn(ctx, p)
		}, SetProviderRefFn: func(_ context.Context, id, ref string) error { stored = id + ":" + ref; return nil }}
		svc := newSubSvc(&mocks.SubscriptionRepository{}, activePlan(50), save, pays, okProvider())
		res, apiErr := svc.Checkout(ctx, userID, api.CheckoutRequest{PlanID: planA, PromoCode: " save "})
		require.Nil(t, apiErr)
		assert.Equal(t, &api.CheckoutResponse{PaymentURL: "https://pay.example/ref-1", PaymentID: payID}, res)
		assert.Equal(t, payID+":ref-1", stored)
		assert.Empty(t, eventsOf(svc), "nothing is activated before the webhook")
	})
	t.Run("invalid input", func(t *testing.T) {
		plans := &mocks.PlanRepository{GetByIDFn: func(_ context.Context, id string) (*db.Plan, error) {
			switch id {
			case planA:
				return nil, repositories.ErrNotFound
			default:
				return &db.Plan{ID: id, Price: 5, Active: false}, nil
			}
		}}
		svc := newSubSvc(&mocks.SubscriptionRepository{}, plans, save, &mocks.PaymentRepository{}, okProvider())
		for name, tc := range map[string]struct {
			user, plan string
			want       int
		}{
			"bad user":      {"x", planA, http.StatusBadRequest},
			"bad plan id":   {userID, "nope", http.StatusBadRequest},
			"unknown plan":  {userID, planA, http.StatusNotFound},
			"inactive plan": {userID, planB, http.StatusNotFound},
		} {
			_, apiErr := svc.Checkout(ctx, tc.user, api.CheckoutRequest{PlanID: tc.plan})
			require.NotNil(t, apiErr, name)
			assert.Equal(t, tc.want, apiErr.Code, name)
		}
	})
	t.Run("invalid promo", func(t *testing.T) {
		expired := &mocks.PromoRepository{GetByCodeFn: func(context.Context, string) (*db.PromoCode, error) {
			return &db.PromoCode{ID: "p1", Type: "fixed", Value: 5, ValidFrom: fixedNow.Add(-2 * time.Hour), ValidUntil: fixedNow.Add(-time.Hour)}, nil
		}}
		unknown := &mocks.PromoRepository{GetByCodeFn: func(context.Context, string) (*db.PromoCode, error) { return nil, repositories.ErrNotFound }}
		for _, promos := range []*mocks.PromoRepository{expired, unknown} {
			prov := okProvider()
			_, apiErr := newSubSvc(&mocks.SubscriptionRepository{}, activePlan(50), promos, &mocks.PaymentRepository{}, prov).
				Checkout(ctx, userID, api.CheckoutRequest{PlanID: planA, PromoCode: "SAVE"})
			require.NotNil(t, apiErr)
			assert.Equal(t, http.StatusBadRequest, apiErr.Code)
			assert.Zero(t, prov.CreateCheckoutCall)
		}
	})
	t.Run("provider failure marks the payment failed", func(t *testing.T) {
		var failed string
		pays := &mocks.PaymentRepository{CreateFn: createFn, MarkFailedFn: func(_ context.Context, id string) error { failed = id; return nil }}
		prov := &mocks.PaymentProvider{CreateCheckoutFn: func(context.Context, core.CheckoutRequest) (core.CheckoutResult, error) {
			return core.CheckoutResult{}, errors.New("flouci: status 500")
		}}
		_, apiErr := newSubSvc(&mocks.SubscriptionRepository{}, activePlan(50), save, pays, prov).
			Checkout(ctx, userID, api.CheckoutRequest{PlanID: planA})
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusBadGateway, apiErr.Code)
		assert.Equal(t, "payment provider error", apiErr.Error.Error())
		assert.Equal(t, payID, failed)
	})
	t.Run("provider not configured", func(t *testing.T) {
		prov := okProvider()
		prov.AvailableFn = func() bool { return false }
		created := false
		pays := &mocks.PaymentRepository{CreateFn: func(ctx context.Context, p *db.Payment) (*db.Payment, error) { created = true; return createFn(ctx, p) }}
		_, apiErr := newSubSvc(&mocks.SubscriptionRepository{}, activePlan(50), save, pays, prov).
			Checkout(ctx, userID, api.CheckoutRequest{PlanID: planA})
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusServiceUnavailable, apiErr.Code)
		assert.Equal(t, "payment provider unavailable", apiErr.Error.Error())
		assert.False(t, created)
	})
	t.Run("free plan needs no checkout", func(t *testing.T) {
		pays := &mocks.PaymentRepository{CreateFn: func(context.Context, *db.Payment) (*db.Payment, error) {
			t.Fatal("must not create a payment")
			return nil, nil
		}}
		_, apiErr := newSubSvc(&mocks.SubscriptionRepository{}, activePlan(0), save, pays, okProvider()).
			Checkout(ctx, userID, api.CheckoutRequest{PlanID: planA})
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusBadRequest, apiErr.Code)
	})
	t.Run("100% promo activates without the provider", func(t *testing.T) {
		prov := okProvider()
		full := &mocks.PromoRepository{
			GetByCodeFn: func(context.Context, string) (*db.PromoCode, error) {
				return &db.PromoCode{ID: "p1", Code: "FREE", Type: "percentage", Value: 100,
					ValidFrom: fixedNow.Add(-time.Hour), ValidUntil: fixedNow.Add(time.Hour)}, nil
			},
			IncrementUsageFn: func(context.Context, string) (bool, error) { return true, nil },
		}
		var sub *db.Subscription
		subs := &mocks.SubscriptionRepository{
			GetActiveByUserFn: func(context.Context, string) (*db.Subscription, error) { return nil, repositories.ErrNotFound },
			CreateFn:          func(_ context.Context, s *db.Subscription) (*db.Subscription, error) { sub = s; return s, nil },
		}
		pays := &mocks.PaymentRepository{CreateFn: createFn, MarkCompletedFn: func(context.Context, string) (bool, error) { return true, nil }}
		svc := newSubSvc(subs, activePlan(50), full, pays, prov)
		res, apiErr := svc.Checkout(ctx, userID, api.CheckoutRequest{PlanID: planA, PromoCode: "FREE"})
		require.Nil(t, apiErr)
		assert.Equal(t, &api.CheckoutResponse{PaymentID: payID}, res)
		assert.Zero(t, prov.CreateCheckoutCall)
		require.NotNil(t, sub)
		assert.Equal(t, planA, sub.PlanID)
		assert.Equal(t, []string{core.EventPaymentCompleted, core.EventSubscriptionCreated}, eventsOf(svc))

		full.IncrementUsageFn = func(context.Context, string) (bool, error) { return false, nil }
		svc = newSubSvc(subs, activePlan(50), full, pays, prov)
		_, apiErr = svc.Checkout(ctx, userID, api.CheckoutRequest{PlanID: planA, PromoCode: "FREE"})
		require.NotNil(t, apiErr, "usage limit reached concurrently")
		assert.Equal(t, http.StatusBadRequest, apiErr.Code)
		assert.Empty(t, eventsOf(svc))
	})
}

func TestWebhook(t *testing.T) {
	ctx := context.Background()
	promoID := "p1"
	pending := &db.Payment{ID: payID, UserID: userID, PlanID: planA, PromoCodeID: &promoID, Provider: core.FlouciProviderName,
		Status: "pending", AmountMillimes: 40000}
	signed := func(paid bool) *mocks.PaymentProvider {
		return &mocks.PaymentProvider{
			VerifyWebhookFn: func(_ []byte, sig string) (string, error) {
				if sig != "good" {
					return "", core.ErrInvalidSignature
				}
				return "ref-1", nil
			},
			VerifyPaymentFn: func(_ context.Context, ref string) (bool, error) { assert.Equal(t, "ref-1", ref); return paid, nil },
		}
	}
	lookup := func(p *db.Payment) func(context.Context, string, string) (*db.Payment, error) {
		return func(_ context.Context, provider, ref string) (*db.Payment, error) {
			assert.Equal(t, core.FlouciProviderName, provider)
			assert.Equal(t, "ref-1", ref)
			c := *p
			return &c, nil
		}
	}
	noSub := func(created *[]*db.Subscription) *mocks.SubscriptionRepository {
		return &mocks.SubscriptionRepository{
			GetActiveByUserFn: func(context.Context, string) (*db.Subscription, error) { return nil, repositories.ErrNotFound },
			CreateFn: func(_ context.Context, s *db.Subscription) (*db.Subscription, error) {
				*created = append(*created, s)
				return s, nil
			},
		}
	}

	t.Run("unknown provider", func(t *testing.T) {
		_, apiErr := newSubSvc(nil, nil, nil, nil, signed(true)).Webhook(ctx, "stripe", []byte("{}"), "good")
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusNotFound, apiErr.Code)
	})
	t.Run("bad signature", func(t *testing.T) {
		pays := &mocks.PaymentRepository{MarkCompletedFn: func(context.Context, string) (bool, error) {
			t.Fatal("must not touch payments")
			return false, nil
		}}
		_, apiErr := newSubSvc(nil, nil, nil, pays, signed(true)).Webhook(ctx, "flouci", []byte("{}"), "forged")
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusUnauthorized, apiErr.Code)
		assert.Equal(t, "invalid signature", apiErr.Error.Error())
	})
	t.Run("payment unknown to us", func(t *testing.T) {
		pays := &mocks.PaymentRepository{GetByProviderRefFn: func(context.Context, string, string) (*db.Payment, error) { return nil, repositories.ErrNotFound }}
		_, apiErr := newSubSvc(nil, nil, nil, pays, signed(true)).Webhook(ctx, "flouci", []byte("{}"), "good")
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusNotFound, apiErr.Code)
	})
	t.Run("provider says not paid", func(t *testing.T) {
		pays := &mocks.PaymentRepository{GetByProviderRefFn: lookup(pending), MarkCompletedFn: func(context.Context, string) (bool, error) {
			t.Fatal("must not complete an unverified payment")
			return false, nil
		}}
		svc := newSubSvc(nil, nil, nil, pays, signed(false))
		_, apiErr := svc.Webhook(ctx, "flouci", []byte("{}"), "good")
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusBadRequest, apiErr.Code)
		assert.Empty(t, eventsOf(svc))
	})
	t.Run("verification call fails", func(t *testing.T) {
		prov := signed(true)
		prov.VerifyPaymentFn = func(context.Context, string) (bool, error) { return false, errors.New("boom") }
		pays := &mocks.PaymentRepository{GetByProviderRefFn: lookup(pending)}
		_, apiErr := newSubSvc(nil, nil, nil, pays, prov).Webhook(ctx, "flouci", []byte("{}"), "good")
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusBadGateway, apiErr.Code)
	})
	t.Run("success creates the subscription and records the promo", func(t *testing.T) {
		var created []*db.Subscription
		var promoUsed []string
		pays := &mocks.PaymentRepository{GetByProviderRefFn: lookup(pending),
			MarkCompletedFn: func(context.Context, string) (bool, error) { return true, nil }}
		promos := &mocks.PromoRepository{IncrementUsageFn: func(_ context.Context, id string) (bool, error) { promoUsed = append(promoUsed, id); return true, nil }}
		plans := &mocks.PlanRepository{GetByIDFn: func(context.Context, string) (*db.Plan, error) { return &db.Plan{ID: planA, DurationDays: 30}, nil }}
		svc := newSubSvc(noSub(&created), plans, promos, pays, signed(true))
		res, apiErr := svc.Webhook(ctx, "flouci", []byte("{}"), "good")
		require.Nil(t, apiErr)
		assert.Equal(t, "ok", res.Message)
		require.Len(t, created, 1)
		assert.Equal(t, userID, created[0].UserID)
		assert.Equal(t, fixedNow.AddDate(0, 0, 30), created[0].EndsAt)
		assert.Equal(t, 40.0, created[0].Amount)
		assert.Equal(t, []string{"p1"}, promoUsed)
		assert.Equal(t, []string{core.EventPaymentCompleted, core.EventSubscriptionCreated}, eventsOf(svc))
	})
	t.Run("same plan extends the current subscription", func(t *testing.T) {
		end := fixedNow.Add(24 * time.Hour)
		var extended time.Time
		subs := &mocks.SubscriptionRepository{
			GetActiveByUserFn: func(context.Context, string) (*db.Subscription, error) {
				return &db.Subscription{ID: "s1", PlanID: planA, EndsAt: end}, nil
			},
			ExtendFn: func(_ context.Context, id string, at time.Time) error {
				assert.Equal(t, "s1", id)
				extended = at
				return nil
			},
			CreateFn: func(context.Context, *db.Subscription) (*db.Subscription, error) {
				t.Fatal("must not create a second active subscription")
				return nil, nil
			},
		}
		pays := &mocks.PaymentRepository{GetByProviderRefFn: lookup(pending),
			MarkCompletedFn: func(context.Context, string) (bool, error) { return true, nil }}
		plans := &mocks.PlanRepository{GetByIDFn: func(context.Context, string) (*db.Plan, error) { return &db.Plan{ID: planA, DurationDays: 30}, nil }}
		svc := newSubSvc(subs, plans, &mocks.PromoRepository{}, pays, signed(true))
		_, apiErr := svc.Webhook(ctx, "flouci", []byte("{}"), "good")
		require.Nil(t, apiErr)
		assert.Equal(t, end.AddDate(0, 0, 30), extended)
		assert.Equal(t, []string{core.EventPaymentCompleted}, eventsOf(svc))
	})
	t.Run("replay is a no-op", func(t *testing.T) {
		done := *pending
		done.Status = "completed"
		raced := &mocks.PaymentRepository{GetByProviderRefFn: lookup(pending), MarkCompletedFn: func(context.Context, string) (bool, error) { return false, nil }}
		replayed := &mocks.PaymentRepository{GetByProviderRefFn: lookup(&done)}
		for name, pays := range map[string]*mocks.PaymentRepository{"already completed": replayed, "lost the race": raced} {
			var created []*db.Subscription
			svc := newSubSvc(noSub(&created), &mocks.PlanRepository{}, &mocks.PromoRepository{}, pays, signed(true))
			res, apiErr := svc.Webhook(ctx, "flouci", []byte("{}"), "good")
			require.Nil(t, apiErr, name)
			assert.Equal(t, "ok", res.Message, name)
			assert.Empty(t, created, name)
			assert.Empty(t, eventsOf(svc), name)
		}
	})
}

func TestStopAndResumeSubscription(t *testing.T) {
	ctx := context.Background()
	ends := fixedNow.AddDate(0, 0, 20)
	active := func(context.Context, string) (*db.Subscription, error) {
		return &db.Subscription{ID: "s1", PlanID: planA, EndsAt: ends}, nil
	}
	t.Run("stopping keeps the subscription until it ends", func(t *testing.T) {
		var asked *bool
		subs := &mocks.SubscriptionRepository{GetActiveByUserFn: active, SetRenewalFn: func(_ context.Context, uid string, renews bool) (*db.Subscription, error) {
			assert.Equal(t, userID, uid)
			asked = &renews
			return &db.Subscription{ID: "s1", PlanID: planA, Status: "active", EndsAt: ends, StoppedAt: &fixedNow}, nil
		}}
		res, apiErr := newSubSvc(subs, activePlan(50), nil, nil, nil).CancelSubscription(ctx, userID)
		require.Nil(t, apiErr)
		require.NotNil(t, asked)
		assert.False(t, *asked)
		assert.False(t, res.Subscription.Renews)
		assert.Equal(t, ends, res.Subscription.EndsAt)
		assert.Equal(t, "active", res.Subscription.Status)
		assert.False(t, res.IsFree)
	})
	t.Run("resuming", func(t *testing.T) {
		subs := &mocks.SubscriptionRepository{GetActiveByUserFn: active, SetRenewalFn: func(_ context.Context, _ string, renews bool) (*db.Subscription, error) {
			assert.True(t, renews)
			return &db.Subscription{ID: "s1", PlanID: planA, Status: "active", EndsAt: ends}, nil
		}}
		res, apiErr := newSubSvc(subs, activePlan(50), nil, nil, nil).ResumeSubscription(ctx, userID)
		require.Nil(t, apiErr)
		assert.True(t, res.Subscription.Renews)
	})
	t.Run("no active subscription", func(t *testing.T) {
		subs := &mocks.SubscriptionRepository{GetActiveByUserFn: func(context.Context, string) (*db.Subscription, error) { return nil, repositories.ErrNotFound }}
		_, apiErr := newSubSvc(subs, activePlan(50), nil, nil, nil).CancelSubscription(ctx, userID)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusNotFound, apiErr.Code)
		_, apiErr = newSubSvc(subs, activePlan(50), nil, nil, nil).ResumeSubscription(ctx, userID)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusNotFound, apiErr.Code)
	})
	t.Run("the free plan has nothing to stop", func(t *testing.T) {
		subs := &mocks.SubscriptionRepository{GetActiveByUserFn: active, SetRenewalFn: func(context.Context, string, bool) (*db.Subscription, error) {
			t.Fatal("must not touch the subscription")
			return nil, nil
		}}
		_, apiErr := newSubSvc(subs, activePlan(0), nil, nil, nil).CancelSubscription(ctx, userID)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusNotFound, apiErr.Code)
	})
	t.Run("bad user id", func(t *testing.T) {
		_, apiErr := newSubSvc(nil, nil, nil, nil, nil).CancelSubscription(ctx, "x")
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusBadRequest, apiErr.Code)
	})
}

func TestMySubscriptionLapsed(t *testing.T) {
	ctx := context.Background()
	plans := &mocks.PlanRepository{GetByNameFn: func(context.Context, string) (*db.Plan, error) { return freePlan, nil }}
	none := func(context.Context, string) (*db.Subscription, error) { return nil, repositories.ErrNotFound }
	ended := fixedNow.AddDate(0, 0, -2)
	last := func(sub *db.Subscription, err error) *mocks.SubscriptionRepository {
		return &mocks.SubscriptionRepository{GetActiveByUserFn: none, LastEndedFn: func(context.Context, string) (*db.Subscription, error) { return sub, err }}
	}

	res, apiErr := newSubSvc(last(&db.Subscription{ID: "old", PlanID: planA, EndsAt: ended}, nil), plans, nil, nil, nil).MySubscription(ctx, userID)
	require.Nil(t, apiErr)
	assert.True(t, res.IsFree)
	require.NotNil(t, res.Lapsed, "ended without being stopped: the renewal is due")
	assert.Equal(t, "old", res.Lapsed.ID)
	assert.True(t, res.Lapsed.Renews)

	res, apiErr = newSubSvc(last(&db.Subscription{ID: "old", PlanID: planA, EndsAt: ended, StoppedAt: &ended}, nil), plans, nil, nil, nil).MySubscription(ctx, userID)
	require.Nil(t, apiErr)
	assert.Nil(t, res.Lapsed, "the student stopped it: nothing is asked")

	res, apiErr = newSubSvc(last(nil, repositories.ErrNotFound), plans, nil, nil, nil).MySubscription(ctx, userID)
	require.Nil(t, apiErr)
	assert.Nil(t, res.Lapsed, "never subscribed")

	_, apiErr = newSubSvc(last(nil, errors.New("db down")), plans, nil, nil, nil).MySubscription(ctx, userID)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
}

func TestListPayments(t *testing.T) {
	ctx := context.Background()
	promoID, ref := "p1", "ref-1"
	no := int64(123)
	regular := int64(50000)
	done := time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)
	end := done.AddDate(0, 0, 30)
	pays := &mocks.PaymentRepository{ListCompletedFn: func(_ context.Context, uid string, limit int) ([]db.Payment, error) {
		assert.Equal(t, userID, uid)
		assert.Equal(t, maxReceipts, limit)
		return []db.Payment{
			{ID: payID, UserID: userID, PlanID: planA, PromoCodeID: &promoID, Provider: "flouci", ProviderRef: &ref, Status: "completed",
				AmountMillimes: 30000, DiscountMillimes: 5000, RegularMillimes: &regular, CompletedAt: &done, ReceiptNo: &no, PeriodStart: &done, PeriodEnd: &end},
			{ID: "older", UserID: userID, PlanID: planA, Provider: "flouci", Status: "completed", AmountMillimes: 35000},
		}, nil
	}}
	planCalls := 0
	plans := &mocks.PlanRepository{GetByIDFn: func(context.Context, string) (*db.Plan, error) {
		planCalls++
		return &db.Plan{ID: planA, Name: "Mensuel", DurationDays: 30}, nil
	}}
	promos := &mocks.PromoRepository{GetByIDFn: func(context.Context, string) (*db.PromoCode, error) { return &db.PromoCode{Code: "ENIS2027"}, nil }}

	res, apiErr := newSubSvc(nil, plans, promos, pays, nil).ListPayments(ctx, userID)
	require.Nil(t, apiErr)
	require.Len(t, res.Items, 2)
	got := res.Items[0]
	assert.Equal(t, "2026-000123", got.ReceiptNo)
	assert.Equal(t, "Mensuel", got.PlanName)
	assert.Equal(t, 30, got.DurationDays)
	assert.Equal(t, "ENIS2027", got.PromoCode)
	assert.Equal(t, "ref-1", got.ProviderRef)
	require.NotNil(t, got.RegularMillimes)
	assert.EqualValues(t, 50000, *got.RegularMillimes)
	assert.Nil(t, res.Items[1].RegularMillimes, "a payment opened without a regular price has none")
	assert.EqualValues(t, 30000, got.AmountMillimes)
	assert.Equal(t, &end, got.PeriodEnd)
	assert.Empty(t, res.Items[1].ReceiptNo, "no number, no receipt number")
	assert.Empty(t, res.Items[1].PromoCode)
	assert.Equal(t, 1, planCalls, "one plan lookup for both payments")

	// A plan deleted since leaves the name empty instead of failing the list.
	gone := &mocks.PlanRepository{GetByIDFn: func(context.Context, string) (*db.Plan, error) { return nil, repositories.ErrNotFound }}
	res, apiErr = newSubSvc(nil, gone, promos, pays, nil).ListPayments(ctx, userID)
	require.Nil(t, apiErr)
	assert.Empty(t, res.Items[0].PlanName)

	_, apiErr = newSubSvc(nil, plans, promos, pays, nil).ListPayments(ctx, "x")
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.Code)

	down := &mocks.PaymentRepository{ListCompletedFn: func(context.Context, string, int) ([]db.Payment, error) { return nil, errors.New("db down") }}
	_, apiErr = newSubSvc(nil, plans, promos, down, nil).ListPayments(ctx, userID)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
}

func TestVerifyPayment(t *testing.T) {
	ctx := context.Background()
	ref := "ref-1"
	plans := &mocks.PlanRepository{GetByIDFn: func(context.Context, string) (*db.Plan, error) {
		return &db.Plan{ID: planA, Name: "Mensuel", DurationDays: 30}, nil
	}}
	// store is one payment row; MarkCompleted flips it like the table does.
	store := func(status string) (*mocks.PaymentRepository, *db.Payment) {
		row := &db.Payment{ID: payID, UserID: userID, PlanID: planA, Provider: core.FlouciProviderName, ProviderRef: &ref, Status: status, AmountMillimes: 35000}
		return &mocks.PaymentRepository{
			GetFn: func(_ context.Context, uid, id string) (*db.Payment, error) {
				if uid != userID || id != payID {
					return nil, repositories.ErrNotFound
				}
				c := *row
				return &c, nil
			},
			MarkCompletedFn: func(context.Context, string) (bool, error) {
				if row.Status != "pending" {
					return false, nil
				}
				now, no := fixedNow, int64(7)
				row.Status, row.CompletedAt, row.ReceiptNo = "completed", &now, &no
				return true, nil
			},
			SetPeriodFn: func(_ context.Context, _ string, start, end time.Time) error {
				row.PeriodStart, row.PeriodEnd = &start, &end
				return nil
			},
		}, row
	}
	noSub := &mocks.SubscriptionRepository{
		GetActiveByUserFn: func(context.Context, string) (*db.Subscription, error) { return nil, repositories.ErrNotFound },
		CreateFn:          func(_ context.Context, s *db.Subscription) (*db.Subscription, error) { return s, nil },
	}
	gateway := func(paid bool, err error) *mocks.PaymentProvider {
		return &mocks.PaymentProvider{VerifyPaymentFn: func(_ context.Context, r string) (bool, error) {
			assert.Equal(t, "ref-1", r)
			return paid, err
		}}
	}

	t.Run("paid: completed without waiting for the webhook", func(t *testing.T) {
		pays, row := store("pending")
		svc := newSubSvc(noSub, plans, nil, pays, gateway(true, nil))
		res, apiErr := svc.VerifyPayment(ctx, userID, payID)
		require.Nil(t, apiErr)
		assert.Equal(t, "completed", res.Status)
		assert.Equal(t, "2026-000007", res.ReceiptNo)
		assert.Equal(t, fixedNow, *row.PeriodStart)
		assert.Equal(t, fixedNow.AddDate(0, 0, 30), *row.PeriodEnd)
		assert.Equal(t, []string{core.EventPaymentCompleted, core.EventSubscriptionCreated}, eventsOf(svc))

		// Asking again changes nothing and publishes nothing more.
		res, apiErr = svc.VerifyPayment(ctx, userID, payID)
		require.Nil(t, apiErr)
		assert.Equal(t, "completed", res.Status)
		assert.Len(t, eventsOf(svc), 2)
	})
	t.Run("extending the same plan: the period starts where the current one ends", func(t *testing.T) {
		pays, row := store("pending")
		curEnd := fixedNow.AddDate(0, 0, 10)
		var extended time.Time
		subs := &mocks.SubscriptionRepository{
			GetActiveByUserFn: func(context.Context, string) (*db.Subscription, error) {
				return &db.Subscription{ID: "s", PlanID: planA, EndsAt: curEnd}, nil
			},
			ExtendFn: func(_ context.Context, _ string, end time.Time) error { extended = end; return nil },
		}
		_, apiErr := newSubSvc(subs, plans, nil, pays, gateway(true, nil)).VerifyPayment(ctx, userID, payID)
		require.Nil(t, apiErr)
		assert.Equal(t, curEnd.AddDate(0, 0, 30), extended)
		assert.Equal(t, curEnd, *row.PeriodStart)
		assert.Equal(t, extended, *row.PeriodEnd)
	})
	t.Run("not paid yet or gateway down: still pending", func(t *testing.T) {
		for _, g := range []*mocks.PaymentProvider{gateway(false, nil), gateway(false, core.ErrProviderUnavailable)} {
			pays, row := store("pending")
			res, apiErr := newSubSvc(noSub, plans, nil, pays, g).VerifyPayment(ctx, userID, payID)
			require.Nil(t, apiErr)
			assert.Equal(t, "pending", res.Status)
			assert.Equal(t, "pending", row.Status)
			assert.Empty(t, res.ReceiptNo)
		}
	})
	t.Run("failed payment is reported as is, the gateway is not asked", func(t *testing.T) {
		pays, _ := store("failed")
		prov := &mocks.PaymentProvider{VerifyPaymentFn: func(context.Context, string) (bool, error) {
			t.Fatal("must not ask the gateway")
			return false, nil
		}}
		res, apiErr := newSubSvc(noSub, plans, nil, pays, prov).VerifyPayment(ctx, userID, payID)
		require.Nil(t, apiErr)
		assert.Equal(t, "failed", res.Status)
	})
	t.Run("someone else's payment, bad ids", func(t *testing.T) {
		pays, _ := store("pending")
		svc := newSubSvc(noSub, plans, nil, pays, gateway(true, nil))
		_, apiErr := svc.VerifyPayment(ctx, "11111111-2222-3333-4444-555555555555", payID)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusNotFound, apiErr.Code)
		_, apiErr = svc.VerifyPayment(ctx, userID, "nope")
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusBadRequest, apiErr.Code)
		_, apiErr = svc.VerifyPayment(ctx, "x", payID)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusBadRequest, apiErr.Code)
	})
}

func TestCheckoutKeepsRegularPrice(t *testing.T) {
	regular := 50.0
	plans := &mocks.PlanRepository{GetByIDFn: func(context.Context, string) (*db.Plan, error) {
		return &db.Plan{ID: planA, Name: "Mensuel", Price: 35, RegularPrice: &regular, DurationDays: 30, Active: true}, nil
	}}
	var saved *db.Payment
	pays := &mocks.PaymentRepository{CreateFn: func(_ context.Context, p *db.Payment) (*db.Payment, error) {
		saved = p
		c := *p
		c.ID = payID
		return &c, nil
	}}
	prov := &mocks.PaymentProvider{CreateCheckoutFn: func(_ context.Context, req core.CheckoutRequest) (core.CheckoutResult, error) {
		assert.EqualValues(t, 35000, req.AmountMillimes, "the price charged is the plan's price, never the regular one")
		return core.CheckoutResult{Ref: "ref-1", URL: "https://pay.example/ref-1"}, nil
	}}
	_, apiErr := newSubSvc(&mocks.SubscriptionRepository{}, plans, &mocks.PromoRepository{}, pays, prov).Checkout(context.Background(), userID, api.CheckoutRequest{PlanID: planA})
	require.Nil(t, apiErr)
	require.NotNil(t, saved.RegularMillimes)
	assert.EqualValues(t, 50000, *saved.RegularMillimes)
}

// During a sale the checkout charges the price of the sale, and the payment keeps the usual price and the name
// of the sale for the receipt.
func TestCheckoutInASale(t *testing.T) {
	raw := &mocks.PlanRepository{GetByIDFn: func(context.Context, string) (*db.Plan, error) {
		return &db.Plan{ID: planA, Name: "Mensuel", Price: 50, DurationDays: 30, Active: true}, nil
	}}
	now := time.Date(2027, 2, 10, 9, 0, 0, 0, time.UTC)
	sales := &mocks.SaleRepository{Sales: []db.Sale{{ID: "s1", Name: "Rentrée", Percent: 30,
		StartsOn: time.Date(2027, 2, 1, 0, 0, 0, 0, time.UTC), EndsOn: time.Date(2027, 2, 15, 0, 0, 0, 0, time.UTC)}}}
	var saved *db.Payment
	pays := &mocks.PaymentRepository{CreateFn: func(_ context.Context, p *db.Payment) (*db.Payment, error) {
		saved = p
		c := *p
		c.ID = payID
		return &c, nil
	}}
	var charged int64
	prov := &mocks.PaymentProvider{CreateCheckoutFn: func(_ context.Context, req core.CheckoutRequest) (core.CheckoutResult, error) {
		charged = req.AmountMillimes
		return core.CheckoutResult{Ref: "ref-1", URL: "https://pay.example/ref-1"}, nil
	}}
	svc := newSubSvc(&mocks.SubscriptionRepository{}, raw, &mocks.PromoRepository{}, pays, prov)
	svc.plans = PricedPlans(raw, sales, func() time.Time { return now })
	_, apiErr := svc.Checkout(context.Background(), userID, api.CheckoutRequest{PlanID: planA})
	require.Nil(t, apiErr)
	assert.EqualValues(t, 35000, charged)
	assert.EqualValues(t, 35000, saved.AmountMillimes)
	require.NotNil(t, saved.RegularMillimes)
	assert.EqualValues(t, 50000, *saved.RegularMillimes)
	assert.Equal(t, "Rentrée", saved.SaleName)

	// A code typed during the sale is refused: nothing is opened.
	before := charged
	_, apiErr = svc.Checkout(context.Background(), userID, api.CheckoutRequest{PlanID: planA, PromoCode: "ENIS2027"})
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.Code)
	assert.EqualError(t, apiErr.Error, reasonDuringSale)
	assert.Equal(t, before, charged)

	// The day after the sale: the usual price, and nothing about a sale on the payment.
	now = time.Date(2027, 2, 16, 9, 0, 0, 0, time.UTC)
	_, apiErr = svc.Checkout(context.Background(), userID, api.CheckoutRequest{PlanID: planA})
	require.Nil(t, apiErr)
	assert.EqualValues(t, 50000, charged)
	assert.Nil(t, saved.RegularMillimes)
	assert.Empty(t, saved.SaleName)
}

// The event carries what the receipt says, so the email about it needs no second lookup.
func TestPaymentCompletedEventCarriesTheReceipt(t *testing.T) {
	no := int64(123)
	done := time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)
	end := done.AddDate(0, 0, 30)
	pending := &db.Payment{ID: payID, UserID: userID, PlanID: planA, Provider: "flouci", Status: "pending", AmountMillimes: 30000}
	plans := &mocks.PlanRepository{GetByIDFn: func(context.Context, string) (*db.Plan, error) {
		return &db.Plan{ID: planA, Name: "Mensuel", DurationDays: 30}, nil
	}}

	t.Run("with the completed payment", func(t *testing.T) {
		pays := &mocks.PaymentRepository{GetFn: func(_ context.Context, uid, id string) (*db.Payment, error) {
			assert.Equal(t, userID, uid)
			assert.Equal(t, payID, id)
			c := *pending
			c.Status, c.CompletedAt, c.ReceiptNo, c.PeriodStart, c.PeriodEnd = "completed", &done, &no, &done, &end
			return &c, nil
		}}
		svc := newSubSvc(nil, plans, &mocks.PromoRepository{}, pays, nil)
		svc.publish(context.Background(), pending, true)

		assert.Equal(t, []string{core.EventPaymentCompleted, core.EventSubscriptionCreated}, eventsOf(svc))
		payload := svc.events.(*mocks.Publisher).Payloads[0].(map[string]any)
		assert.Equal(t, userID, payload["user_id"])
		assert.EqualValues(t, 30000, payload["amount_millimes"])
		assert.Equal(t, "2026-000123", payload["receipt_no"])
		assert.Equal(t, "Mensuel", payload["plan_name"])
		assert.Equal(t, 30, payload["duration_days"])
		assert.Equal(t, "2026-11-23T10:00:00Z", payload["period_start"])
		assert.Equal(t, "2026-12-23T10:00:00Z", payload["period_end"])
	})
	t.Run("the event still goes when the payment cannot be read again", func(t *testing.T) {
		pays := &mocks.PaymentRepository{GetFn: func(context.Context, string, string) (*db.Payment, error) {
			return nil, errors.New("database down")
		}}
		svc := newSubSvc(nil, plans, &mocks.PromoRepository{}, pays, nil)
		svc.publish(context.Background(), pending, false)

		assert.Equal(t, []string{core.EventPaymentCompleted}, eventsOf(svc))
		payload := svc.events.(*mocks.Publisher).Payloads[0].(map[string]any)
		assert.Equal(t, payID, payload["payment_id"])
		assert.NotContains(t, payload, "receipt_no")
	})
}

func TestSendRenewalReminders(t *testing.T) {
	ends := time.Date(2026, 12, 24, 9, 0, 0, 0, time.UTC)
	claim := func(_ context.Context, stage string, window time.Duration, limit int) ([]db.RenewalReminder, error) {
		assert.Equal(t, 7*24*time.Hour, window, "the same 7 days as the banner of the app")
		assert.Equal(t, reminderBatch, limit)
		switch stage {
		case db.ReminderDue:
			return []db.RenewalReminder{
				{SubscriptionID: "s1", UserID: userID, PlanID: planA, PlanName: "Mensuel", PlanPrice: 35, EndsAt: ends},
				{SubscriptionID: "s2", UserID: "u2", PlanID: planA, PlanName: "Mensuel", PlanPrice: 35, EndsAt: ends},
			}, nil
		case db.ReminderLapsed:
			return []db.RenewalReminder{{SubscriptionID: "s3", UserID: "u3", PlanID: planA, PlanName: "Mensuel", PlanPrice: 35, EndsAt: ends.AddDate(0, -1, 0)}}, nil
		}
		t.Fatalf("unexpected stage %q", stage)
		return nil, nil
	}

	t.Run("one event per subscription, saying what the reminder needs", func(t *testing.T) {
		svc := newSubSvc(&mocks.SubscriptionRepository{ClaimRemindersFn: claim}, nil, nil, nil, nil)
		res, apiErr := svc.SendRenewalReminders(context.Background())
		require.Nil(t, apiErr)
		assert.Equal(t, &api.RenewalRemindersResponse{Due: 2, Lapsed: 1}, res)

		pub := svc.events.(*mocks.Publisher)
		assert.Equal(t, []string{core.EventSubscriptionRenewalDue, core.EventSubscriptionRenewalDue, core.EventSubscriptionRenewalDue}, pub.Events)
		first := pub.Payloads[0].(map[string]any)
		assert.Equal(t, map[string]any{
			"user_id": userID, "subscription_id": "s1", "plan_id": planA, "plan_name": "Mensuel",
			"price_millimes": int64(35000), "ends_at": "2026-12-24T09:00:00Z", "stage": "due",
		}, first)
		assert.Equal(t, "lapsed", pub.Payloads[2].(map[string]any)["stage"])
	})

	t.Run("a reminder that could not be raised is given back", func(t *testing.T) {
		var released []string
		repo := &mocks.SubscriptionRepository{ClaimRemindersFn: claim, ReleaseReminderFn: func(_ context.Context, id, stage string) error {
			released = append(released, id+":"+stage)
			return nil
		}}
		svc := newSubSvc(repo, nil, nil, nil, nil)
		svc.events.(*mocks.Publisher).Fail = func(_ string, payload any) error {
			if payload.(map[string]any)["subscription_id"] == "s2" {
				return errors.New("rabbitmq: broker unreachable")
			}
			return nil
		}
		res, apiErr := svc.SendRenewalReminders(context.Background())
		require.Nil(t, apiErr)
		assert.Equal(t, &api.RenewalRemindersResponse{Due: 1, Lapsed: 1, Failed: 1}, res)
		assert.Equal(t, []string{"s2:due"}, released)
	})

	t.Run("nothing to remind", func(t *testing.T) {
		svc := newSubSvc(&mocks.SubscriptionRepository{}, nil, nil, nil, nil)
		res, apiErr := svc.SendRenewalReminders(context.Background())
		require.Nil(t, apiErr)
		assert.Equal(t, &api.RenewalRemindersResponse{}, res)
		assert.Empty(t, eventsOf(svc))
	})

	t.Run("database down", func(t *testing.T) {
		repo := &mocks.SubscriptionRepository{ClaimRemindersFn: func(context.Context, string, time.Duration, int) ([]db.RenewalReminder, error) {
			return nil, errors.New("database down")
		}}
		_, apiErr := newSubSvc(repo, nil, nil, nil, nil).SendRenewalReminders(context.Background())
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
	})
}
