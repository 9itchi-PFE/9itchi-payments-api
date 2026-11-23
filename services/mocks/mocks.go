// Package mocks holds hand-written repository mocks for service tests.
// Each method delegates to its Fn field; an unset field returns a zero value.
package mocks

import (
	"context"

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
}

func (m *SubscriptionRepository) GetActiveByUser(ctx context.Context, userID string) (*db.Subscription, error) {
	if m.GetActiveByUserFn == nil {
		return nil, nil
	}
	return m.GetActiveByUserFn(ctx, userID)
}
