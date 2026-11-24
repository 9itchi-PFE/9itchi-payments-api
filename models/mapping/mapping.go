package mapping

import (
	"9itchi-payments-api/models/api"
	"9itchi-payments-api/models/db"
)

func PlanToResponse(p *db.Plan) api.PlanResponse {
	quotas := p.Quotas
	if quotas == nil {
		quotas = map[string]int{}
	}
	return api.PlanResponse{
		ID: p.ID, Name: p.Name, Price: p.Price, RegularPrice: p.RegularPrice, DurationDays: p.DurationDays,
		Quotas: quotas, Active: p.Active, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
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
