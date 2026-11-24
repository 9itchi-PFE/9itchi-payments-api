package services

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"9itchi-payments-api/core"
	"9itchi-payments-api/models/db"
	"9itchi-payments-api/repositories"
	"9itchi-payments-api/services/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const adminID = "00000000-0000-4000-8000-000000000001"

func newGiftSvc(gifts *mocks.GiftRepository, subs *mocks.SubscriptionRepository, plans *mocks.PlanRepository) *giftService {
	return &giftService{gifts: gifts, subs: subs, plans: plans, tx: mocks.Transactor{}, events: &mocks.Publisher{},
		now: func() time.Time { return fixedNow }}
}

func TestGiveMonth(t *testing.T) {
	ctx := context.Background()
	pro := &db.Plan{ID: planA, Name: "Pro", Price: 35, DurationDays: 30, Active: true}
	top := &db.Plan{ID: planB, Name: "Max", Price: 80, DurationDays: 30, Active: true}
	plans := &mocks.PlanRepository{
		GetByIDFn: func(_ context.Context, id string) (*db.Plan, error) {
			switch id {
			case planA:
				return pro, nil
			case planB:
				return top, nil
			case freePlan.ID:
				return freePlan, nil
			}
			return nil, repositories.ErrNotFound
		},
		ListFn: func(context.Context, bool) ([]db.Plan, error) { return []db.Plan{*freePlan, *top, *pro}, nil },
	}
	none := func(context.Context, string) (*db.Subscription, error) { return nil, repositories.ErrNotFound }

	t.Run("a running paid subscription ends 30 days later and keeps its renewal", func(t *testing.T) {
		end := fixedNow.AddDate(0, 0, 10)
		var created, expired, extended bool
		subs := &mocks.SubscriptionRepository{
			GetActiveByUserFn: func(_ context.Context, uid string) (*db.Subscription, error) {
				assert.Equal(t, userID, uid)
				return &db.Subscription{ID: "s1", UserID: userID, PlanID: planB, Status: "active", EndsAt: end}, nil
			},
			AddDaysFn: func(_ context.Context, id string, days int) (*db.Subscription, error) {
				assert.Equal(t, "s1", id)
				return &db.Subscription{ID: "s1", PlanID: planB, EndsAt: end.AddDate(0, 0, days)}, nil
			},
			ExtendFn:       func(context.Context, string, time.Time) error { extended = true; return nil },
			CreateFn:       func(context.Context, *db.Subscription) (*db.Subscription, error) { created = true; return nil, nil },
			ExpireActiveFn: func(context.Context, string) error { expired = true; return nil },
		}
		gifts := &mocks.GiftRepository{}
		svc := newGiftSvc(gifts, subs, plans)
		res, apiErr := svc.GiveMonth(ctx, adminID, userID)
		require.Nil(t, apiErr)
		assert.Equal(t, "Max", res.PlanName, "the plan the student is on, not the cheapest")
		assert.Equal(t, 30, res.Days)
		assert.Equal(t, end, res.PeriodStart)
		assert.Equal(t, end.AddDate(0, 0, 30), res.PeriodEnd)
		assert.Equal(t, adminID, res.GivenBy)
		assert.False(t, created || expired, "the subscription is kept")
		assert.False(t, extended, "Extend would turn a stopped renewal back on")
		require.Len(t, gifts.Created, 1)
		assert.Equal(t, "s1", gifts.Created[0].SubscriptionID)
		assert.Equal(t, []string{userID}, gifts.Locked)
		pub := svc.events.(*mocks.Publisher)
		assert.Equal(t, []string{core.EventSubscriptionGifted}, pub.Events)
		payload := pub.Payloads[0].(map[string]any)
		assert.Equal(t, userID, payload["user_id"])
		assert.Equal(t, "Max", payload["plan_name"])
		assert.Equal(t, end.AddDate(0, 0, 30).Format(time.RFC3339), payload["ends_at"])
	})

	t.Run("no subscription: 30 days of the last plan, that end without a renewal", func(t *testing.T) {
		var made *db.Subscription
		var expired bool
		subs := &mocks.SubscriptionRepository{
			GetActiveByUserFn: none,
			LastEndedFn: func(context.Context, string) (*db.Subscription, error) {
				return &db.Subscription{ID: "old", PlanID: planB}, nil
			},
			ExpireActiveFn: func(context.Context, string) error { expired = true; return nil },
			CreateFn: func(_ context.Context, s *db.Subscription) (*db.Subscription, error) {
				made = s
				out := *s
				out.ID = "s2"
				return &out, nil
			},
		}
		gifts := &mocks.GiftRepository{}
		res, apiErr := newGiftSvc(gifts, subs, plans).GiveMonth(ctx, adminID, userID)
		require.Nil(t, apiErr)
		assert.Equal(t, "Max", res.PlanName)
		assert.True(t, expired, "a lapsed subscription is closed first")
		require.NotNil(t, made)
		assert.Equal(t, fixedNow, made.StartsAt)
		assert.Equal(t, fixedNow.AddDate(0, 0, 30), made.EndsAt)
		assert.Zero(t, made.Amount)
		require.NotNil(t, made.StoppedAt, "nothing is asked when the month ends")
		assert.Equal(t, "s2", gifts.Created[0].SubscriptionID)
		assert.Equal(t, fixedNow, res.PeriodStart)
	})

	t.Run("never subscribed: the cheapest paid plan", func(t *testing.T) {
		subs := &mocks.SubscriptionRepository{
			GetActiveByUserFn: none,
			LastEndedFn:       func(context.Context, string) (*db.Subscription, error) { return nil, repositories.ErrNotFound },
			CreateFn: func(_ context.Context, s *db.Subscription) (*db.Subscription, error) {
				out := *s
				out.ID = "s3"
				return &out, nil
			},
		}
		res, apiErr := newGiftSvc(&mocks.GiftRepository{}, subs, plans).GiveMonth(ctx, adminID, userID)
		require.Nil(t, apiErr)
		assert.Equal(t, "Pro", res.PlanName)
	})

	t.Run("a plan that is no longer sold is not given again", func(t *testing.T) {
		gone := &mocks.PlanRepository{
			GetByIDFn: func(context.Context, string) (*db.Plan, error) {
				return &db.Plan{ID: planB, Name: "Max", Price: 80, Active: false}, nil
			},
			ListFn: plans.ListFn,
		}
		subs := &mocks.SubscriptionRepository{
			GetActiveByUserFn: none,
			LastEndedFn: func(context.Context, string) (*db.Subscription, error) {
				return &db.Subscription{ID: "old", PlanID: planB}, nil
			},
			CreateFn: func(_ context.Context, s *db.Subscription) (*db.Subscription, error) { return s, nil },
		}
		res, apiErr := newGiftSvc(&mocks.GiftRepository{}, subs, gone).GiveMonth(ctx, adminID, userID)
		require.Nil(t, apiErr)
		assert.Equal(t, planA, res.PlanID)
	})

	t.Run("no paid plan on sale", func(t *testing.T) {
		onlyFree := &mocks.PlanRepository{ListFn: func(context.Context, bool) ([]db.Plan, error) { return []db.Plan{*freePlan}, nil }}
		subs := &mocks.SubscriptionRepository{GetActiveByUserFn: none,
			LastEndedFn: func(context.Context, string) (*db.Subscription, error) { return nil, repositories.ErrNotFound }}
		gifts := &mocks.GiftRepository{}
		svc := newGiftSvc(gifts, subs, onlyFree)
		_, apiErr := svc.GiveMonth(ctx, adminID, userID)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusConflict, apiErr.Code)
		assert.Empty(t, gifts.Created)
		assert.Empty(t, svc.events.(*mocks.Publisher).Events)
	})

	t.Run("a second month within the minute is refused", func(t *testing.T) {
		gifts := &mocks.GiftRepository{ListFn: func(_ context.Context, uid string, limit int) ([]db.Gift, error) {
			assert.Equal(t, 1, limit)
			return []db.Gift{{ID: "g0", UserID: uid, CreatedAt: fixedNow.Add(-20 * time.Second)}}, nil
		}}
		var touched bool
		subs := &mocks.SubscriptionRepository{GetActiveByUserFn: func(context.Context, string) (*db.Subscription, error) {
			touched = true
			return nil, repositories.ErrNotFound
		}}
		_, apiErr := newGiftSvc(gifts, subs, plans).GiveMonth(ctx, adminID, userID)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusConflict, apiErr.Code)
		assert.False(t, touched)
		assert.Empty(t, gifts.Created)
	})

	t.Run("an older gift does not block a new one", func(t *testing.T) {
		gifts := &mocks.GiftRepository{ListFn: func(_ context.Context, uid string, _ int) ([]db.Gift, error) {
			return []db.Gift{{ID: "g0", UserID: uid, CreatedAt: fixedNow.Add(-2 * time.Minute)}}, nil
		}}
		subs := &mocks.SubscriptionRepository{
			GetActiveByUserFn: func(context.Context, string) (*db.Subscription, error) {
				return &db.Subscription{ID: "s1", PlanID: planA, EndsAt: fixedNow.Add(time.Hour)}, nil
			},
			AddDaysFn: func(context.Context, string, int) (*db.Subscription, error) {
				return &db.Subscription{ID: "s1", EndsAt: fixedNow.Add(time.Hour).AddDate(0, 0, 30)}, nil
			},
		}
		_, apiErr := newGiftSvc(gifts, subs, plans).GiveMonth(ctx, adminID, userID)
		require.Nil(t, apiErr)
		assert.Len(t, gifts.Created, 1)
	})

	t.Run("a broker that is down does not take the month back", func(t *testing.T) {
		subs := &mocks.SubscriptionRepository{
			GetActiveByUserFn: func(context.Context, string) (*db.Subscription, error) {
				return &db.Subscription{ID: "s1", PlanID: planA, EndsAt: fixedNow.Add(time.Hour)}, nil
			},
			AddDaysFn: func(context.Context, string, int) (*db.Subscription, error) {
				return &db.Subscription{ID: "s1", EndsAt: fixedNow.Add(time.Hour).AddDate(0, 0, 30)}, nil
			},
		}
		svc := newGiftSvc(&mocks.GiftRepository{}, subs, plans)
		svc.events = &mocks.Publisher{Fail: func(string, any) error { return errors.New("broker down") }}
		res, apiErr := svc.GiveMonth(ctx, adminID, userID)
		require.Nil(t, apiErr)
		assert.Equal(t, "Pro", res.PlanName)
	})

	t.Run("database failures", func(t *testing.T) {
		subs := &mocks.SubscriptionRepository{GetActiveByUserFn: func(context.Context, string) (*db.Subscription, error) {
			return nil, errors.New("db down")
		}}
		_, apiErr := newGiftSvc(&mocks.GiftRepository{}, subs, plans).GiveMonth(ctx, adminID, userID)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusInternalServerError, apiErr.Code)

		ok := &mocks.SubscriptionRepository{
			GetActiveByUserFn: func(context.Context, string) (*db.Subscription, error) {
				return &db.Subscription{ID: "s1", PlanID: planA, EndsAt: fixedNow.Add(time.Hour)}, nil
			},
			AddDaysFn: func(context.Context, string, int) (*db.Subscription, error) {
				return &db.Subscription{ID: "s1", EndsAt: fixedNow.Add(time.Hour).AddDate(0, 0, 30)}, nil
			},
		}
		svc := newGiftSvc(&mocks.GiftRepository{CreateErr: errors.New("db down")}, ok, plans)
		_, apiErr = svc.GiveMonth(ctx, adminID, userID)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
		assert.Empty(t, svc.events.(*mocks.Publisher).Events, "nothing is announced when nothing was given")
	})

	t.Run("bad ids", func(t *testing.T) {
		svc := newGiftSvc(&mocks.GiftRepository{}, &mocks.SubscriptionRepository{}, plans)
		_, apiErr := svc.GiveMonth(ctx, adminID, "x")
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusBadRequest, apiErr.Code)
		_, apiErr = svc.GiveMonth(ctx, "", userID)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusBadRequest, apiErr.Code)
	})
}

func TestListGifts(t *testing.T) {
	ctx := context.Background()
	plans := &mocks.PlanRepository{GetByIDFn: func(_ context.Context, id string) (*db.Plan, error) {
		if id == planA {
			return &db.Plan{ID: planA, Name: "Pro"}, nil
		}
		return nil, repositories.ErrNotFound
	}}
	gifts := &mocks.GiftRepository{ListFn: func(_ context.Context, uid string, limit int) ([]db.Gift, error) {
		assert.Equal(t, userID, uid)
		assert.Equal(t, maxGifts, limit)
		return []db.Gift{{ID: "g2", PlanID: planA, Days: 30, GivenBy: adminID}, {ID: "g1", PlanID: planB, Days: 30}}, nil
	}}
	res, apiErr := newGiftSvc(gifts, nil, plans).ListGifts(ctx, userID)
	require.Nil(t, apiErr)
	require.Len(t, res.Items, 2)
	assert.Equal(t, "Pro", res.Items[0].PlanName)
	assert.Equal(t, adminID, res.Items[0].GivenBy)
	assert.Empty(t, res.Items[1].PlanName, "a plan that cannot be read leaves the gift listed")

	empty, apiErr := newGiftSvc(&mocks.GiftRepository{}, nil, plans).ListGifts(ctx, userID)
	require.Nil(t, apiErr)
	assert.NotNil(t, empty.Items)
	assert.Empty(t, empty.Items)

	_, apiErr = newGiftSvc(gifts, nil, plans).ListGifts(ctx, "x")
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.Code)

	down := &mocks.GiftRepository{ListFn: func(context.Context, string, int) ([]db.Gift, error) { return nil, errors.New("db down") }}
	_, apiErr = newGiftSvc(down, nil, plans).ListGifts(ctx, userID)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
}
