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

var freePlan = &db.Plan{ID: "f", Name: FreePlanName, Active: true, Quotas: map[string]int{"emails_per_month": 20, "resumes": 3}}

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
	})
	t.Run("free fallback", func(t *testing.T) {
		subs := &mocks.SubscriptionRepository{GetActiveByUserFn: func(context.Context, string) (*db.Subscription, error) {
			return nil, repositories.ErrNotFound
		}}
		res, apiErr := NewPlanService(plans, subs).GetQuotas(context.Background(), userID)
		require.Nil(t, apiErr)
		assert.Equal(t, FreePlanName, res.PlanName)
		assert.Equal(t, 3, res.Quotas["resumes"])
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
