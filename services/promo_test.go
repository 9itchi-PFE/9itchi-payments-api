package services

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"9itchi-payments-api/models/api"
	"9itchi-payments-api/models/db"
	"9itchi-payments-api/repositories"
	"9itchi-payments-api/services/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	planA = "11111111-1111-1111-1111-111111111111"
	planB = "22222222-2222-2222-2222-222222222222"
)

var fixedNow = time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)

func newPromoSvc(plans *mocks.PlanRepository, promos *mocks.PromoRepository) *promoService {
	return &promoService{repo: promos, plans: plans, now: func() time.Time { return fixedNow }}
}

func activePlan(price float64) *mocks.PlanRepository {
	return &mocks.PlanRepository{GetByIDFn: func(_ context.Context, id string) (*db.Plan, error) {
		return &db.Plan{ID: id, Name: "Pro", Price: price, Active: true}, nil
	}}
}

func TestValidatePromo(t *testing.T) {
	base := db.PromoCode{
		ID: "p1", Code: "SAVE", Type: "percentage", Value: 20,
		ValidFrom: fixedNow.Add(-time.Hour), ValidUntil: fixedNow.Add(time.Hour),
	}
	tests := []struct {
		name     string
		mutate   func(p *db.PromoCode)
		price    float64
		wantOK   bool
		discount float64
		final    float64
		reason   string
	}{
		{"percentage", func(p *db.PromoCode) {}, 50, true, 10, 40, ""},
		{"fixed", func(p *db.PromoCode) { p.Type, p.Value = "fixed", 15 }, 50, true, 15, 35, ""},
		{"fixed above price capped to zero", func(p *db.PromoCode) { p.Type, p.Value = "fixed", 99 }, 50, true, 50, 0, ""},
		{"100 percent", func(p *db.PromoCode) { p.Value = 100 }, 50, true, 50, 0, ""},
		{"rounding", func(p *db.PromoCode) { p.Value = 33 }, 9.99, true, 3.30, 6.69, ""},
		{"eligible plan", func(p *db.PromoCode) { p.PlanIDs = []string{planA} }, 50, true, 10, 40, ""},
		{"not eligible plan", func(p *db.PromoCode) { p.PlanIDs = []string{planB} }, 50, false, 0, 50, "promo code does not apply to this plan"},
		{"not yet valid", func(p *db.PromoCode) { p.ValidFrom = fixedNow.Add(time.Minute) }, 50, false, 0, 50, "promo code is not yet valid"},
		{"expired", func(p *db.PromoCode) { p.ValidUntil = fixedNow.Add(-time.Minute) }, 50, false, 0, 50, "promo code has expired"},
		{"usage exhausted", func(p *db.PromoCode) { p.UsageLimit, p.UsedCount = 5, 5 }, 50, false, 0, 50, "promo code usage limit reached"},
		{"usage left", func(p *db.PromoCode) { p.UsageLimit, p.UsedCount = 5, 4 }, 50, true, 10, 40, ""},
		{"unlimited usage", func(p *db.PromoCode) { p.UsageLimit, p.UsedCount = 0, 1000 }, 50, true, 10, 40, ""},
		{"free plan", func(p *db.PromoCode) {}, 0, true, 0, 0, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			promo := base
			tc.mutate(&promo)
			svc := newPromoSvc(activePlan(tc.price), &mocks.PromoRepository{
				GetByCodeFn: func(_ context.Context, code string) (*db.PromoCode, error) {
					assert.Equal(t, "SAVE", code, "code is normalised to upper-case")
					return &promo, nil
				},
			})
			res, apiErr := svc.ValidatePromo(context.Background(), "u", api.ValidatePromoRequest{Code: " save ", PlanID: planA})
			require.Nil(t, apiErr)
			assert.Equal(t, tc.wantOK, res.Valid)
			assert.InDelta(t, tc.discount, res.DiscountAmount, 0.001)
			assert.InDelta(t, tc.final, res.FinalPrice, 0.001)
			assert.Equal(t, tc.reason, res.Reason)
			assert.GreaterOrEqual(t, res.FinalPrice, 0.0)
		})
	}
}

func TestValidatePromoErrors(t *testing.T) {
	t.Run("unknown code is a valid:false answer", func(t *testing.T) {
		svc := newPromoSvc(activePlan(50), &mocks.PromoRepository{
			GetByCodeFn: func(context.Context, string) (*db.PromoCode, error) { return nil, repositories.ErrNotFound },
		})
		res, apiErr := svc.ValidatePromo(context.Background(), "u", api.ValidatePromoRequest{Code: "X", PlanID: planA})
		require.Nil(t, apiErr)
		assert.False(t, res.Valid)
		assert.Equal(t, 50.0, res.FinalPrice)
	})
	t.Run("repo failure is 500 and hidden", func(t *testing.T) {
		svc := newPromoSvc(activePlan(50), &mocks.PromoRepository{
			GetByCodeFn: func(context.Context, string) (*db.PromoCode, error) { return nil, errors.New("pg: secret detail") },
		})
		_, apiErr := svc.ValidatePromo(context.Background(), "u", api.ValidatePromoRequest{Code: "X", PlanID: planA})
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
		assert.NotContains(t, apiErr.Error.Error(), "secret")
	})
	t.Run("plan not found", func(t *testing.T) {
		plans := &mocks.PlanRepository{GetByIDFn: func(context.Context, string) (*db.Plan, error) { return nil, repositories.ErrNotFound }}
		_, apiErr := newPromoSvc(plans, &mocks.PromoRepository{}).ValidatePromo(context.Background(), "u", api.ValidatePromoRequest{Code: "X", PlanID: planA})
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusNotFound, apiErr.Code)
	})
	t.Run("inactive plan is 404", func(t *testing.T) {
		plans := &mocks.PlanRepository{GetByIDFn: func(context.Context, string) (*db.Plan, error) { return &db.Plan{ID: planA}, nil }}
		_, apiErr := newPromoSvc(plans, &mocks.PromoRepository{}).ValidatePromo(context.Background(), "u", api.ValidatePromoRequest{Code: "X", PlanID: planA})
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusNotFound, apiErr.Code)
	})
	t.Run("bad plan id", func(t *testing.T) {
		_, apiErr := newPromoSvc(nil, nil).ValidatePromo(context.Background(), "u", api.ValidatePromoRequest{Code: "X", PlanID: "nope"})
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusBadRequest, apiErr.Code)
	})
}

func TestCreatePromo(t *testing.T) {
	from, until := fixedNow, fixedNow.Add(24*time.Hour)
	valid := api.CreatePromoRequest{Code: "welcome10", Type: "percentage", Value: 10, ValidFrom: from, ValidUntil: until}

	tests := []struct {
		name     string
		mutate   func(r *api.CreatePromoRequest)
		repoErr  error
		wantCode int
	}{
		{"ok", func(r *api.CreatePromoRequest) {}, nil, 0},
		{"percentage over 100", func(r *api.CreatePromoRequest) { r.Value = 101 }, nil, http.StatusBadRequest},
		{"percentage under 1", func(r *api.CreatePromoRequest) { r.Value = 0.5 }, nil, http.StatusBadRequest},
		{"bad type", func(r *api.CreatePromoRequest) { r.Type = "weird" }, nil, http.StatusBadRequest},
		{"from after until", func(r *api.CreatePromoRequest) { r.ValidFrom = until.Add(time.Hour) }, nil, http.StatusBadRequest},
		{"from equals until", func(r *api.CreatePromoRequest) { r.ValidFrom = until }, nil, http.StatusBadRequest},
		{"negative usage", func(r *api.CreatePromoRequest) { r.UsageLimit = -1 }, nil, http.StatusBadRequest},
		{"bad plan id", func(r *api.CreatePromoRequest) { r.PlanIDs = []string{"x"} }, nil, http.StatusBadRequest},
		{"duplicate code", func(r *api.CreatePromoRequest) {}, repositories.ErrConflict, http.StatusConflict},
		{"db failure", func(r *api.CreatePromoRequest) {}, errors.New("boom"), http.StatusInternalServerError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := valid
			tc.mutate(&req)
			var saved *db.PromoCode
			svc := newPromoSvc(nil, &mocks.PromoRepository{CreateFn: func(_ context.Context, p *db.PromoCode) (*db.PromoCode, error) {
				if tc.repoErr != nil {
					return nil, tc.repoErr
				}
				saved = p
				return p, nil
			}})
			res, apiErr := svc.CreatePromo(context.Background(), "admin", req)
			if tc.wantCode != 0 {
				require.NotNil(t, apiErr)
				assert.Equal(t, tc.wantCode, apiErr.Code)
				return
			}
			require.Nil(t, apiErr)
			assert.Equal(t, "WELCOME10", saved.Code)
			assert.NotNil(t, saved.PlanIDs, "empty plan_ids is stored as an empty array, not NULL")
			assert.Equal(t, "WELCOME10", res.Code)
		})
	}
}

func TestUpdateAndDeletePromo(t *testing.T) {
	from, until := fixedNow, fixedNow.Add(time.Hour)
	req := api.UpdatePromoRequest{Code: "abc", Type: "fixed", Value: 5, ValidFrom: from, ValidUntil: until, PlanIDs: []string{planA}}

	t.Run("update ok", func(t *testing.T) {
		svc := newPromoSvc(nil, &mocks.PromoRepository{UpdateFn: func(_ context.Context, id string, p *db.PromoCode) (*db.PromoCode, error) {
			assert.Equal(t, "ABC", p.Code)
			return p, nil
		}})
		res, apiErr := svc.UpdatePromo(context.Background(), "a", planA, req)
		require.Nil(t, apiErr)
		assert.Equal(t, []string{planA}, res.PlanIDs)
	})
	t.Run("update not found", func(t *testing.T) {
		svc := newPromoSvc(nil, &mocks.PromoRepository{UpdateFn: func(context.Context, string, *db.PromoCode) (*db.PromoCode, error) {
			return nil, repositories.ErrNotFound
		}})
		_, apiErr := svc.UpdatePromo(context.Background(), "a", planA, req)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusNotFound, apiErr.Code)
	})
	t.Run("update bad id", func(t *testing.T) {
		_, apiErr := newPromoSvc(nil, nil).UpdatePromo(context.Background(), "a", "x", req)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusBadRequest, apiErr.Code)
	})
	t.Run("delete ok", func(t *testing.T) {
		res, apiErr := newPromoSvc(nil, &mocks.PromoRepository{}).DeletePromo(context.Background(), "a", planA)
		require.Nil(t, apiErr)
		assert.NotEmpty(t, res.Message)
	})
	t.Run("delete not found", func(t *testing.T) {
		svc := newPromoSvc(nil, &mocks.PromoRepository{DeleteFn: func(context.Context, string) error { return repositories.ErrNotFound }})
		_, apiErr := svc.DeletePromo(context.Background(), "a", planA)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusNotFound, apiErr.Code)
	})
}

func TestListPromos(t *testing.T) {
	svc := newPromoSvc(nil, &mocks.PromoRepository{ListFn: func(context.Context) ([]db.PromoCode, error) {
		return []db.PromoCode{{ID: "1", Code: "A"}}, nil
	}})
	res, apiErr := svc.ListPromos(context.Background(), "a")
	require.Nil(t, apiErr)
	require.Len(t, res.Items, 1)
	assert.NotNil(t, res.Items[0].PlanIDs)
}
