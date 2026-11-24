// Package mocks holds hand-written repository mocks for service tests.
// Each method delegates to its Fn field; an unset field returns a zero value.
package mocks

import (
	"context"
	"time"

	"9itchi-payments-api/core"
	"9itchi-payments-api/models/db"
)

type PlanRepository struct {
	ListFn       func(ctx context.Context, onlyActive bool) ([]db.Plan, error)
	GetByIDFn    func(ctx context.Context, id string) (*db.Plan, error)
	GetByNameFn  func(ctx context.Context, name string) (*db.Plan, error)
	CreateFn     func(ctx context.Context, p *db.Plan) (*db.Plan, error)
	UpdateFn     func(ctx context.Context, id string, p *db.Plan) (*db.Plan, error)
	DeactivateFn func(ctx context.Context, id string) (*db.Plan, error)
}

func (m *PlanRepository) List(ctx context.Context, onlyActive bool) ([]db.Plan, error) {
	if m.ListFn == nil {
		return nil, nil
	}
	return m.ListFn(ctx, onlyActive)
}

func (m *PlanRepository) GetByID(ctx context.Context, id string) (*db.Plan, error) {
	if m.GetByIDFn == nil {
		return nil, nil
	}
	return m.GetByIDFn(ctx, id)
}

func (m *PlanRepository) GetByName(ctx context.Context, name string) (*db.Plan, error) {
	if m.GetByNameFn == nil {
		return nil, nil
	}
	return m.GetByNameFn(ctx, name)
}

func (m *PlanRepository) Create(ctx context.Context, p *db.Plan) (*db.Plan, error) {
	if m.CreateFn == nil {
		return nil, nil
	}
	return m.CreateFn(ctx, p)
}

func (m *PlanRepository) Update(ctx context.Context, id string, p *db.Plan) (*db.Plan, error) {
	if m.UpdateFn == nil {
		return nil, nil
	}
	return m.UpdateFn(ctx, id, p)
}

func (m *PlanRepository) Deactivate(ctx context.Context, id string) (*db.Plan, error) {
	if m.DeactivateFn == nil {
		return nil, nil
	}
	return m.DeactivateFn(ctx, id)
}

type PromoRepository struct {
	ListFn           func(ctx context.Context) ([]db.PromoCode, error)
	GetByIDFn        func(ctx context.Context, id string) (*db.PromoCode, error)
	GetByCodeFn      func(ctx context.Context, code string) (*db.PromoCode, error)
	CreateFn         func(ctx context.Context, p *db.PromoCode) (*db.PromoCode, error)
	UpdateFn         func(ctx context.Context, id string, p *db.PromoCode) (*db.PromoCode, error)
	DeleteFn         func(ctx context.Context, id string) error
	IncrementUsageFn func(ctx context.Context, id string) (bool, error)
}

func (m *PromoRepository) List(ctx context.Context) ([]db.PromoCode, error) {
	if m.ListFn == nil {
		return nil, nil
	}
	return m.ListFn(ctx)
}

func (m *PromoRepository) GetByID(ctx context.Context, id string) (*db.PromoCode, error) {
	if m.GetByIDFn == nil {
		return nil, nil
	}
	return m.GetByIDFn(ctx, id)
}

func (m *PromoRepository) GetByCode(ctx context.Context, code string) (*db.PromoCode, error) {
	if m.GetByCodeFn == nil {
		return nil, nil
	}
	return m.GetByCodeFn(ctx, code)
}

func (m *PromoRepository) Create(ctx context.Context, p *db.PromoCode) (*db.PromoCode, error) {
	if m.CreateFn == nil {
		return nil, nil
	}
	return m.CreateFn(ctx, p)
}

func (m *PromoRepository) Update(ctx context.Context, id string, p *db.PromoCode) (*db.PromoCode, error) {
	if m.UpdateFn == nil {
		return nil, nil
	}
	return m.UpdateFn(ctx, id, p)
}

func (m *PromoRepository) Delete(ctx context.Context, id string) error {
	if m.DeleteFn == nil {
		return nil
	}
	return m.DeleteFn(ctx, id)
}

func (m *PromoRepository) IncrementUsage(ctx context.Context, id string) (bool, error) {
	if m.IncrementUsageFn == nil {
		return false, nil
	}
	return m.IncrementUsageFn(ctx, id)
}

type SubscriptionRepository struct {
	GetActiveByUserFn func(ctx context.Context, userID string) (*db.Subscription, error)
	CreateFn          func(ctx context.Context, s *db.Subscription) (*db.Subscription, error)
	ExtendFn          func(ctx context.Context, id string, endsAt time.Time) error
	ExpireActiveFn    func(ctx context.Context, userID string) error
	CancelFn          func(ctx context.Context, userID string) error
}

func (m *SubscriptionRepository) GetActiveByUser(ctx context.Context, userID string) (*db.Subscription, error) {
	if m.GetActiveByUserFn == nil {
		return nil, nil
	}
	return m.GetActiveByUserFn(ctx, userID)
}

func (m *SubscriptionRepository) Create(ctx context.Context, s *db.Subscription) (*db.Subscription, error) {
	if m.CreateFn == nil {
		return s, nil
	}
	return m.CreateFn(ctx, s)
}

func (m *SubscriptionRepository) Extend(ctx context.Context, id string, endsAt time.Time) error {
	if m.ExtendFn == nil {
		return nil
	}
	return m.ExtendFn(ctx, id, endsAt)
}

func (m *SubscriptionRepository) ExpireActive(ctx context.Context, userID string) error {
	if m.ExpireActiveFn == nil {
		return nil
	}
	return m.ExpireActiveFn(ctx, userID)
}

func (m *SubscriptionRepository) Cancel(ctx context.Context, userID string) error {
	if m.CancelFn == nil {
		return nil
	}
	return m.CancelFn(ctx, userID)
}

type PaymentRepository struct {
	CreateFn           func(ctx context.Context, p *db.Payment) (*db.Payment, error)
	SetProviderRefFn   func(ctx context.Context, id, ref string) error
	GetByProviderRefFn func(ctx context.Context, provider, ref string) (*db.Payment, error)
	MarkCompletedFn    func(ctx context.Context, id string) (bool, error)
	MarkFailedFn       func(ctx context.Context, id string) error
}

func (m *PaymentRepository) Create(ctx context.Context, p *db.Payment) (*db.Payment, error) {
	if m.CreateFn == nil {
		return p, nil
	}
	return m.CreateFn(ctx, p)
}

func (m *PaymentRepository) SetProviderRef(ctx context.Context, id, ref string) error {
	if m.SetProviderRefFn == nil {
		return nil
	}
	return m.SetProviderRefFn(ctx, id, ref)
}

func (m *PaymentRepository) GetByProviderRef(ctx context.Context, provider, ref string) (*db.Payment, error) {
	if m.GetByProviderRefFn == nil {
		return nil, nil
	}
	return m.GetByProviderRefFn(ctx, provider, ref)
}

func (m *PaymentRepository) MarkCompleted(ctx context.Context, id string) (bool, error) {
	if m.MarkCompletedFn == nil {
		return false, nil
	}
	return m.MarkCompletedFn(ctx, id)
}

func (m *PaymentRepository) MarkFailed(ctx context.Context, id string) error {
	if m.MarkFailedFn == nil {
		return nil
	}
	return m.MarkFailedFn(ctx, id)
}

type PaymentProvider struct {
	AvailableFn        func() bool
	CreateCheckoutFn   func(ctx context.Context, req core.CheckoutRequest) (core.CheckoutResult, error)
	VerifyWebhookFn    func(payload []byte, signature string) (string, error)
	VerifyPaymentFn    func(ctx context.Context, ref string) (bool, error)
	CreateCheckoutCall int
}

func (m *PaymentProvider) Available() bool { return m.AvailableFn == nil || m.AvailableFn() }

func (m *PaymentProvider) CreateCheckout(ctx context.Context, req core.CheckoutRequest) (core.CheckoutResult, error) {
	m.CreateCheckoutCall++
	if m.CreateCheckoutFn == nil {
		return core.CheckoutResult{}, nil
	}
	return m.CreateCheckoutFn(ctx, req)
}

func (m *PaymentProvider) VerifyWebhook(payload []byte, signature string) (string, error) {
	if m.VerifyWebhookFn == nil {
		return "", nil
	}
	return m.VerifyWebhookFn(payload, signature)
}

func (m *PaymentProvider) VerifyPayment(ctx context.Context, ref string) (bool, error) {
	if m.VerifyPaymentFn == nil {
		return false, nil
	}
	return m.VerifyPaymentFn(ctx, ref)
}

// Transactor runs fn directly (no database), like a transaction that always commits.
type Transactor struct{}

func (Transactor) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

// Publisher records the published event names.
type Publisher struct {
	Events []string
}

func (p *Publisher) Publish(_ context.Context, event string, _ any) error {
	p.Events = append(p.Events, event)
	return nil
}
