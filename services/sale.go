package services

import (
	"context"
	"sort"
	"strings"
	"time"

	"9itchi-payments-api/models"
	"9itchi-payments-api/models/api"
	"9itchi-payments-api/models/db"
	"9itchi-payments-api/models/mapping"
	"9itchi-payments-api/repositories"
)

// SaleService manages the sales: reductions without a code, between two days, set by the admin.
type SaleService interface {
	// GET /admin/sales
	ListSales(ctx context.Context) (*api.SaleListResponse, *models.ApiError)
	// POST /admin/sales
	CreateSale(ctx context.Context, req api.SaleRequest) (*api.SaleResponse, *models.ApiError)
	// PUT /admin/sales/:id
	UpdateSale(ctx context.Context, id string, req api.SaleRequest) (*api.SaleResponse, *models.ApiError)
	// DELETE /admin/sales/:id
	DeleteSale(ctx context.Context, id string) *models.ApiError
}

type saleService struct {
	repo  repositories.SaleRepository
	plans repositories.PlanRepository
	now   func() time.Time
}

func NewSaleService(repo repositories.SaleRepository, plans repositories.PlanRepository) SaleService {
	return &saleService{repo: repo, plans: plans, now: time.Now}
}

func (s *saleService) ListSales(ctx context.Context) (*api.SaleListResponse, *models.ApiError) {
	sales, err := s.repo.List(ctx)
	if err != nil {
		return nil, repoError(err, "sale not found")
	}
	day := today(s.now())
	items := make([]api.SaleResponse, 0, len(sales))
	for i := range sales {
		items = append(items, mapping.SaleToResponse(&sales[i], day))
	}
	// What is running or still to come first (the soonest first), then what is over (the latest first).
	rank := map[string]int{"running": 0, "upcoming": 1, "over": 2}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if rank[a.State] != rank[b.State] {
			return rank[a.State] < rank[b.State]
		}
		if a.State == "over" {
			return a.EndsOn > b.EndsOn
		}
		return a.StartsOn < b.StartsOn
	})
	return &api.SaleListResponse{Items: items}, nil
}

// read checks a request and turns it into a sale. A sale that ended before today cannot be written: it would
// change nothing, and is most likely a mistake in the year.
func (s *saleService) read(ctx context.Context, req api.SaleRequest) (*db.Sale, *models.ApiError) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, badRequest("name is required")
	}
	if req.Percent < 1 || req.Percent > 90 {
		return nil, badRequest("percent must be between 1 and 90")
	}
	from, err := time.Parse(time.DateOnly, req.StartsOn)
	if err != nil {
		return nil, badRequest("starts_on must be a day (YYYY-MM-DD)")
	}
	to, err := time.Parse(time.DateOnly, req.EndsOn)
	if err != nil {
		return nil, badRequest("ends_on must be a day (YYYY-MM-DD)")
	}
	if to.Before(from) {
		return nil, badRequest("ends_on must not be before starts_on")
	}
	if to.Before(today(s.now())) {
		return nil, badRequest("ends_on is already past")
	}
	ids := make([]string, 0, len(req.PlanIDs))
	seen := map[string]bool{}
	for _, id := range req.PlanIDs {
		id = strings.ToLower(id)
		if !isUUID(id) {
			return nil, badRequest("invalid plan id")
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		p, err := s.plans.GetByID(ctx, id)
		if err != nil {
			return nil, repoError(err, "plan not found")
		}
		if p.Price <= 0 {
			return nil, badRequest("the free plan cannot be in a sale")
		}
		ids = append(ids, id)
	}
	return &db.Sale{Name: name, Percent: req.Percent, StartsOn: from, EndsOn: to, PlanIDs: ids}, nil
}

func (s *saleService) CreateSale(ctx context.Context, req api.SaleRequest) (*api.SaleResponse, *models.ApiError) {
	sale, apiErr := s.read(ctx, req)
	if apiErr != nil {
		return nil, apiErr
	}
	created, err := s.repo.Create(ctx, sale)
	if err != nil {
		return nil, repoError(err, "sale not found")
	}
	out := mapping.SaleToResponse(created, today(s.now()))
	return &out, nil
}

func (s *saleService) UpdateSale(ctx context.Context, id string, req api.SaleRequest) (*api.SaleResponse, *models.ApiError) {
	if !isUUID(id) {
		return nil, badRequest("invalid sale id")
	}
	sale, apiErr := s.read(ctx, req)
	if apiErr != nil {
		return nil, apiErr
	}
	updated, err := s.repo.Update(ctx, id, sale)
	if err != nil {
		return nil, repoError(err, "sale not found")
	}
	out := mapping.SaleToResponse(updated, today(s.now()))
	return &out, nil
}

func (s *saleService) DeleteSale(ctx context.Context, id string) *models.ApiError {
	if !isUUID(id) {
		return badRequest("invalid sale id")
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return repoError(err, "sale not found")
	}
	return nil
}
