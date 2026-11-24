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

func TestCancelSubscription(t *testing.T) {
	ctx := context.Background()
	active := func(context.Context, string) (*db.Subscription, error) {
		return &db.Subscription{ID: "s1", PlanID: planA}, nil
	}
	t.Run("success", func(t *testing.T) {
		cancelled := ""
		subs := &mocks.SubscriptionRepository{GetActiveByUserFn: active, CancelFn: func(_ context.Context, uid string) error { cancelled = uid; return nil }}
		res, apiErr := newSubSvc(subs, activePlan(50), nil, nil, nil).CancelSubscription(ctx, userID)
		require.Nil(t, apiErr)
		assert.Equal(t, "subscription cancelled", res.Message)
		assert.Equal(t, userID, cancelled)
	})
	t.Run("no active subscription", func(t *testing.T) {
		subs := &mocks.SubscriptionRepository{GetActiveByUserFn: func(context.Context, string) (*db.Subscription, error) { return nil, repositories.ErrNotFound }}
		_, apiErr := newSubSvc(subs, activePlan(50), nil, nil, nil).CancelSubscription(ctx, userID)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusNotFound, apiErr.Code)
	})
	t.Run("free plan cannot be cancelled", func(t *testing.T) {
		subs := &mocks.SubscriptionRepository{GetActiveByUserFn: active, CancelFn: func(context.Context, string) error {
			t.Fatal("must not cancel")
			return nil
		}}
		_, apiErr := newSubSvc(subs, activePlan(0), nil, nil, nil).CancelSubscription(ctx, userID)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusNotFound, apiErr.Code)
	})
}
