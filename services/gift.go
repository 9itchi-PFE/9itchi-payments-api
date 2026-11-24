package services

import (
	"context"
	"errors"
	"net/http"
	"time"

	"9itchi-payments-api/core"
	"9itchi-payments-api/models"
	"9itchi-payments-api/models/api"
	"9itchi-payments-api/models/db"
	"9itchi-payments-api/repositories"
	"9itchi-payments-api/utils"
)

const (
	// giftDays is the month the team gives, whatever the period of the plan.
	giftDays = 30
	// giftPause is how long a student who was just given a month cannot be given another: a double click, or two
	// admins answering the same request, give one month and not two.
	giftPause = time.Minute
	// maxGifts bounds GET /admin/users/:id/gifts.
	maxGifts = 100
)

var (
	errGiftTooSoon = errors.New("a month was just given to this student")
	errNoPaidPlan  = errors.New("no paid plan to give")
)

// GiftService holds the business logic of the months the team gives. Handlers stay thin.
type GiftService interface {
	// POST /admin/users/:id/gifts: give the student 30 days. A running paid subscription ends 30 days later and
	// renews as the student left it. Without one, 30 days of a paid plan start now (the plan the student had last,
	// else the cheapest one) and end without a renewal being asked. Nothing is paid and no receipt is made.
	GiveMonth(ctx context.Context, adminID, userID string) (*api.GiftResponse, *models.ApiError)
	// GET /admin/users/:id/gifts: what the team gave this student, newest first
	ListGifts(ctx context.Context, userID string) (*api.GiftListResponse, *models.ApiError)
}

type giftService struct {
	gifts  repositories.GiftRepository
	subs   repositories.SubscriptionRepository
	plans  repositories.PlanRepository
	tx     repositories.Transactor
	events core.Publisher
	now    func() time.Time
}

func NewGiftService(gifts repositories.GiftRepository, subs repositories.SubscriptionRepository, plans repositories.PlanRepository,
	tx repositories.Transactor, events core.Publisher) GiftService {
	return &giftService{gifts: gifts, subs: subs, plans: plans, tx: tx, events: events, now: time.Now}
}

func giftResponse(g *db.Gift, plan *db.Plan) api.GiftResponse {
	res := api.GiftResponse{ID: g.ID, PlanID: g.PlanID, Days: g.Days, GivenBy: g.GivenBy,
		PeriodStart: g.PeriodStart, PeriodEnd: g.PeriodEnd, CreatedAt: g.CreatedAt}
	if plan != nil {
		res.PlanName = plan.Name
	}
	return res
}

func (s *giftService) GiveMonth(ctx context.Context, adminID, userID string) (*api.GiftResponse, *models.ApiError) {
	if !isUUID(adminID) {
		return nil, badRequest("invalid admin id")
	}
	if !isUUID(userID) {
		return nil, badRequest("invalid user id")
	}
	var gift *db.Gift
	var plan *db.Plan
	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		if err := s.gifts.LockUser(ctx, userID); err != nil {
			return err
		}
		now := s.now()
		last, err := s.gifts.ListByUser(ctx, userID, 1)
		if err != nil {
			return err
		}
		if len(last) > 0 && now.Sub(last[0].CreatedAt) < giftPause {
			return errGiftTooSoon
		}
		row := &db.Gift{UserID: userID, Days: giftDays, GivenBy: adminID}
		cur, err := s.subs.GetActiveByUser(ctx, userID)
		if err != nil && !errors.Is(err, repositories.ErrNotFound) {
			return err
		}
		if cur != nil {
			if plan, err = s.plans.GetByID(ctx, cur.PlanID); err != nil {
				return err
			}
		}
		if cur != nil && plan.Price > 0 {
			moved, err := s.subs.AddDays(ctx, cur.ID, giftDays)
			if err != nil {
				return err
			}
			row.SubscriptionID, row.PlanID, row.PeriodStart, row.PeriodEnd = cur.ID, plan.ID, cur.EndsAt, moved.EndsAt
		} else {
			if plan, err = s.planToGive(ctx, userID); err != nil {
				return err
			}
			if err := s.subs.ExpireActive(ctx, userID); err != nil {
				return err
			}
			// Stopped from the start: the student did not buy it, so no renewal is asked when it ends.
			created, err := s.subs.Create(ctx, &db.Subscription{UserID: userID, PlanID: plan.ID, StartsAt: now,
				EndsAt: now.AddDate(0, 0, giftDays), StoppedAt: &now})
			if err != nil {
				return err
			}
			row.SubscriptionID, row.PlanID, row.PeriodStart, row.PeriodEnd = created.ID, plan.ID, created.StartsAt, created.EndsAt
		}
		gift, err = s.gifts.Create(ctx, row)
		return err
	})
	switch {
	case errors.Is(err, errGiftTooSoon), errors.Is(err, errNoPaidPlan):
		return nil, &models.ApiError{Code: http.StatusConflict, Error: err}
	case err != nil:
		return nil, repoError(err, "plan not found")
	}
	// After the commit: a broker that is down is logged and never takes the month back.
	payload := map[string]any{"user_id": userID, "gift_id": gift.ID, "plan_id": plan.ID, "plan_name": plan.Name,
		"days": gift.Days, "ends_at": gift.PeriodEnd.UTC().Format(time.RFC3339)}
	if err := s.events.Publish(ctx, core.EventSubscriptionGifted, payload); err != nil {
		utils.Logger.Logger.Errorw("publish event", "event", core.EventSubscriptionGifted, "error", err)
	}
	res := giftResponse(gift, plan)
	return &res, nil
}

// planToGive picks the paid plan of a month given to a student with no paid subscription running: the plan they
// had last when it is still sold, else the cheapest paid plan on sale.
func (s *giftService) planToGive(ctx context.Context, userID string) (*db.Plan, error) {
	last, err := s.subs.LastEnded(ctx, userID)
	if err != nil && !errors.Is(err, repositories.ErrNotFound) {
		return nil, err
	}
	if last != nil {
		p, err := s.plans.GetByID(ctx, last.PlanID)
		if err != nil && !errors.Is(err, repositories.ErrNotFound) {
			return nil, err
		}
		if p != nil && p.Active && p.Price > 0 {
			return p, nil
		}
	}
	all, err := s.plans.List(ctx, true)
	if err != nil {
		return nil, err
	}
	var best *db.Plan
	for i := range all {
		if all[i].Price > 0 && (best == nil || all[i].Price < best.Price) {
			best = &all[i]
		}
	}
	if best == nil {
		return nil, errNoPaidPlan
	}
	return best, nil
}

func (s *giftService) ListGifts(ctx context.Context, userID string) (*api.GiftListResponse, *models.ApiError) {
	if !isUUID(userID) {
		return nil, badRequest("invalid user id")
	}
	gifts, err := s.gifts.ListByUser(ctx, userID, maxGifts)
	if err != nil {
		return nil, repoError(err, "gift not found")
	}
	out := &api.GiftListResponse{Items: make([]api.GiftResponse, 0, len(gifts))}
	plans := map[string]*db.Plan{}
	for i := range gifts {
		plan, seen := plans[gifts[i].PlanID]
		if !seen {
			// A plan that cannot be read leaves its name empty: the gift is still listed.
			if p, err := s.plans.GetByID(ctx, gifts[i].PlanID); err == nil {
				plan = p
			}
			plans[gifts[i].PlanID] = plan
		}
		out.Items = append(out.Items, giftResponse(&gifts[i], plan))
	}
	return out, nil
}
