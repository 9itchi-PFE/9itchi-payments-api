package services

import (
	"context"

	"9itchi-payments-api/models"
	"9itchi-payments-api/models/api"
	"9itchi-payments-api/repositories"
)

// PromoService holds all business logic for promo. Handlers stay thin.
type PromoService interface {
	// POST /promo-codes/validate: check code against a plan
	ValidatePromo(ctx context.Context, userID string, req api.ValidatePromoRequest) (any, *models.ApiError)
	// GET /admin/promo-codes: list codes
	ListPromos(ctx context.Context, userID string) (any, *models.ApiError)
	// POST /admin/promo-codes: percentage or fixed amount
	CreatePromo(ctx context.Context, userID string, req api.CreatePromoRequest) (any, *models.ApiError)
	// PUT /admin/promo-codes/:id: edit validity/limits
	UpdatePromo(ctx context.Context, userID string, id string, req api.UpdatePromoRequest) (any, *models.ApiError)
	// DELETE /admin/promo-codes/:id: delete code
	DeletePromo(ctx context.Context, userID string, id string) (any, *models.ApiError)
}

type promoService struct {
	repo repositories.PromoRepository
}

func NewPromoService(repo repositories.PromoRepository) PromoService {
	return &promoService{repo: repo}
}

func (s *promoService) ValidatePromo(_ context.Context, _ string, _ api.ValidatePromoRequest) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *promoService) ListPromos(_ context.Context, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *promoService) CreatePromo(_ context.Context, _ string, _ api.CreatePromoRequest) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *promoService) UpdatePromo(_ context.Context, _ string, _ string, _ api.UpdatePromoRequest) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *promoService) DeletePromo(_ context.Context, _ string, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}
