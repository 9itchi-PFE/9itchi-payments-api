package services

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

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
	// priced reads the same plans at today's price, sales included: what students are shown.
	priced repositories.PlanRepository
	subs   repositories.SubscriptionRepository
	// grants is the free usage given to emails, laid over the Free plan (nil: none).
	grants *freeGrants
}

func NewPlanService(repo, priced repositories.PlanRepository, subs repositories.SubscriptionRepository) PlanService {
	return &planService{repo: repo, priced: priced, subs: subs}
}

func (s *planService) ListPlans(ctx context.Context) (*api.PlanListResponse, *models.ApiError) {
	return s.list(ctx, true)
}

func (s *planService) ListAllPlans(ctx context.Context) (*api.PlanListResponse, *models.ApiError) {
	return s.list(ctx, false)
}

func (s *planService) list(ctx context.Context, onlyActive bool) (*api.PlanListResponse, *models.ApiError) {
	// Students see what a plan costs today; the admin sees what was typed, the end of the launch included.
	repo := s.repo
	if onlyActive {
		repo = s.priced
	}
	plans, err := repo.List(ctx, onlyActive)
	if err != nil {
		return nil, repoError(err, "plan not found")
	}
	items := make([]api.PlanResponse, 0, len(plans))
	for i := range plans {
		items = append(items, mapping.PlanToResponse(&plans[i]))
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

// validatePaidPlan holds every plan but Free to what a paid plan is: it costs something (a free one could never be
// bought, checkout refuses it) and it is not called Free in any case, the name the fallback plan is found by.
func validatePaidPlan(name string, price float64) *models.ApiError {
	if strings.EqualFold(name, FreePlanName) {
		return badRequest("the name Free is kept for the plan without a subscription")
	}
	if price <= 0 {
		return badRequest("a plan other than Free must cost more than 0")
	}
	return nil
}

func (s *planService) CreatePlan(ctx context.Context, _ string, req api.CreatePlanRequest) (*api.PlanResponse, *models.ApiError) {
	name := strings.TrimSpace(req.Name)
	if apiErr := validatePlanFields(name, req.Price, req.RegularPrice, req.DurationDays, req.Quotas); apiErr != nil {
		return nil, apiErr
	}
	if apiErr := validatePaidPlan(name, req.Price); apiErr != nil {
		return nil, apiErr
	}
	ends, apiErr := launchEnd(req.LaunchEndsOn, req.RegularPrice)
	if apiErr != nil {
		return nil, apiErr
	}
	details := db.PlanDetails{}
	if req.Details != nil {
		if details, apiErr = cleanDetails(*req.Details); apiErr != nil {
			return nil, apiErr
		}
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	p, err := s.repo.Create(ctx, &db.Plan{Name: name, Price: req.Price, RegularPrice: req.RegularPrice, LaunchEndsOn: ends, DurationDays: req.DurationDays, Quotas: req.Quotas, Details: details, Active: active})
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
	var details db.PlanDetails
	if req.Details != nil {
		if details, apiErr = cleanDetails(*req.Details); apiErr != nil {
			return nil, apiErr
		}
	}
	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, repoError(err, "plan not found")
	}
	if req.Details == nil {
		details = current.Details // left out: the text stays as it is
	}
	active := current.Active
	if req.Active != nil {
		active = *req.Active
	}
	// The Free plan is the fallback for users without a subscription: it must stay named Free and active.
	if current.Name == FreePlanName && (name != FreePlanName || !active) {
		return nil, badRequest("the Free plan cannot be renamed or deactivated")
	}
	if current.Name != FreePlanName {
		if apiErr := validatePaidPlan(name, req.Price); apiErr != nil {
			return nil, apiErr
		}
	}
	// Quota periods of running subscriptions are counted with the plan's length: changing it under them would move
	// their resets. The length stays while anyone holds the plan; the team creates a new plan instead.
	if req.DurationDays != current.DurationDays {
		held, err := s.subs.HasActiveOnPlan(ctx, id)
		if err != nil {
			return nil, repoError(err, "plan not found")
		}
		if held {
			return nil, &models.ApiError{Code: http.StatusConflict, Error: errors.New("the length of a plan with active subscribers cannot change")}
		}
	}
	p, err := s.repo.Update(ctx, id, &db.Plan{Name: name, Price: req.Price, RegularPrice: req.RegularPrice, LaunchEndsOn: ends, DurationDays: req.DurationDays, Quotas: req.Quotas, Details: details, Active: active})
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
	sub, plan, err := userPlan(ctx, s.subs, s.repo, userID)
	if err != nil {
		return nil, repoError(err, "plan not found")
	}
	plan, _ = s.grants.apply(ctx, userID, plan, sub != nil)
	quotas := plan.Quotas
	if quotas == nil {
		quotas = map[string]int{}
	}
	return &api.QuotasResponse{UserID: userID, PlanID: plan.ID, PlanName: plan.Name, Quotas: quotas, PeriodStart: periodStart(sub, plan, time.Now())}, nil
}

// periodStart is when the period a subscription is in began. Periods are counted back from its end, one plan
// duration at a time, down to the one that holds now; never before the subscription itself. Days the team gave, or
// a renewal paid ahead, push the end further than one duration away: the period in progress is still the one now
// falls in. Nil without a subscription.
func periodStart(sub *db.Subscription, plan *db.Plan, now time.Time) *time.Time {
	if sub == nil || plan == nil {
		return nil
	}
	start := sub.StartsAt
	if plan.DurationDays > 0 {
		for at := sub.EndsAt.AddDate(0, 0, -plan.DurationDays); at.After(sub.StartsAt); at = at.AddDate(0, 0, -plan.DurationDays) {
			if !at.After(now) {
				start = at
				break
			}
		}
	}
	return &start
}

// Languages a plan can be described in, and the limits of a description.
var planLocales = map[string]bool{"fr": true, "en": true, "ar": true}

const (
	maxPlanLabel    = 40
	maxPlanTagline  = 120
	maxPlanFeatures = 10
	maxPlanFeature  = 140
)

// cleanDetails checks the text students read about a plan and returns it trimmed. A fault is refused, nothing is
// dropped silently: an unknown language, a text too long, too many features or an empty one is a 400.
func cleanDetails(in db.PlanDetails) (db.PlanDetails, *models.ApiError) {
	out := make(db.PlanDetails, len(in))
	for locale, text := range in {
		if !planLocales[locale] {
			return nil, badRequest("details: language must be fr, en or ar")
		}
		label, tagline := strings.TrimSpace(text.Label), strings.TrimSpace(text.Tagline)
		if utf8.RuneCountInString(label) > maxPlanLabel {
			return nil, badRequest("details: label is too long (40 characters at most)")
		}
		if utf8.RuneCountInString(tagline) > maxPlanTagline {
			return nil, badRequest("details: tagline is too long (120 characters at most)")
		}
		if len(text.Features) > maxPlanFeatures {
			return nil, badRequest("details: 10 features at most")
		}
		var features []string
		for _, f := range text.Features {
			f = strings.TrimSpace(f)
			if f == "" || utf8.RuneCountInString(f) > maxPlanFeature {
				return nil, badRequest("details: each feature must be 1 to 140 characters")
			}
			features = append(features, f)
		}
		out[locale] = db.PlanText{Label: label, Tagline: tagline, Features: features}
	}
	return out, nil
}
