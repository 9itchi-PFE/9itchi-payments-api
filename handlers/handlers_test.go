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
	me       func(string) (*api.MySubscriptionResponse, *models.ApiError)
	checkout func(api.CheckoutRequest) (*api.CheckoutResponse, *models.ApiError)
	webhook  func(provider string, body []byte, signature string) (*api.MessageResponse, *models.ApiError)
}

func (m *mockSubSvc) Checkout(_ context.Context, _ string, r api.CheckoutRequest) (*api.CheckoutResponse, *models.ApiError) {
	return m.checkout(r)
}
func (m *mockSubSvc) ListPayments(context.Context, string) (*api.PaymentListResponse, *models.ApiError) {
	return &api.PaymentListResponse{Items: []api.PaymentResponse{{ID: "p1", Status: "completed", ReceiptNo: "2026-000001"}}}, nil
}
func (m *mockSubSvc) VerifyPayment(_ context.Context, _ string, id string) (*api.PaymentResponse, *models.ApiError) {
	if id == "missing" {
		return nil, &models.ApiError{Code: http.StatusNotFound, Error: errors.New("payment not found")}
	}
	return &api.PaymentResponse{ID: id, Status: "pending"}, nil
}
func (m *mockSubSvc) MySubscription(_ context.Context, uid string) (*api.MySubscriptionResponse, *models.ApiError) {
	return m.me(uid)
}
func (m *mockSubSvc) CancelSubscription(context.Context, string) (*api.MySubscriptionResponse, *models.ApiError) {
	return &api.MySubscriptionResponse{Subscription: &api.SubscriptionResponse{ID: "s", Renews: false}}, nil
}
func (m *mockSubSvc) ResumeSubscription(context.Context, string) (*api.MySubscriptionResponse, *models.ApiError) {
	return &api.MySubscriptionResponse{Subscription: &api.SubscriptionResponse{ID: "s", Renews: true}}, nil
}
func (m *mockSubSvc) Webhook(_ context.Context, provider string, body []byte, signature string) (*api.MessageResponse, *models.ApiError) {
	return m.webhook(provider, body, signature)
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

func TestCheckoutHandler(t *testing.T) {
	svc := &mockSubSvc{checkout: func(r api.CheckoutRequest) (*api.CheckoutResponse, *models.ApiError) {
		return &api.CheckoutResponse{PaymentURL: "https://pay.example/x", PaymentID: "pay-1"}, nil
	}}
	body := `{"plan_id":"11111111-1111-1111-1111-111111111111","promo_code":"SAVE"}`
	w := do(http.MethodPost, "/subscriptions/checkout", "/subscriptions/checkout", body, Checkout(svc))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"payment_url":"https://pay.example/x","payment_id":"pay-1"}`, w.Body.String())

	for _, bad := range []string{`{}`, `{"plan_id":"nope"}`, `{`} {
		w = do(http.MethodPost, "/subscriptions/checkout", "/subscriptions/checkout", bad, Checkout(svc))
		assert.Equal(t, http.StatusBadRequest, w.Code, bad)
	}
}

func TestWebhookHandler(t *testing.T) {
	var gotProvider, gotBody, gotSig string
	svc := &mockSubSvc{webhook: func(provider string, body []byte, sig string) (*api.MessageResponse, *models.ApiError) {
		gotProvider, gotBody, gotSig = provider, string(body), sig
		if sig != "good" {
			return nil, &models.ApiError{Code: http.StatusUnauthorized, Error: errors.New("invalid signature")}
		}
		return &api.MessageResponse{Message: "ok"}, nil
	}}
	run := func(sig string) *httptest.ResponseRecorder {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		r.POST("/webhooks/:provider", Webhook(svc))
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/webhooks/flouci", strings.NewReader(`{"payment_id":"r1"}`))
		req.Header.Set("X-Flouci-Signature", sig)
		r.ServeHTTP(w, req)
		return w
	}
	w := run("good")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "flouci", gotProvider)
	assert.Equal(t, `{"payment_id":"r1"}`, gotBody, "raw body is passed untouched for signature checks")
	assert.Equal(t, "good", gotSig)

	w = run("forged")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), `"error":"invalid signature"`)
}

func TestCancelSubscriptionHandler(t *testing.T) {
	w := do(http.MethodPost, "/subscriptions/cancel", "/subscriptions/cancel", "", CancelSubscription(&mockSubSvc{}))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"renews":false`)

	w = do(http.MethodPost, "/subscriptions/resume", "/subscriptions/resume", "", ResumeSubscription(&mockSubSvc{}))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"renews":true`)
}

func TestPaymentHandlers(t *testing.T) {
	w := do(http.MethodGet, "/payments", "/payments", "", ListPayments(&mockSubSvc{}))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"receipt_no":"2026-000001"`)

	w = do(http.MethodPost, "/payments/:id/verify", "/payments/p1/verify", "", VerifyPayment(&mockSubSvc{}))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"pending"`)

	w = do(http.MethodPost, "/payments/:id/verify", "/payments/missing/verify", "", VerifyPayment(&mockSubSvc{}))
	assert.Equal(t, http.StatusNotFound, w.Code)
}
