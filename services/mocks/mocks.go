// Package mocks holds hand-written repository mocks for service tests.
// Each method delegates to its Fn field; an unset field returns a zero value.
package mocks

import (
	"context"
	"time"

	"9itchi-payments-api/core"
	"9itchi-payments-api/models/db"
	"9itchi-payments-api/repositories"
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
	AddDaysFn         func(ctx context.Context, id string, days int) (*db.Subscription, error)
	ExpireActiveFn    func(ctx context.Context, userID string) error
	SetRenewalFn      func(ctx context.Context, userID string, renews bool) (*db.Subscription, error)
	LastEndedFn       func(ctx context.Context, userID string) (*db.Subscription, error)
	ClaimRemindersFn  func(ctx context.Context, stage string, window time.Duration, limit int) ([]db.RenewalReminder, error)
	ReleaseReminderFn func(ctx context.Context, subscriptionID, stage string) error
	HasActiveOnPlanFn func(ctx context.Context, planID string) (bool, error)
}

func (m *SubscriptionRepository) HasActiveOnPlan(ctx context.Context, planID string) (bool, error) {
	if m.HasActiveOnPlanFn == nil {
		return false, nil
	}
	return m.HasActiveOnPlanFn(ctx, planID)
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

func (m *SubscriptionRepository) SetRenewal(ctx context.Context, userID string, renews bool) (*db.Subscription, error) {
	if m.SetRenewalFn == nil {
		return nil, repositories.ErrNotFound
	}
	return m.SetRenewalFn(ctx, userID, renews)
}

func (m *SubscriptionRepository) LastEnded(ctx context.Context, userID string) (*db.Subscription, error) {
	if m.LastEndedFn == nil {
		return nil, repositories.ErrNotFound
	}
	return m.LastEndedFn(ctx, userID)
}

type PaymentRepository struct {
	CreateFn           func(ctx context.Context, p *db.Payment) (*db.Payment, error)
	SetProviderRefFn   func(ctx context.Context, id, ref string) error
	GetByProviderRefFn func(ctx context.Context, provider, ref string) (*db.Payment, error)
	MarkCompletedFn    func(ctx context.Context, id string) (bool, error)
	MarkFailedFn       func(ctx context.Context, id string) error
	SetPeriodFn        func(ctx context.Context, id string, start, end time.Time) error
	GetFn              func(ctx context.Context, userID, id string) (*db.Payment, error)
	ListCompletedFn    func(ctx context.Context, userID string, limit int) ([]db.Payment, error)
}

func (m *PaymentRepository) SetPeriod(ctx context.Context, id string, start, end time.Time) error {
	if m.SetPeriodFn == nil {
		return nil
	}
	return m.SetPeriodFn(ctx, id, start, end)
}

func (m *PaymentRepository) Get(ctx context.Context, userID, id string) (*db.Payment, error) {
	if m.GetFn == nil {
		return nil, repositories.ErrNotFound
	}
	return m.GetFn(ctx, userID, id)
}

func (m *PaymentRepository) ListCompletedByUser(ctx context.Context, userID string, limit int) ([]db.Payment, error) {
	if m.ListCompletedFn == nil {
		return nil, nil
	}
	return m.ListCompletedFn(ctx, userID, limit)
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

// Publisher records the published event names and their payloads.
type Publisher struct {
	Events   []string
	Payloads []any
	// Fail, when set, decides which publish fails, like a broker that is down.
	Fail func(event string, payload any) error
}

func (p *Publisher) Publish(_ context.Context, event string, payload any) error {
	if p.Fail != nil {
		if err := p.Fail(event, payload); err != nil {
			return err
		}
	}
	p.Events = append(p.Events, event)
	p.Payloads = append(p.Payloads, payload)
	return nil
}

type IssuerRepository struct {
	GetFn    func(ctx context.Context) (*db.ReceiptIssuer, error)
	UpdateFn func(ctx context.Context, userID string, i *db.ReceiptIssuer) (*db.ReceiptIssuer, error)
}

func (m *IssuerRepository) Get(ctx context.Context) (*db.ReceiptIssuer, error) {
	if m.GetFn == nil {
		return &db.ReceiptIssuer{}, nil
	}
	return m.GetFn(ctx)
}

func (m *IssuerRepository) Update(ctx context.Context, userID string, i *db.ReceiptIssuer) (*db.ReceiptIssuer, error) {
	if m.UpdateFn == nil {
		return i, nil
	}
	return m.UpdateFn(ctx, userID, i)
}

func (m *SubscriptionRepository) ClaimReminders(ctx context.Context, stage string, window time.Duration, limit int) ([]db.RenewalReminder, error) {
	if m.ClaimRemindersFn == nil {
		return nil, nil
	}
	return m.ClaimRemindersFn(ctx, stage, window, limit)
}

func (m *SubscriptionRepository) ReleaseReminder(ctx context.Context, subscriptionID, stage string) error {
	if m.ReleaseReminderFn == nil {
		return nil
	}
	return m.ReleaseReminderFn(ctx, subscriptionID, stage)
}

func (m *SubscriptionRepository) AddDays(ctx context.Context, id string, days int) (*db.Subscription, error) {
	if m.AddDaysFn == nil {
		return nil, nil
	}
	return m.AddDaysFn(ctx, id, days)
}

// GiftRepository keeps what was created; ListFn and CreateErr replace the defaults.
type GiftRepository struct {
	Locked    []string
	Created   []*db.Gift
	CreateErr error
	ListFn    func(ctx context.Context, userID string, limit int) ([]db.Gift, error)
}

func (m *GiftRepository) LockUser(_ context.Context, userID string) error {
	m.Locked = append(m.Locked, userID)
	return nil
}

func (m *GiftRepository) Create(_ context.Context, g *db.Gift) (*db.Gift, error) {
	if m.CreateErr != nil {
		return nil, m.CreateErr
	}
	out := *g
	out.ID = "gift-1"
	m.Created = append(m.Created, &out)
	return &out, nil
}

func (m *GiftRepository) ListByUser(ctx context.Context, userID string, limit int) ([]db.Gift, error) {
	if m.ListFn == nil {
		return nil, nil
	}
	return m.ListFn(ctx, userID, limit)
}

// SaleRepository is a mock of repositories.SaleRepository: Sales is the table, Err fails every call.
type SaleRepository struct {
	Sales []db.Sale
	Err   error
}

func (m *SaleRepository) List(context.Context) ([]db.Sale, error) {
	return append([]db.Sale(nil), m.Sales...), m.Err
}

func (m *SaleRepository) GetByID(_ context.Context, id string) (*db.Sale, error) {
	for i := range m.Sales {
		if m.Sales[i].ID == id {
			out := m.Sales[i]
			return &out, m.Err
		}
	}
	if m.Err != nil {
		return nil, m.Err
	}
	return nil, repositories.ErrNotFound
}

func (m *SaleRepository) RunningOn(_ context.Context, day time.Time) ([]db.Sale, error) {
	var out []db.Sale
	for _, s := range m.Sales {
		if !day.Before(s.StartsOn) && !day.After(s.EndsOn) {
			out = append(out, s)
		}
	}
	return out, m.Err
}

func (m *SaleRepository) Create(_ context.Context, s *db.Sale) (*db.Sale, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	out := *s
	out.ID = "sale-" + string(rune('a'+len(m.Sales)))
	m.Sales = append(m.Sales, out)
	return &out, nil
}

func (m *SaleRepository) Update(_ context.Context, id string, s *db.Sale) (*db.Sale, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	for i := range m.Sales {
		if m.Sales[i].ID == id {
			out := *s
			out.ID = id
			m.Sales[i] = out
			return &out, nil
		}
	}
	return nil, repositories.ErrNotFound
}

func (m *SaleRepository) Delete(_ context.Context, id string) error {
	if m.Err != nil {
		return m.Err
	}
	for i := range m.Sales {
		if m.Sales[i].ID == id {
			m.Sales = append(m.Sales[:i], m.Sales[i+1:]...)
			return nil
		}
	}
	return repositories.ErrNotFound
}

type StudentDiscountRepository struct {
	GetFn    func(ctx context.Context) (*db.StudentDiscount, error)
	UpdateFn func(ctx context.Context, userID string, d *db.StudentDiscount) (*db.StudentDiscount, error)
}

func (m *StudentDiscountRepository) Get(ctx context.Context) (*db.StudentDiscount, error) {
	if m.GetFn == nil {
		return &db.StudentDiscount{Domains: []string{}}, nil
	}
	return m.GetFn(ctx)
}

func (m *StudentDiscountRepository) Update(ctx context.Context, userID string, d *db.StudentDiscount) (*db.StudentDiscount, error) {
	if m.UpdateFn == nil {
		return d, nil
	}
	return m.UpdateFn(ctx, userID, d)
}
