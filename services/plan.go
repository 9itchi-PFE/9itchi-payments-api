package services

import (
	"context"
	"strings"
	"time"

	"9itchi-payments-api/models"
	"9itchi-payments-api/models/api"
	"9itchi-payments-api/models/db"
	"9itchi-payments-api/models/mapping"
	"9itchi-payments-api/repositories"
)

// PlanService holds all business logic for plan. Handlers stay thin.
type PlanService interface {
	// GET /plans: active plans
	ListPlans(ctx context.Context) (*api.PlanListResponse, *models.ApiError)
	// GET /admin/plans: all plans, including inactive
	ListAllPlans(ctx context.Context) (*api.PlanListResponse, *models.ApiError)
	// POST /admin/plans: create plan (price, duration, quotas)
	CreatePlan(ctx context.Context, userID string, req api.CreatePlanRequest) (*api.PlanResponse, *models.ApiError)
	// PUT /admin/plans/:id: edit plan (full replacement: leaving regular_price out removes it)
	UpdatePlan(ctx context.Context, userID string, id string, req api.UpdatePlanRequest) (*api.PlanResponse, *models.ApiError)
	// DELETE /admin/plans/:id: deactivate plan
	DeactivatePlan(ctx context.Context, userID string, id string) (*api.PlanResponse, *models.ApiError)
	// GET /internal/quotas/:userId: quotas of the user's active plan (Free fallback)
	GetQuotas(ctx context.Context, userId string) (*api.QuotasResponse, *models.ApiError)
}

type planService struct {
	repo repositories.PlanRepository
	subs repositories.SubscriptionRepository
	now  func() time.Time
}

func NewPlanService(repo repositories.PlanRepository, subs repositories.SubscriptionRepository) PlanService {
	return &planService{repo: repo, subs: subs, now: time.Now}
}

func (s *planService) ListPlans(ctx context.Context) (*api.PlanListResponse, *models.ApiError) {
	return s.list(ctx, true)
}

func (s *planService) ListAllPlans(ctx context.Context) (*api.PlanListResponse, *models.ApiError) {
	return s.list(ctx, false)
}

func (s *planService) list(ctx context.Context, onlyActive bool) (*api.PlanListResponse, *models.ApiError) {
	plans, err := s.repo.List(ctx, onlyActive)
	if err != nil {
		return nil, repoError(err, "plan not found")
	}
	items := make([]api.PlanResponse, 0, len(plans))
	for i := range plans {
		p := &plans[i]
		if onlyActive {
			// Students see what a plan costs today; the admin sees what was typed, the end of the launch included.
			p = priceAt(p, s.now())
		}
		items = append(items, mapping.PlanToResponse(p))
	}
	return &api.PlanListResponse{Items: items}, nil
}

func validatePlanFields(name string, price float64, regular *float64, duration int, quotas map[string]int) *models.ApiError {
	if name == "" {
		return badRequest("name is required")
	}
	if price < 0 {
		return badRequest("price must be >= 0")
	}
	if regular != nil && *regular <= price {
		return badRequest("regular_price must be above price")
	}
	if duration <= 0 {
		return badRequest("duration_days must be > 0")
	}
	for k, v := range quotas {
		if k == "" || v < 0 {
			return badRequest("quotas must have non-empty keys and values >= 0")
		}
	}
	return nil
}

func (s *planService) CreatePlan(ctx context.Context, _ string, req api.CreatePlanRequest) (*api.PlanResponse, *models.ApiError) {
	name := strings.TrimSpace(req.Name)
	if apiErr := validatePlanFields(name, req.Price, req.RegularPrice, req.DurationDays, req.Quotas); apiErr != nil {
		return nil, apiErr
	}
	ends, apiErr := launchEnd(req.LaunchEndsOn, req.RegularPrice)
	if apiErr != nil {
		return nil, apiErr
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	p, err := s.repo.Create(ctx, &db.Plan{Name: name, Price: req.Price, RegularPrice: req.RegularPrice, LaunchEndsOn: ends, DurationDays: req.DurationDays, Quotas: req.Quotas, Active: active})
	if err != nil {
		return nil, repoError(err, "plan not found")
	}
	resp := mapping.PlanToResponse(p)
	return &resp, nil
}

func (s *planService) UpdatePlan(ctx context.Context, _ string, id string, req api.UpdatePlanRequest) (*api.PlanResponse, *models.ApiError) {
	if !isUUID(id) {
		return nil, badRequest("invalid plan id")
	}
	name := strings.TrimSpace(req.Name)
	if apiErr := validatePlanFields(name, req.Price, req.RegularPrice, req.DurationDays, req.Quotas); apiErr != nil {
		return nil, apiErr
	}
	ends, apiErr := launchEnd(req.LaunchEndsOn, req.RegularPrice)
	if apiErr != nil {
		return nil, apiErr
	}
	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, repoError(err, "plan not found")
	}
	active := current.Active
	if req.Active != nil {
		active = *req.Active
	}
	// The Free plan is the fallback for users without a subscription: it must stay named Free and active.
	if current.Name == FreePlanName && (name != FreePlanName || !active) {
		return nil, badRequest("the Free plan cannot be renamed or deactivated")
	}
	p, err := s.repo.Update(ctx, id, &db.Plan{Name: name, Price: req.Price, RegularPrice: req.RegularPrice, LaunchEndsOn: ends, DurationDays: req.DurationDays, Quotas: req.Quotas, Active: active})
	if err != nil {
		return nil, repoError(err, "plan not found")
	}
	resp := mapping.PlanToResponse(p)
	return &resp, nil
}

func (s *planService) DeactivatePlan(ctx context.Context, _ string, id string) (*api.PlanResponse, *models.ApiError) {
	if !isUUID(id) {
		return nil, badRequest("invalid plan id")
	}
	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, repoError(err, "plan not found")
	}
	if current.Name == FreePlanName {
		return nil, badRequest("the Free plan cannot be renamed or deactivated")
	}
	p, err := s.repo.Deactivate(ctx, id)
	if err != nil {
		return nil, repoError(err, "plan not found")
	}
	resp := mapping.PlanToResponse(p)
	return &resp, nil
}

func (s *planService) GetQuotas(ctx context.Context, userID string) (*api.QuotasResponse, *models.ApiError) {
	if !isUUID(userID) {
		return nil, badRequest("invalid user id")
	}
	_, plan, err := userPlan(ctx, s.subs, s.repo, userID)
	if err != nil {
		return nil, repoError(err, "plan not found")
	}
	quotas := plan.Quotas
	if quotas == nil {
		quotas = map[string]int{}
	}
	return &api.QuotasResponse{UserID: userID, PlanID: plan.ID, PlanName: plan.Name, Quotas: quotas}, nil
}
