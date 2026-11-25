package services

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"9itchi-payments-api/models"
	"9itchi-payments-api/models/api"
	"9itchi-payments-api/models/db"
	"9itchi-payments-api/models/mapping"
	"9itchi-payments-api/repositories"
)

const (
	promoPercentage = "percentage"
	promoFixed      = "fixed"
)

// PromoService holds all business logic for promo. Handlers stay thin.
type PromoService interface {
	// POST /promo-codes/validate: check code against a plan
	ValidatePromo(ctx context.Context, userID string, req api.ValidatePromoRequest) (*api.ValidatePromoResponse, *models.ApiError)
	// GET /admin/promo-codes: list codes
	ListPromos(ctx context.Context, userID string) (*api.PromoListResponse, *models.ApiError)
	// POST /admin/promo-codes: percentage or fixed amount
	CreatePromo(ctx context.Context, userID string, req api.CreatePromoRequest) (*api.PromoResponse, *models.ApiError)
	// PUT /admin/promo-codes/:id: edit validity/limits
	UpdatePromo(ctx context.Context, userID string, id string, req api.UpdatePromoRequest) (*api.PromoResponse, *models.ApiError)
	// DELETE /admin/promo-codes/:id: delete code
	DeletePromo(ctx context.Context, userID string, id string) (*api.MessageResponse, *models.ApiError)
}

type promoService struct {
	repo  repositories.PromoRepository
	plans repositories.PlanRepository
	now   func() time.Time
}

func NewPromoService(repo repositories.PromoRepository, plans repositories.PlanRepository) PromoService {
	return &promoService{repo: repo, plans: plans, now: time.Now}
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

func normalizeCode(code string) string { return strings.ToUpper(strings.TrimSpace(code)) }

func validatePromoFields(code, typ string, value float64, from, until time.Time, usageLimit int) *models.ApiError {
	if len(code) < 3 {
		return badRequest("code must be at least 3 characters")
	}
	switch typ {
	case promoPercentage:
		if value < 1 || value > 100 {
			return badRequest("percentage value must be between 1 and 100")
		}
	case promoFixed:
		if value <= 0 {
			return badRequest("fixed value must be > 0")
		}
	default:
		return badRequest("type must be percentage or fixed")
	}
	if !from.Before(until) {
		return badRequest("valid_from must be before valid_until")
	}
	if usageLimit < 0 {
		return badRequest("usage_limit must be >= 0")
	}
	return nil
}

// discountFor computes the server-side discount for a promo on a plan at a given time.
// reasonDuringSale is why a code is refused on a plan that is in a sale.
const reasonDuringSale = "promo codes cannot be used during a sale"

func discountFor(promo *db.PromoCode, plan *db.Plan, now time.Time) api.ValidatePromoResponse {
	invalid := func(reason string) api.ValidatePromoResponse {
		return api.ValidatePromoResponse{Valid: false, FinalPrice: plan.Price, Reason: reason}
	}
	switch {
	case plan.Sale != nil:
		// A sale and a code never add up: while the plan is in a sale, no code is taken.
		return invalid(reasonDuringSale)
	case now.Before(promo.ValidFrom):
		return invalid("promo code is not yet valid")
	case !now.Before(promo.ValidUntil):
		return invalid("promo code has expired")
	case promo.UsageLimit > 0 && promo.UsedCount >= promo.UsageLimit:
		return invalid("promo code usage limit reached")
	}
	if len(promo.PlanIDs) > 0 {
		eligible := false
		for _, id := range promo.PlanIDs {
			if id == plan.ID {
				eligible = true
				break
			}
		}
		if !eligible {
			return invalid("promo code does not apply to this plan")
		}
	}
	var discount float64
	if promo.Type == promoPercentage {
		discount = plan.Price * promo.Value / 100
	} else {
		discount = promo.Value
	}
	discount = math.Min(round2(discount), plan.Price)
	return api.ValidatePromoResponse{Valid: true, DiscountAmount: discount, FinalPrice: math.Max(round2(plan.Price-discount), 0)}
}

func (s *promoService) ValidatePromo(ctx context.Context, _ string, req api.ValidatePromoRequest) (*api.ValidatePromoResponse, *models.ApiError) {
	if !isUUID(req.PlanID) {
		return nil, badRequest("invalid plan id")
	}
	plan, err := s.plans.GetByID(ctx, req.PlanID)
	if err != nil {
		return nil, repoError(err, "plan not found")
	}
	if !plan.Active {
		return nil, repoError(repositories.ErrNotFound, "plan not found")
	}
	promo, err := s.repo.GetByCode(ctx, normalizeCode(req.Code))
	if errors.Is(err, repositories.ErrNotFound) {
		return &api.ValidatePromoResponse{Valid: false, FinalPrice: plan.Price, Reason: "invalid promo code"}, nil
	}
	if err != nil {
		return nil, repoError(err, "promo code not found")
	}
	res := discountFor(promo, plan, s.now())
	return &res, nil
}

func (s *promoService) ListPromos(ctx context.Context, _ string) (*api.PromoListResponse, *models.ApiError) {
	promos, err := s.repo.List(ctx)
	if err != nil {
		return nil, repoError(err, "promo code not found")
	}
	items := make([]api.PromoResponse, 0, len(promos))
	for i := range promos {
		items = append(items, mapping.PromoToResponse(&promos[i]))
	}
	return &api.PromoListResponse{Items: items}, nil
}

func (s *promoService) CreatePromo(ctx context.Context, _ string, req api.CreatePromoRequest) (*api.PromoResponse, *models.ApiError) {
	code := normalizeCode(req.Code)
	if apiErr := validatePromoFields(code, req.Type, req.Value, req.ValidFrom, req.ValidUntil, req.UsageLimit); apiErr != nil {
		return nil, apiErr
	}
	ids, apiErr := cleanPlanIDs(req.PlanIDs)
	if apiErr != nil {
		return nil, apiErr
	}
	p, err := s.repo.Create(ctx, &db.PromoCode{Code: code, Type: req.Type, Value: req.Value,
		ValidFrom: req.ValidFrom, ValidUntil: req.ValidUntil, UsageLimit: req.UsageLimit, PlanIDs: ids})
	if err != nil {
		return nil, repoError(err, "promo code not found")
	}
	resp := mapping.PromoToResponse(p)
	return &resp, nil
}

func (s *promoService) UpdatePromo(ctx context.Context, _ string, id string, req api.UpdatePromoRequest) (*api.PromoResponse, *models.ApiError) {
	if !isUUID(id) {
		return nil, badRequest("invalid promo code id")
	}
	code := normalizeCode(req.Code)
	if apiErr := validatePromoFields(code, req.Type, req.Value, req.ValidFrom, req.ValidUntil, req.UsageLimit); apiErr != nil {
		return nil, apiErr
	}
	ids, apiErr := cleanPlanIDs(req.PlanIDs)
	if apiErr != nil {
		return nil, apiErr
	}
	p, err := s.repo.Update(ctx, id, &db.PromoCode{Code: code, Type: req.Type, Value: req.Value,
		ValidFrom: req.ValidFrom, ValidUntil: req.ValidUntil, UsageLimit: req.UsageLimit, PlanIDs: ids})
	if err != nil {
		return nil, repoError(err, "promo code not found")
	}
	resp := mapping.PromoToResponse(p)
	return &resp, nil
}

func (s *promoService) DeletePromo(ctx context.Context, _ string, id string) (*api.MessageResponse, *models.ApiError) {
	if !isUUID(id) {
		return nil, badRequest("invalid promo code id")
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, repoError(err, "promo code not found")
	}
	return &api.MessageResponse{Message: "promo code deleted"}, nil
}

// cleanPlanIDs returns a non-nil slice (empty = all plans) and rejects malformed ids.
func cleanPlanIDs(ids []string) ([]string, *models.ApiError) {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if !isUUID(id) {
			return nil, badRequest("plan_ids must be uuids")
		}
		out = append(out, strings.ToLower(id))
	}
	return out, nil
}
