package services

import (
	"context"

	"9itchi-payments-api/models"
	"9itchi-payments-api/models/api"
	"9itchi-payments-api/repositories"
)

// PlanService holds all business logic for plan. Handlers stay thin.
type PlanService interface {
	// GET /plans: active plans
	ListPlans(ctx context.Context) (any, *models.ApiError)
	// POST /admin/plans: create plan (price, duration, quotas)
	CreatePlan(ctx context.Context, userID string, req api.CreatePlanRequest) (any, *models.ApiError)
	// PUT /admin/plans/:id: edit plan
	UpdatePlan(ctx context.Context, userID string, id string, req api.UpdatePlanRequest) (any, *models.ApiError)
	// DELETE /admin/plans/:id: deactivate plan
	DeactivatePlan(ctx context.Context, userID string, id string) (any, *models.ApiError)
	// GET /internal/quotas/:userId: remaining quotas for a user
	GetQuotas(ctx context.Context, userId string) (any, *models.ApiError)
}

type planService struct {
	repo repositories.PlanRepository
}

func NewPlanService(repo repositories.PlanRepository) PlanService {
	return &planService{repo: repo}
}

func (s *planService) ListPlans(_ context.Context) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *planService) CreatePlan(_ context.Context, _ string, _ api.CreatePlanRequest) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *planService) UpdatePlan(_ context.Context, _ string, _ string, _ api.UpdatePlanRequest) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *planService) DeactivatePlan(_ context.Context, _ string, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *planService) GetQuotas(_ context.Context, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}
