package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"9itchi-payments-api/models"
	"9itchi-payments-api/models/api"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

type mockPlanSvc struct {
	list    func() (*api.PlanListResponse, *models.ApiError)
	create  func(api.CreatePlanRequest) (*api.PlanResponse, *models.ApiError)
	quotas  func(string) (*api.QuotasResponse, *models.ApiError)
	deactiv func(string) (*api.PlanResponse, *models.ApiError)
}

func (m *mockPlanSvc) ListPlans(context.Context) (*api.PlanListResponse, *models.ApiError) {
	return m.list()
}
func (m *mockPlanSvc) ListAllPlans(context.Context) (*api.PlanListResponse, *models.ApiError) {
	return m.list()
}
func (m *mockPlanSvc) CreatePlan(_ context.Context, _ string, r api.CreatePlanRequest) (*api.PlanResponse, *models.ApiError) {
	return m.create(r)
}
func (m *mockPlanSvc) UpdatePlan(context.Context, string, string, api.UpdatePlanRequest) (*api.PlanResponse, *models.ApiError) {
	return nil, models.NotImplemented()
}
func (m *mockPlanSvc) DeactivatePlan(_ context.Context, _ string, id string) (*api.PlanResponse, *models.ApiError) {
	return m.deactiv(id)
}
func (m *mockPlanSvc) GetQuotas(_ context.Context, id string) (*api.QuotasResponse, *models.ApiError) {
	return m.quotas(id)
}

type mockPromoSvc struct {
	validate func(api.ValidatePromoRequest) (*api.ValidatePromoResponse, *models.ApiError)
}

func (m *mockPromoSvc) ValidatePromo(_ context.Context, _ string, r api.ValidatePromoRequest) (*api.ValidatePromoResponse, *models.ApiError) {
	return m.validate(r)
}
func (m *mockPromoSvc) ListPromos(context.Context, string) (*api.PromoListResponse, *models.ApiError) {
	return &api.PromoListResponse{Items: []api.PromoResponse{}}, nil
}
func (m *mockPromoSvc) CreatePromo(context.Context, string, api.CreatePromoRequest) (*api.PromoResponse, *models.ApiError) {
	return &api.PromoResponse{ID: "p"}, nil
}
func (m *mockPromoSvc) UpdatePromo(context.Context, string, string, api.UpdatePromoRequest) (*api.PromoResponse, *models.ApiError) {
	return &api.PromoResponse{ID: "p"}, nil
}
func (m *mockPromoSvc) DeletePromo(context.Context, string, string) (*api.MessageResponse, *models.ApiError) {
	return &api.MessageResponse{Message: "ok"}, nil
}

type mockSubSvc struct {
	me func(string) (*api.MySubscriptionResponse, *models.ApiError)
}

func (m *mockSubSvc) Checkout(context.Context, string, api.CheckoutRequest) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}
func (m *mockSubSvc) MySubscription(_ context.Context, uid string) (*api.MySubscriptionResponse, *models.ApiError) {
	return m.me(uid)
}
func (m *mockSubSvc) CancelSubscription(context.Context, string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}
func (m *mockSubSvc) Webhook(context.Context, string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

// do runs one handler on a gin engine so path params and JSON binding behave as in production.
func do(method, route, path, body string, h gin.HandlerFunc) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Handle(method, route, func(c *gin.Context) { c.Set("user_id", "u-1"); h(c) })
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestListPlansHandler(t *testing.T) {
	svc := &mockPlanSvc{list: func() (*api.PlanListResponse, *models.ApiError) {
		return &api.PlanListResponse{Items: []api.PlanResponse{{ID: "1", Name: "Free", Quotas: map[string]int{"resumes": 3}}}}, nil
	}}
	w := do(http.MethodGet, "/plans", "/plans", "", ListPlans(svc))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"items"`)
	assert.Contains(t, w.Body.String(), `"duration_days"`)
}

func TestListPlansHandlerError(t *testing.T) {
	svc := &mockPlanSvc{list: func() (*api.PlanListResponse, *models.ApiError) {
		return nil, &models.ApiError{Code: http.StatusInternalServerError, Error: errors.New("internal server error")}
	}}
	w := do(http.MethodGet, "/plans", "/plans", "", ListPlans(svc))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.JSONEq(t, `{"error":"internal server error","status":500}`, w.Body.String())
}

func TestCreatePlanHandler(t *testing.T) {
	svc := &mockPlanSvc{create: func(r api.CreatePlanRequest) (*api.PlanResponse, *models.ApiError) {
		return &api.PlanResponse{ID: "n", Name: r.Name}, nil
	}}
	tests := []struct {
		name string
		body string
		want int
	}{
		{"ok", `{"name":"Pro","price":10,"duration_days":30,"quotas":{"resumes":10}}`, http.StatusCreated},
		{"negative price", `{"name":"Pro","price":-1,"duration_days":30,"quotas":{}}`, http.StatusBadRequest},
		{"zero duration", `{"name":"Pro","price":1,"duration_days":0,"quotas":{}}`, http.StatusBadRequest},
		{"missing name", `{"price":1,"duration_days":5,"quotas":{}}`, http.StatusBadRequest},
		{"bad json", `{`, http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := do(http.MethodPost, "/admin/plans", "/admin/plans", tc.body, CreatePlan(svc))
			assert.Equal(t, tc.want, w.Code)
			if tc.want != http.StatusCreated {
				assert.Contains(t, w.Body.String(), `"error"`)
			}
		})
	}
}

func TestDeactivatePlanHandlerNotFound(t *testing.T) {
	svc := &mockPlanSvc{deactiv: func(id string) (*api.PlanResponse, *models.ApiError) {
		assert.Equal(t, "abc", id)
		return nil, &models.ApiError{Code: http.StatusNotFound, Error: errors.New("plan not found")}
	}}
	w := do(http.MethodDelete, "/admin/plans/:id", "/admin/plans/abc", "", DeactivatePlan(svc))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetQuotasHandler(t *testing.T) {
	svc := &mockPlanSvc{quotas: func(id string) (*api.QuotasResponse, *models.ApiError) {
		return &api.QuotasResponse{UserID: id, Quotas: map[string]int{"resumes": 3}}, nil
	}}
	w := do(http.MethodGet, "/internal/quotas/:userId", "/internal/quotas/u9", "", GetQuotas(svc))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"user_id":"u9","plan_id":"","plan_name":"","quotas":{"resumes":3}}`, w.Body.String())
}

func TestValidatePromoHandler(t *testing.T) {
	svc := &mockPromoSvc{validate: func(r api.ValidatePromoRequest) (*api.ValidatePromoResponse, *models.ApiError) {
		return &api.ValidatePromoResponse{Valid: true, DiscountAmount: 5, FinalPrice: 45}, nil
	}}
	w := do(http.MethodPost, "/promo-codes/validate", "/promo-codes/validate",
		`{"code":"SAVE","plan_id":"11111111-1111-1111-1111-111111111111"}`, ValidatePromo(svc))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"valid":true,"discount_amount":5,"final_price":45}`, w.Body.String())

	for _, body := range []string{`{"code":"SAVE","plan_id":"nope"}`, `{"plan_id":"11111111-1111-1111-1111-111111111111"}`} {
		w = do(http.MethodPost, "/promo-codes/validate", "/promo-codes/validate", body, ValidatePromo(svc))
		assert.Equal(t, http.StatusBadRequest, w.Code, body)
	}
}

func TestCreatePromoHandlerValidation(t *testing.T) {
	svc := &mockPromoSvc{}
	ok := `{"code":"WELCOME","type":"percentage","value":10,"valid_from":"2026-01-01T00:00:00Z","valid_until":"2026-02-01T00:00:00Z","usage_limit":0,"plan_ids":[]}`
	assert.Equal(t, http.StatusCreated, do(http.MethodPost, "/admin/promo-codes", "/admin/promo-codes", ok, CreatePromo(svc)).Code)

	bad := `{"code":"WELCOME","type":"bogus","value":10,"valid_from":"2026-01-01T00:00:00Z","valid_until":"2026-02-01T00:00:00Z"}`
	assert.Equal(t, http.StatusBadRequest, do(http.MethodPost, "/admin/promo-codes", "/admin/promo-codes", bad, CreatePromo(svc)).Code)

	badPlan := `{"code":"WELCOME","type":"fixed","value":10,"valid_from":"2026-01-01T00:00:00Z","valid_until":"2026-02-01T00:00:00Z","plan_ids":["x"]}`
	assert.Equal(t, http.StatusBadRequest, do(http.MethodPost, "/admin/promo-codes", "/admin/promo-codes", badPlan, CreatePromo(svc)).Code)
}

func TestMySubscriptionHandler(t *testing.T) {
	svc := &mockSubSvc{me: func(uid string) (*api.MySubscriptionResponse, *models.ApiError) {
		assert.Equal(t, "u-1", uid, "user id comes from the auth context")
		return &api.MySubscriptionResponse{IsFree: true, Plan: api.PlanResponse{Name: "Free"}}, nil
	}}
	w := do(http.MethodGet, "/subscriptions/me", "/subscriptions/me", "", MySubscription(svc))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"is_free":true`)
	assert.Contains(t, w.Body.String(), `"subscription":null`)
}

func TestCheckoutHandlerStays501(t *testing.T) {
	w := do(http.MethodPost, "/subscriptions/checkout", "/subscriptions/checkout", `{}`, Checkout(&mockSubSvc{}))
	assert.Equal(t, http.StatusNotImplemented, w.Code)
}
