package mapping

import (
	"time"

	"9itchi-payments-api/models/api"
	"9itchi-payments-api/models/db"
)

func PlanToResponse(p *db.Plan) api.PlanResponse {
	quotas := p.Quotas
	if quotas == nil {
		quotas = map[string]int{}
	}
	details := p.Details
	if details == nil {
		details = db.PlanDetails{}
	}
	ends := ""
	if p.LaunchEndsOn != nil {
		ends = p.LaunchEndsOn.Format(time.DateOnly)
	}
	var sale *api.PlanSale
	if p.Sale != nil {
		sale = &api.PlanSale{Name: p.Sale.Name, Percent: p.Sale.Percent, EndsOn: p.Sale.EndsOn.Format(time.DateOnly)}
	}
	return api.PlanResponse{
		ID: p.ID, Name: p.Name, Price: p.Price, RegularPrice: p.RegularPrice, LaunchEndsOn: ends, Sale: sale, DurationDays: p.DurationDays,
		Quotas: quotas, Details: details, Active: p.Active, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

// SaleToResponse writes a sale for the admin, with where it stands on the given day (a date at midnight UTC).
func SaleToResponse(s *db.Sale, day time.Time) api.SaleResponse {
	ids := s.PlanIDs
	if ids == nil {
		ids = []string{}
	}
	state := "running"
	switch {
	case day.Before(s.StartsOn):
		state = "upcoming"
	case day.After(s.EndsOn):
		state = "over"
	}
	return api.SaleResponse{
		ID: s.ID, Name: s.Name, Percent: s.Percent, StartsOn: s.StartsOn.Format(time.DateOnly), EndsOn: s.EndsOn.Format(time.DateOnly),
		PlanIDs: ids, State: state, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

func PromoToResponse(p *db.PromoCode) api.PromoResponse {
	ids := p.PlanIDs
	if ids == nil {
		ids = []string{}
	}
	return api.PromoResponse{
		ID: p.ID, Code: p.Code, Type: p.Type, Value: p.Value,
		ValidFrom: p.ValidFrom, ValidUntil: p.ValidUntil, UsageLimit: p.UsageLimit,
		UsedCount: p.UsedCount, PlanIDs: ids, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func SubscriptionToResponse(s *db.Subscription) *api.SubscriptionResponse {
	return &api.SubscriptionResponse{
		ID: s.ID, PlanID: s.PlanID, PromoCodeID: s.PromoCodeID, Status: s.Status,
		Amount: s.Amount, StartsAt: s.StartsAt, EndsAt: s.EndsAt, Renews: s.StoppedAt == nil, StoppedAt: s.StoppedAt,
	}
}
