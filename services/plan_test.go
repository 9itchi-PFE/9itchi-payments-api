package services

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"9itchi-payments-api/models"
	"9itchi-payments-api/models/api"
	"9itchi-payments-api/models/db"
	"9itchi-payments-api/repositories"
	"9itchi-payments-api/services/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const userID = "33333333-3333-3333-3333-333333333333"

var freePlan = &db.Plan{ID: "f", Name: FreePlanName, Active: true, Quotas: map[string]int{"emails_per_month": 0, "resumes": 1, "cv_downloads": 1}}

func TestListPlans(t *testing.T) {
	for _, onlyActive := range []bool{true, false} {
		repo := &mocks.PlanRepository{ListFn: func(_ context.Context, a bool) ([]db.Plan, error) {
			assert.Equal(t, onlyActive, a)
			return []db.Plan{{ID: "1", Name: "Pro"}}, nil
		}}
		svc := NewPlanService(repo, &mocks.SubscriptionRepository{})
		var res *api.PlanListResponse
		var apiErr *models.ApiError
		if onlyActive {
			res, apiErr = svc.ListPlans(context.Background())
		} else {
			res, apiErr = svc.ListAllPlans(context.Background())
		}
		require.Nil(t, apiErr)
		require.Len(t, res.Items, 1)
		assert.NotNil(t, res.Items[0].Quotas)
	}
	t.Run("error", func(t *testing.T) {
		repo := &mocks.PlanRepository{ListFn: func(context.Context, bool) ([]db.Plan, error) { return nil, errors.New("x") }}
		_, apiErr := NewPlanService(repo, nil).ListPlans(context.Background())
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
	})
}

func TestCreatePlan(t *testing.T) {
	valid := api.CreatePlanRequest{Name: " Pro ", Price: 10, DurationDays: 30, Quotas: map[string]int{"resumes": 10}}
	tests := []struct {
		name     string
		mutate   func(r *api.CreatePlanRequest)
		repoErr  error
		wantCode int
	}{
		{"ok", func(r *api.CreatePlanRequest) {}, nil, 0},
		{"free price ok", func(r *api.CreatePlanRequest) { r.Price = 0 }, nil, 0},
		{"negative price", func(r *api.CreatePlanRequest) { r.Price = -1 }, nil, http.StatusBadRequest},
		{"zero duration", func(r *api.CreatePlanRequest) { r.DurationDays = 0 }, nil, http.StatusBadRequest},
		{"blank name", func(r *api.CreatePlanRequest) { r.Name = "  " }, nil, http.StatusBadRequest},
		{"negative quota", func(r *api.CreatePlanRequest) { r.Quotas = map[string]int{"a": -1} }, nil, http.StatusBadRequest},
		{"regular price above price", func(r *api.CreatePlanRequest) { r.RegularPrice = floatPtr(15) }, nil, 0},
		{"regular price equal to price", func(r *api.CreatePlanRequest) { r.RegularPrice = floatPtr(10) }, nil, http.StatusBadRequest},
		{"regular price below price", func(r *api.CreatePlanRequest) { r.RegularPrice = floatPtr(5) }, nil, http.StatusBadRequest},
		{"duplicate name", func(r *api.CreatePlanRequest) {}, repositories.ErrConflict, http.StatusConflict},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := valid
			tc.mutate(&req)
			repo := &mocks.PlanRepository{CreateFn: func(_ context.Context, p *db.Plan) (*db.Plan, error) {
				if tc.repoErr != nil {
					return nil, tc.repoErr
				}
				assert.Equal(t, "Pro", p.Name)
				assert.True(t, p.Active, "active defaults to true")
				p.ID = "new"
				return p, nil
			}}
			res, apiErr := NewPlanService(repo, nil).CreatePlan(context.Background(), "admin", req)
			if tc.wantCode != 0 {
				require.NotNil(t, apiErr)
				assert.Equal(t, tc.wantCode, apiErr.Code)
				return
			}
			require.Nil(t, apiErr)
			assert.Equal(t, "new", res.ID)
		})
	}
}

func TestUpdatePlan(t *testing.T) {
	req := api.UpdatePlanRequest{Name: "Pro+", Price: 20, DurationDays: 30, Quotas: map[string]int{"resumes": 20}}
	existing := func(name string) *mocks.PlanRepository {
		return &mocks.PlanRepository{
			GetByIDFn: func(context.Context, string) (*db.Plan, error) {
				return &db.Plan{ID: planA, Name: name, Active: true}, nil
			},
			UpdateFn: func(_ context.Context, _ string, p *db.Plan) (*db.Plan, error) { return p, nil },
		}
	}
	t.Run("ok keeps active when omitted", func(t *testing.T) {
		res, apiErr := NewPlanService(existing("Pro"), nil).UpdatePlan(context.Background(), "a", planA, req)
		require.Nil(t, apiErr)
		assert.True(t, res.Active)
		assert.Equal(t, "Pro+", res.Name)
	})
	t.Run("free cannot be renamed", func(t *testing.T) {
		_, apiErr := NewPlanService(existing(FreePlanName), nil).UpdatePlan(context.Background(), "a", planA, req)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusBadRequest, apiErr.Code)
	})
	t.Run("not found", func(t *testing.T) {
		repo := &mocks.PlanRepository{GetByIDFn: func(context.Context, string) (*db.Plan, error) { return nil, repositories.ErrNotFound }}
		_, apiErr := NewPlanService(repo, nil).UpdatePlan(context.Background(), "a", planA, req)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusNotFound, apiErr.Code)
	})
	t.Run("bad id", func(t *testing.T) {
		_, apiErr := NewPlanService(nil, nil).UpdatePlan(context.Background(), "a", "x", req)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusBadRequest, apiErr.Code)
	})
}

func TestDeactivatePlan(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		repo := &mocks.PlanRepository{
			GetByIDFn:    func(context.Context, string) (*db.Plan, error) { return &db.Plan{ID: planA, Name: "Pro"}, nil },
			DeactivateFn: func(context.Context, string) (*db.Plan, error) { return &db.Plan{ID: planA, Name: "Pro"}, nil },
		}
		res, apiErr := NewPlanService(repo, nil).DeactivatePlan(context.Background(), "a", planA)
		require.Nil(t, apiErr)
		assert.False(t, res.Active)
	})
	t.Run("free refused", func(t *testing.T) {
		repo := &mocks.PlanRepository{GetByIDFn: func(context.Context, string) (*db.Plan, error) { return freePlan, nil }}
		_, apiErr := NewPlanService(repo, nil).DeactivatePlan(context.Background(), "a", planA)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusBadRequest, apiErr.Code)
	})
	t.Run("not found", func(t *testing.T) {
		repo := &mocks.PlanRepository{GetByIDFn: func(context.Context, string) (*db.Plan, error) { return nil, repositories.ErrNotFound }}
		_, apiErr := NewPlanService(repo, nil).DeactivatePlan(context.Background(), "a", planA)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusNotFound, apiErr.Code)
	})
}

func TestGetQuotas(t *testing.T) {
	pro := &db.Plan{ID: planA, Name: "Pro", Quotas: map[string]int{"emails_per_month": 100}}
	plans := &mocks.PlanRepository{
		GetByIDFn:   func(context.Context, string) (*db.Plan, error) { return pro, nil },
		GetByNameFn: func(context.Context, string) (*db.Plan, error) { return freePlan, nil },
	}
	t.Run("active subscription plan", func(t *testing.T) {
		subs := &mocks.SubscriptionRepository{GetActiveByUserFn: func(context.Context, string) (*db.Subscription, error) {
			return &db.Subscription{PlanID: planA, EndsAt: time.Now().Add(time.Hour)}, nil
		}}
		res, apiErr := NewPlanService(plans, subs).GetQuotas(context.Background(), userID)
		require.Nil(t, apiErr)
		assert.Equal(t, map[string]int{"emails_per_month": 100}, res.Quotas)
		assert.NotNil(t, res.PeriodStart)
	})
	t.Run("free fallback", func(t *testing.T) {
		subs := &mocks.SubscriptionRepository{GetActiveByUserFn: func(context.Context, string) (*db.Subscription, error) {
			return nil, repositories.ErrNotFound
		}}
		res, apiErr := NewPlanService(plans, subs).GetQuotas(context.Background(), userID)
		require.Nil(t, apiErr)
		assert.Equal(t, FreePlanName, res.PlanName)
		assert.Equal(t, 1, res.Quotas["resumes"])
		assert.Nil(t, res.PeriodStart, "no subscription, no period")
	})
	t.Run("subscription lookup failure", func(t *testing.T) {
		subs := &mocks.SubscriptionRepository{GetActiveByUserFn: func(context.Context, string) (*db.Subscription, error) {
			return nil, errors.New("db down")
		}}
		_, apiErr := NewPlanService(plans, subs).GetQuotas(context.Background(), userID)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
	})
	t.Run("bad user id", func(t *testing.T) {
		_, apiErr := NewPlanService(plans, nil).GetQuotas(context.Background(), "x")
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusBadRequest, apiErr.Code)
	})
}

func floatPtr(v float64) *float64 { return &v }

func TestPlanRegularPrice(t *testing.T) {
	const id = "44444444-4444-4444-4444-444444444444"
	var saved *db.Plan
	repo := &mocks.PlanRepository{
		CreateFn: func(_ context.Context, p *db.Plan) (*db.Plan, error) { saved = p; return p, nil },
		GetByIDFn: func(context.Context, string) (*db.Plan, error) {
			return &db.Plan{ID: id, Name: "Pro", Active: true}, nil
		},
		UpdateFn: func(_ context.Context, _ string, p *db.Plan) (*db.Plan, error) { saved = p; return p, nil },
	}
	svc := NewPlanService(repo, &mocks.SubscriptionRepository{})
	quotas := map[string]int{"resumes": 10}

	res, apiErr := svc.CreatePlan(context.Background(), userID, api.CreatePlanRequest{Name: "Pro", Price: 35, RegularPrice: floatPtr(50), DurationDays: 30, Quotas: quotas})
	require.Nil(t, apiErr)
	require.NotNil(t, res.RegularPrice)
	assert.Equal(t, 50.0, *res.RegularPrice)
	assert.Equal(t, 50.0, *saved.RegularPrice)

	t.Run("an update without it removes it", func(t *testing.T) {
		res, apiErr := svc.UpdatePlan(context.Background(), userID, id, api.UpdatePlanRequest{Name: "Pro", Price: 35, DurationDays: 30, Quotas: quotas})
		require.Nil(t, apiErr)
		assert.Nil(t, res.RegularPrice)
		assert.Nil(t, saved.RegularPrice)
	})
	t.Run("an update below the price is refused", func(t *testing.T) {
		_, apiErr := svc.UpdatePlan(context.Background(), userID, id, api.UpdatePlanRequest{Name: "Pro", Price: 35, RegularPrice: floatPtr(35), DurationDays: 30, Quotas: quotas})
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusBadRequest, apiErr.Code)
	})
}

func TestPeriodStart(t *testing.T) {
	plan := &db.Plan{DurationDays: 30}
	began := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	day := func(n int) time.Time { return began.AddDate(0, 0, n) }
	assert.Nil(t, periodStart(nil, plan, day(1)))
	first := periodStart(&db.Subscription{StartsAt: began, EndsAt: day(30)}, plan, day(12))
	assert.Equal(t, began, *first)
	third := periodStart(&db.Subscription{StartsAt: began, EndsAt: day(90)}, plan, day(75))
	assert.Equal(t, day(60), *third, "a renewed subscription: its third period")
	// A month given, or a renewal paid ahead: the end is two periods away, the period in progress is the one now is in.
	ahead := periodStart(&db.Subscription{StartsAt: began, EndsAt: day(90)}, plan, day(40))
	assert.Equal(t, day(30), *ahead, "never a start in the future: nothing would be counted")
	early := periodStart(&db.Subscription{StartsAt: began, EndsAt: day(60)}, plan, day(5))
	assert.Equal(t, began, *early)
	short := periodStart(&db.Subscription{StartsAt: began, EndsAt: day(10)}, plan, day(3))
	assert.Equal(t, began, *short, "never before the subscription itself")
	odd := periodStart(&db.Subscription{StartsAt: began, EndsAt: day(90)}, &db.Plan{}, day(40))
	assert.Equal(t, began, *odd)
}
