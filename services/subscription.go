package services

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"

	"9itchi-payments-api/core"
	"9itchi-payments-api/models"
	"9itchi-payments-api/models/api"
	"9itchi-payments-api/models/db"
	"9itchi-payments-api/models/mapping"
	"9itchi-payments-api/repositories"
	"9itchi-payments-api/utils"
)

const (
	checkoutProvider = core.FlouciProviderName
	// freeProvider labels fully discounted payments (0 TND), which never reach a gateway.
	freeProvider     = "free"
	paymentCompleted = "completed"
	paymentPending   = "pending"
)

var errPromoExhausted = errors.New("promo code usage limit reached")

// SubscriptionService holds all business logic for subscription. Handlers stay thin.
type SubscriptionService interface {
	// POST /subscriptions/checkout: start payment for a plan (+ promo)
	Checkout(ctx context.Context, userID string, req api.CheckoutRequest) (*api.CheckoutResponse, *models.ApiError)
	// GET /subscriptions/me: active subscription with its plan, else the Free plan
	MySubscription(ctx context.Context, userID string) (*api.MySubscriptionResponse, *models.ApiError)
	// POST /subscriptions/cancel: stop the renewal of the active paid subscription; access lasts until it ends
	CancelSubscription(ctx context.Context, userID string) (*api.MySubscriptionResponse, *models.ApiError)
	// POST /subscriptions/resume: renew again a subscription that was stopped and has not ended yet
	ResumeSubscription(ctx context.Context, userID string) (*api.MySubscriptionResponse, *models.ApiError)
	// GET /payments: the user's completed payments (receipts), newest first
	ListPayments(ctx context.Context, userID string) (*api.PaymentListResponse, *models.ApiError)
	// POST /payments/:id/verify: where a payment stands; a pending one is re-checked with the gateway and
	// completed when it is paid, so the return page does not depend on the webhook
	VerifyPayment(ctx context.Context, userID, id string) (*api.PaymentResponse, *models.ApiError)
	// POST /webhooks/:provider: payment provider callback (signature verified, payment re-checked with the provider)
	Webhook(ctx context.Context, provider string, payload []byte, signature string) (*api.MessageResponse, *models.ApiError)
	// POST /internal/subscriptions/renewal-reminders: raise subscription.renewal_due for the renewing subscriptions
	// that end soon or just lapsed, once per period (called by schedule-api)
	SendRenewalReminders(ctx context.Context) (*api.RenewalRemindersResponse, *models.ApiError)
}

type subscriptionService struct {
	repo      repositories.SubscriptionRepository
	plans     repositories.PlanRepository
	promos    repositories.PromoRepository
	payments  repositories.PaymentRepository
	tx        repositories.Transactor
	providers core.ProviderFactory
	events    core.Publisher
	now       func() time.Time
}

func NewSubscriptionService(repo repositories.SubscriptionRepository, plans repositories.PlanRepository,
	promos repositories.PromoRepository, payments repositories.PaymentRepository, tx repositories.Transactor,
	providers core.ProviderFactory, events core.Publisher) SubscriptionService {
	return &subscriptionService{repo: repo, plans: plans, promos: promos, payments: payments, tx: tx,
		providers: providers, events: events, now: time.Now}
}

func millimes(price float64) int64 { return int64(math.Round(price * 1000)) }

func providerError(err error) *models.ApiError {
	if errors.Is(err, core.ErrProviderUnavailable) {
		return &models.ApiError{Code: http.StatusServiceUnavailable, Error: errors.New("payment provider unavailable")}
	}
	utils.Logger.Logger.Errorw("payment provider error", "error", err)
	return &models.ApiError{Code: http.StatusBadGateway, Error: errors.New("payment provider error")}
}

// price returns the amount due and the discount in millimes, plus the promo id when a code applies.
func (s *subscriptionService) price(ctx context.Context, plan *db.Plan, code string) (amount, discount int64, promoID *string, apiErr *models.ApiError) {
	amount = millimes(plan.Price)
	code = normalizeCode(code)
	if code == "" {
		return amount, 0, nil, nil
	}
	promo, err := s.promos.GetByCode(ctx, code)
	if errors.Is(err, repositories.ErrNotFound) {
		return 0, 0, nil, badRequest("invalid promo code")
	}
	if err != nil {
		return 0, 0, nil, repoError(err, "promo code not found")
	}
	res := discountFor(promo, plan, s.now())
	if !res.Valid {
		return 0, 0, nil, badRequest(res.Reason)
	}
	final := millimes(res.FinalPrice)
	return final, amount - final, &promo.ID, nil
}

func (s *subscriptionService) Checkout(ctx context.Context, userID string, req api.CheckoutRequest) (*api.CheckoutResponse, *models.ApiError) {
	if !isUUID(userID) {
		return nil, badRequest("invalid user id")
	}
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
	if plan.Price <= 0 {
		return nil, badRequest("the free plan needs no checkout")
	}
	plan, launch, apiErr := s.withKeptLaunch(ctx, userID, plan)
	if apiErr != nil {
		return nil, apiErr
	}
	amount, discount, promoID, apiErr := s.price(ctx, plan, req.PromoCode)
	if apiErr != nil {
		return nil, apiErr
	}
	pay := &db.Payment{UserID: userID, PlanID: plan.ID, PromoCodeID: promoID, Provider: checkoutProvider,
		AmountMillimes: amount, DiscountMillimes: discount, LaunchMillimes: launch}
	if plan.RegularPrice != nil {
		regular := millimes(*plan.RegularPrice)
		pay.RegularMillimes = &regular
	}
	if plan.Sale != nil {
		pay.SaleName = plan.Sale.Name
	}
	if amount == 0 {
		return s.activateFree(ctx, pay)
	}
	return s.startPayment(ctx, pay)
}

// activateFree creates the payment and activates the subscription without calling a gateway.
func (s *subscriptionService) activateFree(ctx context.Context, pay *db.Payment) (*api.CheckoutResponse, *models.ApiError) {
	pay.Provider = freeProvider
	var created *db.Payment
	var subCreated bool
	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		var err error
		if created, err = s.payments.Create(ctx, pay); err != nil {
			return err
		}
		_, subCreated, err = s.complete(ctx, created)
		return err
	})
	if errors.Is(err, errPromoExhausted) {
		return nil, badRequest("promo code usage limit reached")
	}
	if err != nil {
		return nil, repoError(err, "plan not found")
	}
	s.publish(ctx, created, subCreated)
	return &api.CheckoutResponse{PaymentID: created.ID}, nil
}

func (s *subscriptionService) startPayment(ctx context.Context, pay *db.Payment) (*api.CheckoutResponse, *models.ApiError) {
	newProvider, ok := s.providers[checkoutProvider]
	if !ok {
		return nil, providerError(core.ErrProviderUnavailable)
	}
	p := newProvider()
	if !p.Available() {
		return nil, providerError(core.ErrProviderUnavailable)
	}
	created, err := s.payments.Create(ctx, pay)
	if err != nil {
		return nil, repoError(err, "plan not found")
	}
	res, err := p.CreateCheckout(ctx, core.CheckoutRequest{PaymentID: created.ID, AmountMillimes: created.AmountMillimes})
	if err == nil && (res.Ref == "" || res.URL == "") {
		err = errors.New("provider returned no payment reference")
	}
	if err != nil {
		if ferr := s.payments.MarkFailed(ctx, created.ID); ferr != nil {
			utils.Logger.Logger.Errorw("mark payment failed", "error", ferr)
		}
		return nil, providerError(err)
	}
	if err := s.payments.SetProviderRef(ctx, created.ID, res.Ref); err != nil {
		return nil, repoError(err, "payment not found")
	}
	return &api.CheckoutResponse{PaymentURL: res.URL, PaymentID: created.ID}, nil
}

// complete marks the payment completed, creates or extends the subscription and records promo usage in one
// transaction. applied=false means the payment was already completed (replay): nothing changes.
func (s *subscriptionService) complete(ctx context.Context, pay *db.Payment) (applied, subCreated bool, err error) {
	err = s.tx.WithTx(ctx, func(ctx context.Context) error {
		ok, err := s.payments.MarkCompleted(ctx, pay.ID)
		if err != nil || !ok {
			return err
		}
		applied = true
		if pay.PromoCodeID != nil {
			used, err := s.promos.IncrementUsage(ctx, *pay.PromoCodeID)
			if err != nil {
				return err
			}
			// A paid order keeps its discount (the money is taken); a free one must not exceed the limit.
			if !used && pay.AmountMillimes == 0 {
				return errPromoExhausted
			}
		}
		plan, err := s.plans.GetByID(ctx, pay.PlanID)
		if err != nil {
			return err
		}
		cur, err := s.repo.GetActiveByUser(ctx, pay.UserID)
		if err != nil && !errors.Is(err, repositories.ErrNotFound) {
			return err
		}
		if cur != nil && cur.PlanID == pay.PlanID {
			// More days of the same plan: the paid period starts where the current one ends.
			end := cur.EndsAt.AddDate(0, 0, plan.DurationDays)
			if err := s.repo.Extend(ctx, cur.ID, end); err != nil {
				return err
			}
			return s.payments.SetPeriod(ctx, pay.ID, cur.EndsAt, end)
		}
		if err := s.repo.ExpireActive(ctx, pay.UserID); err != nil {
			return err
		}
		now := s.now()
		end := now.AddDate(0, 0, plan.DurationDays)
		// Another plan (a monthly subscriber taking the yearly one): the new plan starts now, and the days the
		// student already paid and has not used are added at its end, so that changing plan never loses any.
		if cur != nil && cur.EndsAt.After(now) {
			end = end.Add(cur.EndsAt.Sub(now))
		}
		if _, err := s.repo.Create(ctx, &db.Subscription{UserID: pay.UserID, PlanID: pay.PlanID, PromoCodeID: pay.PromoCodeID,
			Amount: float64(pay.AmountMillimes) / 1000, StartsAt: now, EndsAt: end, LaunchMillimes: pay.LaunchMillimes}); err != nil {
			return err
		}
		subCreated = true
		return s.payments.SetPeriod(ctx, pay.ID, now, end)
	})
	return applied, subCreated, err
}

// publish sends the domain events after commit; a failure is logged and never undoes the payment.
func (s *subscriptionService) publish(ctx context.Context, pay *db.Payment, subCreated bool) {
	payload := map[string]any{"user_id": pay.UserID, "plan_id": pay.PlanID, "payment_id": pay.ID, "amount_millimes": pay.AmountMillimes}
	// What the receipt says, so that the email about it needs no second lookup. The payment is read again because
	// completing it gave it its number and its period. Without it the event still goes, with the amounts only.
	if done, err := s.payments.Get(ctx, pay.UserID, pay.ID); err == nil {
		r := s.paymentResponse(ctx, done, map[string]*db.Plan{})
		payload["receipt_no"], payload["plan_name"], payload["duration_days"] = r.ReceiptNo, r.PlanName, r.DurationDays
		if r.PeriodStart != nil && r.PeriodEnd != nil {
			payload["period_start"], payload["period_end"] = r.PeriodStart.UTC().Format(time.RFC3339), r.PeriodEnd.UTC().Format(time.RFC3339)
		}
	}
	names := []string{core.EventPaymentCompleted}
	if subCreated {
		names = append(names, core.EventSubscriptionCreated)
	}
	for _, name := range names {
		if err := s.events.Publish(ctx, name, payload); err != nil {
			utils.Logger.Logger.Errorw("publish event", "event", name, "error", err)
		}
	}
}

func (s *subscriptionService) MySubscription(ctx context.Context, userID string) (*api.MySubscriptionResponse, *models.ApiError) {
	if !isUUID(userID) {
		return nil, badRequest("invalid user id")
	}
	sub, plan, err := userPlan(ctx, s.repo, s.plans, userID)
	if err != nil {
		return nil, repoError(err, "plan not found")
	}
	resp := &api.MySubscriptionResponse{Plan: mapping.PlanToResponse(plan), IsFree: sub == nil}
	if sub != nil {
		resp.Subscription = mapping.SubscriptionToResponse(sub)
		// The renewal costs the launch price the student keeps: the plan is shown at that price.
		if kept := keptPrice(sub, plan, s.now()); kept > 0 {
			resp.Plan = mapping.PlanToResponse(atKeptPrice(plan, kept))
			resp.KeptPrice = ptrFloat(float64(kept) / 1000)
		} else if kept := keeps(sub, plan, s.now()); kept > 0 {
			resp.KeptPrice = ptrFloat(float64(kept) / 1000)
		}
		return resp, nil
	}
	// Nothing active: say whether a renewal is still waiting to be paid.
	last, err := s.repo.LastEnded(ctx, userID)
	if err != nil && !errors.Is(err, repositories.ErrNotFound) {
		return nil, repoError(err, "plan not found")
	}
	if last != nil && last.StoppedAt == nil {
		resp.Lapsed = mapping.SubscriptionToResponse(last)
		if lastPlan, err := s.plans.GetByID(ctx, last.PlanID); err == nil {
			if kept := keeps(last, lastPlan, s.now()); kept > 0 {
				resp.KeptPrice = ptrFloat(float64(kept) / 1000)
			}
		}
	}
	return resp, nil
}

func (s *subscriptionService) CancelSubscription(ctx context.Context, userID string) (*api.MySubscriptionResponse, *models.ApiError) {
	return s.setRenewal(ctx, userID, false)
}

func (s *subscriptionService) ResumeSubscription(ctx context.Context, userID string) (*api.MySubscriptionResponse, *models.ApiError) {
	return s.setRenewal(ctx, userID, true)
}

// setRenewal stops or resumes the renewal of the active paid subscription. Access never changes here: what was
// paid for lasts until ends_at.
func (s *subscriptionService) setRenewal(ctx context.Context, userID string, renews bool) (*api.MySubscriptionResponse, *models.ApiError) {
	if !isUUID(userID) {
		return nil, badRequest("invalid user id")
	}
	const notFound = "no active subscription"
	sub, err := s.repo.GetActiveByUser(ctx, userID)
	if err != nil {
		return nil, repoError(err, notFound)
	}
	plan, err := s.plans.GetByID(ctx, sub.PlanID)
	if err != nil {
		return nil, repoError(err, notFound)
	}
	if plan.Price <= 0 {
		return nil, repoError(repositories.ErrNotFound, notFound)
	}
	if sub, err = s.repo.SetRenewal(ctx, userID, renews); err != nil {
		return nil, repoError(err, notFound)
	}
	return &api.MySubscriptionResponse{Subscription: mapping.SubscriptionToResponse(sub), Plan: mapping.PlanToResponse(plan)}, nil
}

// maxReceipts bounds GET /payments (one payment a month: years of history).
const maxReceipts = 200

// paymentResponse adds the plan and the promo code to a payment. A plan or code deleted since is left empty.
func (s *subscriptionService) paymentResponse(ctx context.Context, pay *db.Payment, plans map[string]*db.Plan) api.PaymentResponse {
	res := api.PaymentResponse{
		ID: pay.ID, Status: pay.Status, PlanID: pay.PlanID, AmountMillimes: pay.AmountMillimes, DiscountMillimes: pay.DiscountMillimes, RegularMillimes: pay.RegularMillimes, SaleName: pay.SaleName,
		Provider: pay.Provider, PeriodStart: pay.PeriodStart, PeriodEnd: pay.PeriodEnd, CompletedAt: pay.CompletedAt, CreatedAt: pay.CreatedAt,
	}
	if pay.ProviderRef != nil {
		res.ProviderRef = *pay.ProviderRef
	}
	if pay.ReceiptNo != nil && pay.CompletedAt != nil {
		res.ReceiptNo = fmt.Sprintf("%d-%06d", pay.CompletedAt.UTC().Year(), *pay.ReceiptNo)
	}
	plan, seen := plans[pay.PlanID]
	if !seen {
		if p, err := s.plans.GetByID(ctx, pay.PlanID); err == nil {
			plan = p
		}
		plans[pay.PlanID] = plan
	}
	if plan != nil {
		res.PlanName, res.DurationDays = plan.Name, plan.DurationDays
	}
	if pay.PromoCodeID != nil {
		if promo, err := s.promos.GetByID(ctx, *pay.PromoCodeID); err == nil && promo != nil {
			res.PromoCode = promo.Code
		}
	}
	return res
}

func (s *subscriptionService) ListPayments(ctx context.Context, userID string) (*api.PaymentListResponse, *models.ApiError) {
	if !isUUID(userID) {
		return nil, badRequest("invalid user id")
	}
	pays, err := s.payments.ListCompletedByUser(ctx, userID, maxReceipts)
	if err != nil {
		return nil, repoError(err, "payment not found")
	}
	out := &api.PaymentListResponse{Items: make([]api.PaymentResponse, 0, len(pays))}
	plans := map[string]*db.Plan{}
	for i := range pays {
		out.Items = append(out.Items, s.paymentResponse(ctx, &pays[i], plans))
	}
	return out, nil
}

func (s *subscriptionService) VerifyPayment(ctx context.Context, userID, id string) (*api.PaymentResponse, *models.ApiError) {
	if !isUUID(userID) {
		return nil, badRequest("invalid user id")
	}
	if !isUUID(id) {
		return nil, badRequest("invalid payment id")
	}
	pay, err := s.payments.Get(ctx, userID, id)
	if err != nil {
		return nil, repoError(err, "payment not found")
	}
	if pay.Status == paymentPending && pay.ProviderRef != nil {
		// The gateway is asked, never the browser: a pending payment only completes when it is really paid.
		// When the gateway cannot be reached the payment simply stays pending and the client asks again.
		if newProvider, ok := s.providers[pay.Provider]; ok {
			paid, verr := newProvider().VerifyPayment(ctx, *pay.ProviderRef)
			if verr != nil {
				utils.Logger.Logger.Warnw("verify payment", "payment_id", pay.ID, "error", verr)
			} else if paid {
				applied, subCreated, cerr := s.complete(ctx, pay)
				if cerr != nil {
					return nil, repoError(cerr, "payment not found")
				}
				if applied {
					s.publish(ctx, pay, subCreated)
				}
				if pay, err = s.payments.Get(ctx, userID, id); err != nil {
					return nil, repoError(err, "payment not found")
				}
			}
		}
	}
	res := s.paymentResponse(ctx, pay, map[string]*db.Plan{})
	return &res, nil
}

func (s *subscriptionService) Webhook(ctx context.Context, provider string, payload []byte, signature string) (*api.MessageResponse, *models.ApiError) {
	newProvider, ok := s.providers[provider]
	if !ok {
		return nil, repoError(repositories.ErrNotFound, "not found")
	}
	p := newProvider()
	ref, err := p.VerifyWebhook(payload, signature)
	if errors.Is(err, core.ErrInvalidSignature) {
		return nil, &models.ApiError{Code: http.StatusUnauthorized, Error: errors.New("invalid signature")}
	}
	if err != nil {
		return nil, badRequest("invalid webhook")
	}
	pay, err := s.payments.GetByProviderRef(ctx, provider, ref)
	if err != nil {
		return nil, repoError(err, "payment not found")
	}
	ack := &api.MessageResponse{Message: "ok"}
	if pay.Status == paymentCompleted {
		return ack, nil
	}
	paid, err := p.VerifyPayment(ctx, ref)
	if err != nil {
		return nil, providerError(err)
	}
	if !paid {
		return nil, badRequest("payment not verified")
	}
	applied, subCreated, err := s.complete(ctx, pay)
	if err != nil {
		return nil, repoError(err, "payment not found")
	}
	if applied {
		s.publish(ctx, pay, subCreated)
	}
	return ack, nil
}

const (
	// renewalReminderWindow is how long before the end of a period the student is reminded: the same 7 days as the
	// banner of the app. lapseReminderWindow is how long after a lapse the reminder is still worth sending.
	renewalReminderWindow = 7 * 24 * time.Hour
	lapseReminderWindow   = 7 * 24 * time.Hour
	// reminderBatch bounds one call; the rest is taken at the next one.
	reminderBatch = 200
)

func (s *subscriptionService) SendRenewalReminders(ctx context.Context) (*api.RenewalRemindersResponse, *models.ApiError) {
	res := &api.RenewalRemindersResponse{}
	for _, stage := range []struct {
		name   string
		window time.Duration
		count  *int
	}{{db.ReminderDue, renewalReminderWindow, &res.Due}, {db.ReminderLapsed, lapseReminderWindow, &res.Lapsed}} {
		claimed, err := s.repo.ClaimReminders(ctx, stage.name, stage.window, reminderBatch)
		if err != nil {
			return nil, repoError(err, "subscription not found")
		}
		for _, r := range claimed {
			payload := map[string]any{
				"user_id": r.UserID, "subscription_id": r.SubscriptionID, "plan_id": r.PlanID, "plan_name": r.PlanName,
				"price_millimes": millimes(r.PlanPrice), "ends_at": r.EndsAt.UTC().Format(time.RFC3339), "stage": stage.name,
			}
			if err := s.events.Publish(ctx, core.EventSubscriptionRenewalDue, payload); err != nil {
				// Not raised: the claim is given back so that the next call tries again.
				utils.Logger.Logger.Warnw("publish event", "event", core.EventSubscriptionRenewalDue, "subscription_id", r.SubscriptionID, "error", err)
				if rerr := s.repo.ReleaseReminder(ctx, r.SubscriptionID, stage.name); rerr != nil {
					utils.Logger.Logger.Errorw("release renewal reminder", "subscription_id", r.SubscriptionID, "error", rerr)
				}
				res.Failed++
				continue
			}
			*stage.count++
		}
	}
	return res, nil
}

// keeps is the launch price, in millimes, the holder of sub keeps on plan: the launch price they subscribed at,
// when the renewal comes at most KeepGap after the end of sub. 0 when nothing is kept.
func keeps(sub *db.Subscription, plan *db.Plan, now time.Time) int64 {
	if sub == nil || plan == nil || sub.LaunchMillimes == nil || sub.PlanID != plan.ID || now.Sub(sub.EndsAt) > db.KeepGap {
		return 0
	}
	return *sub.LaunchMillimes
}

// keptPrice is what keeps gives when it is lower than what plan costs today (a sale or a new launch price can be
// lower): the price a renewal is charged. 0 when today's price applies.
func keptPrice(sub *db.Subscription, plan *db.Plan, now time.Time) int64 {
	if kept := keeps(sub, plan, now); kept > 0 && kept < millimes(plan.Price) {
		return kept
	}
	return 0
}

// atKeptPrice is plan at the launch price kept by a subscriber: the price of today is struck through.
func atKeptPrice(plan *db.Plan, kept int64) *db.Plan {
	out := *plan
	usual := plan.Price
	if plan.RegularPrice != nil && *plan.RegularPrice > usual {
		usual = *plan.RegularPrice
	}
	out.Price, out.RegularPrice, out.LaunchEndsOn, out.Sale = float64(kept)/1000, &usual, nil, nil
	launch := out.Price
	out.LaunchPrice = &launch
	return &out
}

// withKeptLaunch prices plan for a checkout by userID: at the launch price they keep when they renew in time.
// It also says which launch price the payment carries on to the subscription (the lowest of the one kept and
// the one running today), nil when none: a renewal paid during a cheaper sale still keeps the launch price.
func (s *subscriptionService) withKeptLaunch(ctx context.Context, userID string, plan *db.Plan) (*db.Plan, *int64, *models.ApiError) {
	cur, err := s.repo.GetActiveByUser(ctx, userID)
	if err != nil && !errors.Is(err, repositories.ErrNotFound) {
		return nil, nil, repoError(err, "plan not found")
	}
	if cur == nil {
		if cur, err = s.repo.LastEnded(ctx, userID); err != nil && !errors.Is(err, repositories.ErrNotFound) {
			return nil, nil, repoError(err, "plan not found")
		}
	}
	var carry *int64
	if kept := keeps(cur, plan, s.now()); kept > 0 {
		carry = &kept
	}
	if plan.LaunchPrice != nil {
		if launch := millimes(*plan.LaunchPrice); carry == nil || launch < *carry {
			carry = &launch
		}
	}
	if kept := keptPrice(cur, plan, s.now()); kept > 0 {
		return atKeptPrice(plan, kept), carry, nil
	}
	return plan, carry, nil
}

func ptrFloat(f float64) *float64 { return &f }
